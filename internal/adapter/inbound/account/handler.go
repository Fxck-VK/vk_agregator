// Package account exposes the product account boundary over HTTP.
package account

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/accountoauth"
	miniappauth "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
)

const maxLinkRequestBytes = 8 << 10

type contextKey int

const ctxAccountIDKey contextKey = iota

// Config holds account API auth settings.
type Config struct {
	// AppSecret is the VK Mini App protected key used to verify launch params.
	// When empty, local/dev requests may use X-VK-User-ID.
	AppSecret string
	// LaunchParamsMaxAge bounds replay of VK launch params. Zero disables age
	// validation and is intended only for local tests.
	LaunchParamsMaxAge time.Duration
	// AllowQueryLaunchParams permits the legacy launch_params query fallback.
	// It is dev/test-only because URLs leak through browser and proxy surfaces.
	AllowQueryLaunchParams bool
}

// AccountService is the safe product account boundary.
type AccountService interface {
	Profile(ctx context.Context, accountID uuid.UUID) (accountservice.AccountProfile, error)
	ListIdentities(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]accountservice.AccountIdentitySafe, error)
	LinkVerifiedIdentity(ctx context.Context, actorAccountID, accountID uuid.UUID, login domain.VerifiedAccountLogin) (accountservice.AccountIdentitySafe, error)
	UnlinkIdentity(ctx context.Context, actorAccountID, accountID, identityID uuid.UUID) error
}

// SessionService owns Web/Mobile account sessions. VK launch auth stays a
// separate channel-specific authenticator.
type SessionService interface {
	IssueSession(ctx context.Context, accountID uuid.UUID, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error)
	RefreshSession(ctx context.Context, refreshToken string, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error)
	Logout(ctx context.Context, refreshToken string) error
	RevokeSession(ctx context.Context, accountID, sessionID uuid.UUID) (accountauth.AccountSessionSafe, error)
	ListActiveSessions(ctx context.Context, accountID uuid.UUID, limit int) ([]accountauth.AccountSessionSafe, error)
}

// PasswordService owns email/password verification over an already linked
// email identity.
type PasswordService interface {
	SetPasswordForVerifiedEmail(ctx context.Context, actorAccountID, accountID uuid.UUID, email, password string) error
	ResetPasswordForVerifiedEmail(ctx context.Context, accountID uuid.UUID, email, password string) error
	AuthenticateEmailPassword(ctx context.Context, email, password string) (domain.IdentityResolution, error)
}

// LoginService maps verified method-specific login assertions to accounts.
type LoginService interface {
	ResolveOrCreate(ctx context.Context, login domain.VerifiedAccountLogin) (domain.IdentityResolution, error)
}

// OAuthVerifier owns provider-specific Google/Apple/VK ID/Telegram proof
// verification before AccountService sees an identity.
type OAuthVerifier interface {
	Verify(ctx context.Context, req accountoauth.VerifyRequest) (domain.VerifiedAccountLogin, error)
}

// IdentityLinker owns method-specific verification before identity linking.
type IdentityLinker interface {
	RequestEmailCode(ctx context.Context, accountID uuid.UUID, email string) (accountlink.RequestResult, error)
	VerifyEmailCode(ctx context.Context, accountID uuid.UUID, email, code string) (accountservice.AccountIdentitySafe, error)
	RequestPhoneOTP(ctx context.Context, accountID uuid.UUID, phone string) (accountlink.RequestResult, error)
	VerifyPhoneOTP(ctx context.Context, accountID uuid.UUID, phone, code string) (accountservice.AccountIdentitySafe, error)
}

// Deps are the collaborators needed by the account handler.
type Deps struct {
	Identity  domain.IdentityResolver
	Account   AccountService
	Logins    LoginService
	Sessions  SessionService
	Passwords PasswordService
	OAuth     OAuthVerifier
	Linker    IdentityLinker
	Logger    *slog.Logger
}

// Handler serves /account/* endpoints.
type Handler struct {
	cfg    Config
	deps   Deps
	logger *slog.Logger
}

// NewHandler builds an account API handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{cfg: cfg, deps: deps, logger: logger}
}

