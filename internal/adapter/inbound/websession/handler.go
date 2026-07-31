// Package websession exposes cookie-authenticated browser endpoints.
package websession

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
)

const (
	accessCookieName  = "nh_access"
	refreshCookieName = "nh_refresh"
	csrfCookieName    = "nh_csrf"
	maxRequestBytes   = 8 << 10
)

type principalContextKey struct{}

// Config contains browser adapter settings.
type Config struct {
	WebOrigin string
}

// PrincipalAuthenticator resolves a cookie access token to a canonical account.
type PrincipalAuthenticator interface {
	AuthenticateAccessToken(ctx context.Context, token string) (domain.RequestPrincipal, error)
}

// SessionService issues and revokes browser sessions.
type SessionService interface {
	IssueSession(ctx context.Context, accountID uuid.UUID, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error)
	RefreshSession(ctx context.Context, refreshToken string, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error)
	Logout(ctx context.Context, refreshToken string) error
}

// PasswordService verifies an email/password pair linked to an account.
type PasswordService interface {
	AuthenticateEmailPassword(ctx context.Context, email, password string) (domain.IdentityResolution, error)
}

// AccountService returns the safe profile for the authenticated account.
type AccountService interface {
	Profile(ctx context.Context, accountID uuid.UUID) (accountservice.AccountProfile, error)
}

// Deps are services shared with other account adapters.
type Deps struct {
	Authenticator PrincipalAuthenticator
	Sessions      SessionService
	Passwords     PasswordService
	Account       AccountService
	Conversations domain.ConversationRepository
}

// Handler serves the browser-only /web/v1 endpoints.
type Handler struct {
	cfg  Config
	deps Deps
}

// NewHandler builds a browser session handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	return &Handler{cfg: cfg, deps: deps}
}

// Routes returns the versioned browser API router.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /web/v1/auth/password/login", h.passwordLogin)
	mux.HandleFunc("POST /web/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /web/v1/auth/logout", h.logout)
	mux.HandleFunc("GET /web/v1/me", h.requirePrincipal(h.me))
	mux.HandleFunc("GET /web/v1/conversations", h.requirePrincipal(h.listConversations))
	mux.HandleFunc("POST /web/v1/conversations", h.requireUnsafePrincipal(h.createConversation))
	return mux
}

// PrincipalFromContext returns the valid principal established by this adapter.
func PrincipalFromContext(ctx context.Context) (domain.RequestPrincipal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(domain.RequestPrincipal)
	return principal, ok && principal.Validate() == nil
}

func (h *Handler) passwordLogin(w http.ResponseWriter, r *http.Request) {
	if !h.requireOrigin(w, r) {
		return
	}
	if h.deps.Passwords == nil || h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
		return
	}
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceInfo string `json:"device_info"`
	}
	if !decodeJSON(w, r, &req) || strings.TrimSpace(req.Email) == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid password login request")
		return
	}
	resolution, err := h.deps.Passwords.AuthenticateEmailPassword(r.Context(), req.Email, req.Password)
	if err != nil || resolution.AccountID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	tokens, err := h.deps.Sessions.IssueSession(r.Context(), resolution.AccountID, sessionMetadata(r, req.DeviceInfo))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
		return
	}
	if err := h.setSessionCookies(w, tokens); err != nil {
		writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, safeSessionResponse{Session: tokens.Session})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	if !h.requireUnsafeRequest(w, r) {
		return
	}
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
		return
	}
	refresh, err := r.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(refresh.Value) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokens, err := h.deps.Sessions.RefreshSession(r.Context(), refresh.Value, sessionMetadata(r, ""))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.setSessionCookies(w, tokens); err != nil {
		writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
		return
	}
	writeJSON(w, http.StatusOK, safeSessionResponse{Session: tokens.Session})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.requireUnsafeRequest(w, r) {
		return
	}
	var logoutErr error
	if h.deps.Sessions != nil {
		if refresh, err := r.Cookie(refreshCookieName); err == nil && strings.TrimSpace(refresh.Value) != "" {
			logoutErr = h.deps.Sessions.Logout(r.Context(), refresh.Value)
		}
	}
	h.expireSessionCookies(w)
	if logoutErr != nil {
		writeError(w, http.StatusServiceUnavailable, "logout failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) requirePrincipal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.deps.Authenticator == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
			return
		}
		access, err := r.Cookie(accessCookieName)
		if err != nil || strings.TrimSpace(access.Value) == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		principal, err := h.deps.Authenticator.AuthenticateAccessToken(r.Context(), access.Value)
		if err != nil || principal.Validate() != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
	}
}