// Routes returns the account API router.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /account/me", h.auth(h.getMe))
	mux.HandleFunc("GET /account/identities", h.auth(h.listIdentities))
	mux.HandleFunc("POST /account/identities/link", h.auth(h.linkIdentity))
	mux.HandleFunc("POST /account/identities/email/request-code", h.auth(h.requestEmailCode))
	mux.HandleFunc("POST /account/identities/email/verify", h.auth(h.verifyEmailCode))
	mux.HandleFunc("POST /account/identities/phone/request-otp", h.auth(h.requestPhoneOTP))
	mux.HandleFunc("POST /account/identities/phone/verify", h.auth(h.verifyPhoneOTP))
	mux.HandleFunc("DELETE /account/identities/{id}", h.auth(h.unlinkIdentity))
	mux.HandleFunc("POST /account/identities/{id}/unlink", h.auth(h.unlinkIdentity))
	mux.HandleFunc("GET /account/sessions", h.auth(h.listSessions))
	mux.HandleFunc("POST /account/sessions", h.auth(h.issueSession))
	mux.HandleFunc("POST /account/sessions/refresh", h.refreshSession)
	mux.HandleFunc("POST /account/sessions/logout", h.logoutSession)
	mux.HandleFunc("POST /account/sessions/{id}/revoke", h.auth(h.revokeSession))
	mux.HandleFunc("POST /account/password/set", h.auth(h.setPassword))
	mux.HandleFunc("POST /account/password/login", h.passwordLogin)
	mux.HandleFunc("POST /account/password/request-reset", h.requestPasswordReset)
	mux.HandleFunc("POST /account/password/reset", h.resetPassword)
	mux.HandleFunc("POST /account/oauth/login", h.oauthLogin)
	mux.HandleFunc("POST /account/oauth/link", h.auth(h.oauthLink))
	return mux
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := h.accountIDFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), ctxAccountIDKey, accountID)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) accountIDFromRequest(r *http.Request) (uuid.UUID, error) {
	if h.deps.Identity == nil {
		return uuid.Nil, errors.New("account api: identity resolver is required")
	}
	raw := strings.TrimSpace(r.Header.Get("X-Launch-Params"))
	if raw == "" && h.cfg.AllowQueryLaunchParams {
		raw = strings.TrimSpace(r.URL.Query().Get("launch_params"))
	}
	if raw == "" && h.cfg.AppSecret == "" {
		rawVK := strings.TrimSpace(r.Header.Get("X-VK-User-ID"))
		vkUserID, err := strconv.ParseInt(rawVK, 10, 64)
		if err != nil || vkUserID <= 0 {
			return uuid.Nil, miniappauth.ErrMissingUserID
		}
		return h.resolveVKAccount(r.Context(), vkUserID)
	}
	params, err := miniappauth.VerifyLaunchParams(raw, h.cfg.AppSecret, h.cfg.LaunchParamsMaxAge)
	if err != nil {
		return uuid.Nil, err
	}
	vkUserID, err := miniappauth.VKUserIDFromParams(params)
	if err != nil {
		return uuid.Nil, err
	}
	return h.resolveVKAccount(r.Context(), vkUserID)
}

func (h *Handler) resolveVKAccount(ctx context.Context, vkUserID int64) (uuid.UUID, error) {
	resolution, err := h.deps.Identity.ResolveOrCreate(ctx, domain.IdentityProviderVK, strconv.FormatInt(vkUserID, 10))
	if err != nil {
		return uuid.Nil, err
	}
	if resolution.AccountID == uuid.Nil {
		return uuid.Nil, errors.New("account api: resolver returned empty account id")
	}
	return resolution.AccountID, nil
}

func accountIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	accountID, ok := ctx.Value(ctxAccountIDKey).(uuid.UUID)
	return accountID, ok && accountID != uuid.Nil
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	if h.deps.Account == nil {
		writeError(w, http.StatusInternalServerError, "account unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	profile, err := h.deps.Account.Profile(r.Context(), accountID)
	if err != nil {
		writeError(w, statusForError(err), "account unavailable")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) listIdentities(w http.ResponseWriter, r *http.Request) {
	if h.deps.Account == nil {
		writeError(w, http.StatusInternalServerError, "account unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 50, 100)
	offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 100000)
	items, err := h.deps.Account.ListIdentities(r.Context(), accountID, limit, offset)
	if err != nil {
		writeError(w, statusForError(err), "account unavailable")
		return
	}
	writeJSON(w, http.StatusOK, identitiesResponse{
		Items: items,
		Pagination: paginationDTO{
			Limit:   limit,
			Offset:  offset,
			Count:   len(items),
			HasMore: len(items) == limit,
		},
	})
}

type linkRequest struct {
	Method            domain.AccountLoginMethod `json:"method"`
	ExternalID        string                    `json:"external_id"`
	VerificationToken string                    `json:"verification_token"`
}

func (h *Handler) linkIdentity(w http.ResponseWriter, r *http.Request) {
	var req linkRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid identity link request")
		return
	}
	if strings.TrimSpace(string(req.Method)) == "" || strings.TrimSpace(req.ExternalID) == "" {
		writeError(w, http.StatusBadRequest, "invalid identity link request")
		return
	}
	// This public endpoint intentionally does not trust client JSON as proof of
	// email/phone/OAuth ownership. Method-specific verifier endpoints should
	// validate the proof and call AccountService.LinkVerifiedIdentity.
	if strings.TrimSpace(req.VerificationToken) == "" {
		writeError(w, http.StatusPreconditionRequired, "identity verification required")
		return
	}
	writeError(w, http.StatusNotImplemented, "identity verification flow is not implemented")
}

type emailCodeRequest struct {
	Email string `json:"email"`
}

func (h *Handler) requestEmailCode(w http.ResponseWriter, r *http.Request) {
	if h.deps.Linker == nil {
		writeError(w, http.StatusServiceUnavailable, "email verification unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req emailCodeRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email verification request")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid email verification request")
		return
	}
	result, err := h.deps.Linker.RequestEmailCode(r.Context(), accountID, req.Email)
	if err != nil {
		writeError(w, statusForError(err), "email verification unavailable")
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

type emailVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handler) verifyEmailCode(w http.ResponseWriter, r *http.Request) {
	if h.deps.Linker == nil {
		writeError(w, http.StatusServiceUnavailable, "email verification unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req emailVerifyRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email verification request")
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "invalid email verification request")
		return
	}
	identity, err := h.deps.Linker.VerifyEmailCode(r.Context(), accountID, req.Email, req.Code)
	if err != nil {
		writeError(w, statusForError(err), "email verification failed")
		return
	}
	writeJSON(w, http.StatusCreated, identity)
}

type phoneOTPRequest struct {
	Phone string `json:"phone"`
}

func (h *Handler) requestPhoneOTP(w http.ResponseWriter, r *http.Request) {
	if h.deps.Linker == nil {
		writeError(w, http.StatusServiceUnavailable, "phone verification unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req phoneOTPRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid phone verification request")
		return
	}
	if strings.TrimSpace(req.Phone) == "" {
		writeError(w, http.StatusBadRequest, "invalid phone verification request")
		return
	}
	result, err := h.deps.Linker.RequestPhoneOTP(r.Context(), accountID, req.Phone)
	if err != nil {
		writeError(w, statusForError(err), "phone verification unavailable")
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

type phoneVerifyRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (h *Handler) verifyPhoneOTP(w http.ResponseWriter, r *http.Request) {
	if h.deps.Linker == nil {
		writeError(w, http.StatusServiceUnavailable, "phone verification unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req phoneVerifyRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid phone verification request")
		return
	}
	if strings.TrimSpace(req.Phone) == "" || strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "invalid phone verification request")
		return
	}
	identity, err := h.deps.Linker.VerifyPhoneOTP(r.Context(), accountID, req.Phone, req.Code)
	if err != nil {
		writeError(w, statusForError(err), "phone verification failed")
		return
	}
	writeJSON(w, http.StatusCreated, identity)
}

func (h *Handler) unlinkIdentity(w http.ResponseWriter, r *http.Request) {
	if h.deps.Account == nil {
		writeError(w, http.StatusInternalServerError, "account unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	identityID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil || identityID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid identity id")
		return
	}
	if err := h.deps.Account.UnlinkIdentity(r.Context(), accountID, accountID, identityID); err != nil {
		writeError(w, statusForError(err), "identity unlink failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sessionRequest struct {
	DeviceInfo string `json:"device_info"`
}

type refreshSessionRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceInfo   string `json:"device_info"`
}

type logoutSessionRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordSetRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordLoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceInfo string `json:"device_info"`
}

type passwordResetRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

type oauthRequest struct {
	Provider   domain.IdentityProvider `json:"provider"`
	IDToken    string                  `json:"id_token"`
	AuthData   map[string]string       `json:"auth_data"`
	DeviceInfo string                  `json:"device_info"`
}

func (h *Handler) issueSession(w http.ResponseWriter, r *http.Request) {
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sessionRequest
	if !decodeOptionalJSON(w, r, &req, "invalid session request") {
		return
	}
	tokens, err := h.deps.Sessions.IssueSession(r.Context(), accountID, sessionMetadataFromRequest(r, req.DeviceInfo))
	if err != nil {
		writeError(w, statusForError(err), "session unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, tokens)
}

func (h *Handler) refreshSession(w http.ResponseWriter, r *http.Request) {
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	var req refreshSessionRequest
	if !decodeRequiredJSON(w, r, &req, "invalid refresh request") {
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "invalid refresh request")
		return
	}
	tokens, err := h.deps.Sessions.RefreshSession(r.Context(), req.RefreshToken, sessionMetadataFromRequest(r, req.DeviceInfo))
	if err != nil {
		writeError(w, statusForError(err), "session refresh failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) logoutSession(w http.ResponseWriter, r *http.Request) {
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	var req logoutSessionRequest
	if !decodeRequiredJSON(w, r, &req, "invalid logout request") {
		return
	}
	if err := h.deps.Sessions.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, statusForError(err), "logout failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil || sessionID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := h.deps.Sessions.RevokeSession(r.Context(), accountID, sessionID)
	if err != nil {
		writeError(w, statusForError(err), "session revoke failed")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	if h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 50, 100)
	items, err := h.deps.Sessions.ListActiveSessions(r.Context(), accountID, limit)
	if err != nil {
		writeError(w, statusForError(err), "sessions unavailable")
		return
	}
	writeJSON(w, http.StatusOK, sessionsResponse{
		Items: items,
		Pagination: paginationDTO{
			Limit:   limit,
			Count:   len(items),
			HasMore: len(items) == limit,
		},
	})
}

func (h *Handler) setPassword(w http.ResponseWriter, r *http.Request) {
	if h.deps.Passwords == nil {
		writeError(w, http.StatusServiceUnavailable, "password login unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req passwordSetRequest
	if !decodeRequiredJSON(w, r, &req, "invalid password request") {
		return
	}
	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid password request")
		return
	}
	if err := h.deps.Passwords.SetPasswordForVerifiedEmail(r.Context(), accountID, accountID, req.Email, req.Password); err != nil {
		writeError(w, statusForError(err), "password update failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) passwordLogin(w http.ResponseWriter, r *http.Request) {
	if h.deps.Passwords == nil || h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "password login unavailable")
		return
	}
	var req passwordLoginRequest
	if !decodeRequiredJSON(w, r, &req, "invalid password login request") {
		return
	}
	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid password login request")
		return
	}
	resolution, err := h.deps.Passwords.AuthenticateEmailPassword(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, statusForError(err), "password login failed")
		return
	}
	tokens, err := h.deps.Sessions.IssueSession(r.Context(), resolution.AccountID, sessionMetadataFromRequest(r, req.DeviceInfo))
	if err != nil {
		writeError(w, statusForError(err), "session unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, tokens)
}

func (h *Handler) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if h.deps.Identity == nil || h.deps.Linker == nil {
		writeError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}
	var req emailCodeRequest
	if !decodeRequiredJSON(w, r, &req, "invalid password reset request") {
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "invalid password reset request")
		return
	}
	accountID, err := h.deps.Identity.Resolve(r.Context(), domain.IdentityProviderEmail, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidIdentity) {
			writeJSON(w, http.StatusAccepted, accountlink.RequestResult{Status: "verification_sent"})
			return
		}
		writeError(w, statusForError(err), "password reset unavailable")
		return
	}
	result, err := h.deps.Linker.RequestEmailCode(r.Context(), accountID, email)
	if err != nil {
		writeError(w, statusForError(err), "password reset unavailable")
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	if h.deps.Identity == nil || h.deps.Linker == nil || h.deps.Passwords == nil {
		writeError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}
	var req passwordResetRequest
	if !decodeRequiredJSON(w, r, &req, "invalid password reset request") {
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Code) == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "invalid password reset request")
		return
	}
	accountID, err := h.deps.Identity.Resolve(r.Context(), domain.IdentityProviderEmail, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "password reset failed")
			return
		}
		writeError(w, statusForError(err), "password reset failed")
		return
	}
	if _, err := h.deps.Linker.VerifyEmailCode(r.Context(), accountID, req.Email, req.Code); err != nil {
		writeError(w, statusForError(err), "password reset failed")
		return
	}
	if err := h.deps.Passwords.ResetPasswordForVerifiedEmail(r.Context(), accountID, req.Email, req.NewPassword); err != nil {
		writeError(w, statusForError(err), "password reset failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) oauthLogin(w http.ResponseWriter, r *http.Request) {
	if h.deps.OAuth == nil || h.deps.Logins == nil || h.deps.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth login unavailable")
		return
	}
	req, ok := decodeOAuthRequest(w, r)
	if !ok {
		return
	}
	login, err := h.deps.OAuth.Verify(r.Context(), accountoauth.VerifyRequest{
		Provider: req.Provider,
		IDToken:  req.IDToken,
		AuthData: req.AuthData,
	})
	if err != nil {
		writeError(w, statusForError(err), "oauth login failed")
		return
	}
	resolution, err := h.deps.Logins.ResolveOrCreate(r.Context(), login)
	if err != nil {
		writeError(w, statusForError(err), "oauth login failed")
		return
	}
	tokens, err := h.deps.Sessions.IssueSession(r.Context(), resolution.AccountID, sessionMetadataFromRequest(r, req.DeviceInfo))
	if err != nil {
		writeError(w, statusForError(err), "session unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, tokens)
}

func (h *Handler) oauthLink(w http.ResponseWriter, r *http.Request) {
	if h.deps.OAuth == nil || h.deps.Account == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth link unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	req, decoded := decodeOAuthRequest(w, r)
	if !decoded {
		return
	}
	login, err := h.deps.OAuth.Verify(r.Context(), accountoauth.VerifyRequest{
		Provider: req.Provider,
		IDToken:  req.IDToken,
		AuthData: req.AuthData,
	})
	if err != nil {
		writeError(w, statusForError(err), "oauth link failed")
		return
	}
	identity, err := h.deps.Account.LinkVerifiedIdentity(r.Context(), accountID, accountID, login)
	if err != nil {
		writeError(w, statusForError(err), "oauth link failed")
		return
	}
	writeJSON(w, http.StatusCreated, identity)
}

type identitiesResponse struct {
	Items      []accountservice.AccountIdentitySafe `json:"items"`
	Pagination paginationDTO                        `json:"pagination"`
}

type sessionsResponse struct {
	Items      []accountauth.AccountSessionSafe `json:"items"`
	Pagination paginationDTO                    `json:"pagination"`
}

type paginationDTO struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

func decodeOAuthRequest(w http.ResponseWriter, r *http.Request) (oauthRequest, bool) {
	var req oauthRequest
	if !decodeRequiredJSON(w, r, &req, "invalid oauth request") {
		return oauthRequest{}, false
	}
	provider := domain.NormalizeIdentityProvider(req.Provider)
	if provider == "" {
		writeError(w, http.StatusBadRequest, "invalid oauth request")
		return oauthRequest{}, false
	}
	req.Provider = provider
	return req, true
}

func parseBoundedInt(raw string, def, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		value = def
	}
	if value > max {
		value = max
	}
	return value
}

func sessionMetadataFromRequest(r *http.Request, deviceInfo string) accountauth.SessionMetadata {
	return accountauth.SessionMetadata{
		DeviceInfo: deviceInfo,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	}
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma >= 0 {
			return strings.TrimSpace(forwarded[:comma])
		}
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any, message string) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	return decodeRequiredJSON(w, r, dst, message)
}

func decodeRequiredJSON(w http.ResponseWriter, r *http.Request, dst any, message string) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, message)
			return false
		}
		writeError(w, http.StatusBadRequest, message)
		return false
	}
	return true
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrAccountMergeRequiresConfirmation):
		return http.StatusConflict
	case errors.Is(err, domain.ErrInvalidIdentity):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrAccountIdentityOwnershipRequired):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrAccountLastIdentity):
		return http.StatusConflict
	case errors.Is(err, domain.ErrUnverifiedLogin):
		return http.StatusPreconditionRequired
	case errors.Is(err, accountauth.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, accountauth.ErrInvalidSession):
		return http.StatusUnauthorized
	case errors.Is(err, accountauth.ErrSessionExpired):
		return http.StatusGone
	case errors.Is(err, accountauth.ErrSessionStoreUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, accountauth.ErrPasswordStoreUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, accountauth.ErrInvalidPasswordLogin):
		return http.StatusUnauthorized
	case errors.Is(err, accountauth.ErrWeakPassword):
		return http.StatusBadRequest
	case errors.Is(err, accountlink.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, accountlink.ErrInvalidCode):
		return http.StatusBadRequest
	case errors.Is(err, accountlink.ErrExpiredCode):
		return http.StatusGone
	case errors.Is(err, accountlink.ErrDeliveryUnavailable), errors.Is(err, accountlink.ErrSMSDeliveryUnavailable), errors.Is(err, accountlink.ErrMissingDependency):
		return http.StatusServiceUnavailable
	case errors.Is(err, accountoauth.ErrUnsupportedProvider):
		return http.StatusBadRequest
	case errors.Is(err, accountoauth.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, accountoauth.ErrInvalidAssertion):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