func (h *Handler) requireUnsafePrincipal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireUnsafeRequest(w, r) {
			return
		}
		h.requirePrincipal(next)(w, r)
	}
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Account == nil {
		writeError(w, http.StatusServiceUnavailable, "account unavailable")
		return
	}
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	profile, err := h.deps.Account.Profile(r.Context(), principal.AccountID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "account unavailable")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limitValues, limitProvided := r.URL.Query()["limit"]
	limit, err := conversationLimit(limitValues, limitProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation limit")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	conversations, err := h.deps.Conversations.ListByAccountSource(r.Context(), principal.AccountID, domain.ConversationSourceWeb, limit, 0)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	items := make([]safeConversation, 0, len(conversations))
	for _, conversation := range conversations {
		if conversation == nil {
			writeError(w, http.StatusInternalServerError, "conversations unavailable")
			return
		}
		items = append(items, newSafeConversation(*conversation))
	}
	writeJSON(w, http.StatusOK, safeConversationList{Items: items})
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	externalThreadID := r.Header.Get("X-Idempotency-Key")
	if _, err := uuid.Parse(externalThreadID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid idempotency key")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	conversation := &domain.Conversation{
		AccountID:        principal.AccountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: externalThreadID,
		Status:           domain.ConversationActive,
		Title:            "",
	}
	if err := h.deps.Conversations.CreateConversation(r.Context(), conversation); err == nil {
		writeJSON(w, http.StatusCreated, newSafeConversation(*conversation))
		return
	} else if !errors.Is(err, domain.ErrConflict) {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	existing, err := h.deps.Conversations.GetActiveByReference(r.Context(), domain.ConversationRef{
		AccountID:        principal.AccountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: externalThreadID,
	})
	if err != nil || existing == nil || existing.AccountID != principal.AccountID || existing.Source != domain.ConversationSourceWeb || existing.ExternalThreadID != externalThreadID || existing.Status != domain.ConversationActive {
		writeError(w, http.StatusInternalServerError, "conversations unavailable")
		return
	}
	writeJSON(w, http.StatusOK, newSafeConversation(*existing))
}

func conversationLimit(values []string, provided bool) (int, error) {
	const (
		defaultConversationLimit = 20
		maxConversationLimit     = 50
	)
	if !provided {
		return defaultConversationLimit, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, errors.New("invalid conversation limit")
	}
	limit, err := strconv.Atoi(values[0])
	if err != nil || limit < 1 || limit > maxConversationLimit {
		return 0, errors.New("invalid conversation limit")
	}
	return limit, nil
}

func (h *Handler) requireUnsafeRequest(w http.ResponseWriter, r *http.Request) bool {
	if !h.requireOrigin(w, r) {
		return false
	}
	csrf, err := r.Cookie(csrfCookieName)
	if err != nil || strings.TrimSpace(csrf.Value) == "" || !constantTimeEqual(csrf.Value, r.Header.Get("X-CSRF-Token")) {
		writeError(w, http.StatusForbidden, "csrf validation failed")
		return false
	}
	return true
}

func (h *Handler) requireOrigin(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(h.cfg.WebOrigin) == "" || r.Header.Get("Origin") != h.cfg.WebOrigin {
		writeError(w, http.StatusForbidden, "origin validation failed")
		return false
	}
	return true
}

func (h *Handler) setSessionCookies(w http.ResponseWriter, tokens accountauth.SessionTokens) error {
	accessExpiresAt, err := parseSessionExpiry(tokens.AccessExpiresAt)
	if err != nil {
		return err
	}
	refreshExpiresAt, err := parseSessionExpiry(tokens.ExpiresAt)
	if err != nil {
		return err
	}
	csrf, err := newCSRFToken()
	if err != nil {
		return err
	}
	http.SetCookie(w, sessionCookie(accessCookieName, tokens.AccessToken, accessExpiresAt))
	http.SetCookie(w, sessionCookie(refreshCookieName, tokens.RefreshToken, refreshExpiresAt))
	http.SetCookie(w, csrfCookie(csrf, refreshExpiresAt))
	return nil
}

func (h *Handler) expireSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{accessCookieName, refreshCookieName, csrfCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", Secure: true, HttpOnly: name != csrfCookieName, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	}
}

func sessionCookie(name, value string, expires time.Time) *http.Cookie {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	return &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge}
}

func csrfCookie(value string, expires time.Time) *http.Cookie {
	cookie := sessionCookie(csrfCookieName, value, expires)
	cookie.HttpOnly = false
	return cookie
}

func newCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func parseSessionExpiry(raw string) (time.Time, error) {
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return expiresAt, nil
}

func constantTimeEqual(left, right string) bool {
	return left != "" && right != "" && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func sessionMetadata(r *http.Request, deviceInfo string) accountauth.SessionMetadata {
	ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if comma := strings.Index(ip, ","); comma >= 0 {
		ip = strings.TrimSpace(ip[:comma])
	}
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}
	if ip == "" {
		ip = strings.TrimSpace(r.RemoteAddr)
	}
	return accountauth.SessionMetadata{DeviceInfo: deviceInfo, IP: ip, UserAgent: r.UserAgent()}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return false
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return false
	}
	return true
}

type safeSessionResponse struct {
	Session accountauth.AccountSessionSafe `json:"session"`
}

type safeConversationList struct {
	Items []safeConversation `json:"items"`
}

type safeConversation struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newSafeConversation(conversation domain.Conversation) safeConversation {
	return safeConversation{
		ID:        conversation.ID,
		Title:     conversation.Title,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
