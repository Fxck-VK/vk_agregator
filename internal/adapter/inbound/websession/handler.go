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
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/preparedjobexpiry"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
)

const (
	accessCookieName                = "nh_access"
	refreshCookieName               = "nh_refresh"
	csrfCookieName                  = "nh_csrf"
	maxRequestBytes                 = 8 << 10
	maxConversationMessageSeq int64 = 1<<63 - 1
	webImageArtifactURLTTL          = time.Minute
	// ImageArtifactRedirectOriginHeader is set only on a redirect that was
	// validated against the server-side object-storage policy. The platform
	// proxy consumes it to bind the signed Location to that same origin and
	// deliberately never forwards it to the browser.
	ImageArtifactRedirectOriginHeader = "X-NeiroHub-Image-Artifact-Origin"
)

type principalContextKey struct{}

// Config contains browser adapter settings.
type Config struct {
	WebOrigin                   string
	ImageModels                 []imagegeneration.PublicModel
	ImageArtifactRedirectPolicy ImageArtifactRedirectPolicy
}

// ImageArtifactRedirectPolicy binds signed artifact redirects to the one
// configured object-store origin. It is intentionally constructed only from
// trusted server configuration, never a client request or artifact metadata.
//
// HTTP is never accepted implicitly. It can be enabled solely for an explicit
// loopback development endpoint so a local MinIO test setup remains possible.
type ImageArtifactRedirectPolicy struct {
	scheme                   string
	hostname                 string
	port                     string
	allowVirtualHostedBucket bool
}

// NewImageArtifactRedirectPolicy normalizes the configured S3 endpoint into a
// strict redirect allowlist. The endpoint follows the same host-or-URL shape as
// the object-store client. An insecure endpoint is allowed only when all of
// the following are true: the operator explicitly opts in, APP_ENV is
// development, and the endpoint is loopback-only.
func NewImageArtifactRedirectPolicy(endpoint string, useSSL bool, addressingStyle, environment string, allowInsecureHTTP bool) (ImageArtifactRedirectPolicy, error) {
	parsed, err := parseImageArtifactStorageEndpoint(endpoint, useSSL)
	if err != nil {
		return ImageArtifactRedirectPolicy{}, err
	}
	if parsed.scheme == "http" && (!allowInsecureHTTP || !strings.EqualFold(strings.TrimSpace(environment), "development") || !isLoopbackImageArtifactHost(parsed.hostname)) {
		return ImageArtifactRedirectPolicy{}, errors.New("web image artifact redirects require HTTPS outside explicit local development")
	}

	style := strings.ToLower(strings.TrimSpace(addressingStyle))
	allowVirtualHostedBucket := false
	switch style {
	case "", "path":
	case "auto", "virtual-hosted", "virtual", "dns":
		allowVirtualHostedBucket = true
	default:
		return ImageArtifactRedirectPolicy{}, errors.New("web image artifact redirect policy has an invalid addressing style")
	}

	return ImageArtifactRedirectPolicy{
		scheme:                   parsed.scheme,
		hostname:                 parsed.hostname,
		port:                     parsed.port,
		allowVirtualHostedBucket: allowVirtualHostedBucket,
	}, nil
}

// Allows reports whether rawURL is a signed URL for the configured object-store
// origin and the supplied stored bucket. It does not validate a signature;
// signing remains the object-store client's responsibility.
func (p ImageArtifactRedirectPolicy) Allows(rawURL, bucket string) bool {
	_, ok := p.originFor(rawURL, bucket)
	return ok
}

func (p ImageArtifactRedirectPolicy) originFor(rawURL, bucket string) (string, bool) {
	if p.scheme == "" || p.hostname == "" {
		return "", false
	}
	parsed, err := parseImageArtifactRedirectURL(rawURL)
	if err != nil || parsed.scheme != p.scheme || parsed.port != p.port {
		return "", false
	}
	if parsed.hostname == p.hostname {
		return imageArtifactOrigin(parsed.scheme, parsed.hostname, parsed.port), true
	}
	if !p.allowVirtualHostedBucket {
		return "", false
	}
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	if bucket == "" || parsed.hostname != bucket+"."+p.hostname {
		return "", false
	}
	return imageArtifactOrigin(parsed.scheme, parsed.hostname, parsed.port), true
}

type imageArtifactURLParts struct {
	scheme   string
	hostname string
	port     string
}

func parseImageArtifactStorageEndpoint(rawEndpoint string, useSSL bool) (imageArtifactURLParts, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if endpoint == "" {
		return imageArtifactURLParts{}, errors.New("web image artifact redirect policy requires an object-store endpoint")
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed == nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return imageArtifactURLParts{}, errors.New("web image artifact redirect policy requires a plain object-store origin")
	}
	return parseImageArtifactURL(endpoint)
}

func parseImageArtifactRedirectURL(rawURL string) (imageArtifactURLParts, error) {
	return parseImageArtifactURL(rawURL)
}

func parseImageArtifactURL(rawURL string) (imageArtifactURLParts, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery == "" && parsed.ForceQuery || parsed.Fragment != "" {
		return imageArtifactURLParts{}, errors.New("invalid web image artifact URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return imageArtifactURLParts{}, errors.New("web image artifact URL must use HTTP or HTTPS")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return imageArtifactURLParts{}, errors.New("web image artifact URL must include a host")
	}
	port := parsed.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	return imageArtifactURLParts{scheme: scheme, hostname: hostname, port: port}, nil
}

func isLoopbackImageArtifactHost(hostname string) bool {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(hostname)
	return err == nil && address.IsLoopback()
}

func imageArtifactOrigin(scheme, hostname, port string) string {
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host
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

// ImageJobService records a server-resolved image job and later activates that
// exact stored job. Prepare never invokes a provider or charges a balance;
// activation is a separate explicit browser action.
type ImageJobService interface {
	PrepareAccountJob(ctx context.Context, input joborchestrator.PrepareAccountJobInput) (*domain.Job, error)
	ActivatePreparedAccountJob(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error)
}

// ImageBalanceService returns the backend-owned balance used in a preparation
// response. It must not accept a client-provided balance or price.
type ImageBalanceService interface {
	BalanceForEstimate(ctx context.Context, accountID uuid.UUID) (int64, error)
}

// ImageJobReader reads one job only through the canonical account scope.
type ImageJobReader interface {
	GetByIDForAccount(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error)
}

// ImageJobIdempotencyReader resolves a prepared web job only through its exact
// canonical account owner. A foreign key remains indistinguishable from a
// missing key, so a browser retry cannot learn another account's job state.
type ImageJobIdempotencyReader interface {
	GetByIdempotencyKeyForAccount(ctx context.Context, accountID uuid.UUID, key string) (*domain.Job, error)
}

// ImageJobPrepareLimiter is the shared account-scoped rate limiter for a new
// durable image-job preparation. It deliberately accepts only a server-derived
// key so browser-controlled fields can never select another account's quota.
type ImageJobPrepareLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// ImageJobHistoryReader performs a bounded keyset read for one account.
type ImageJobHistoryReader interface {
	ListCursor(ctx context.Context, filter domain.JobFilter, limit int, after *domain.JobCursor) ([]*domain.Job, error)
}

// ImageJobExpiryReconciler durably expires an unconfirmed web image
// preparation before browser reads can report its status. It is account-scoped
// so a request cannot discover or mutate another account's job.
type ImageJobExpiryReconciler interface {
	ReconcileAccount(ctx context.Context, accountID uuid.UUID, limit int) (preparedjobexpiry.Result, error)
	ReconcileJob(ctx context.Context, accountID, jobID uuid.UUID) (bool, error)
}

// ImageResultReader returns already-moderated output metadata for an exact
// account-owned completed job.
type ImageResultReader interface {
	GetResult(ctx context.Context, accountID, jobID uuid.UUID) (resultservice.Result, error)
}

// ImageArtifactReader resolves an artifact only through its canonical account
// owner. It deliberately never exposes an unscoped artifact lookup to the web
// adapter.
type ImageArtifactReader interface {
	GetByIDForAccount(ctx context.Context, accountID, artifactID uuid.UUID) (*domain.Artifact, error)
}

// ImageArtifactURLSigner creates a short-lived storage URL only after the web
// adapter has confirmed ownership, job scope, result readiness and moderation.
type ImageArtifactURLSigner interface {
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// ImageArtifactObjectReader loads a verified artifact from private object
// storage. Implementations may also be URL signers, which lets the web adapter
// keep its legacy redirect fallback when a deployment has not yet enabled
// same-origin artifact delivery.
type ImageArtifactObjectReader interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// WebChatJobCreator queues a normalized text job through the shared
// orchestrator. The adapter deliberately depends only on job creation.
type WebChatJobCreator interface {
	CreateJob(ctx context.Context, input joborchestrator.CreateJobInput) (*domain.Job, error)
}

// WebChatMessageLimiter enforces the shared per-account web chat quota.
type WebChatMessageLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Deps are services shared with other account adapters.
type Deps struct {
	Authenticator          PrincipalAuthenticator
	Sessions               SessionService
	Passwords              PasswordService
	Account                AccountService
	Conversations          domain.ConversationRepository
	ImageJobs              ImageJobService
	ImageBalance           ImageBalanceService
	ImagePricing           imagegeneration.SnapshotCatalog
	ImageJobReader         ImageJobReader
	ImageJobIdempotency    ImageJobIdempotencyReader
	ImageJobPrepareLimiter ImageJobPrepareLimiter
	ImageJobHistory        ImageJobHistoryReader
	ImageJobExpiry         ImageJobExpiryReconciler
	ImageResults           ImageResultReader
	ImageArtifacts         ImageArtifactReader
	ImageArtifactURLSigner ImageArtifactURLSigner
	WebChatJobs            WebChatJobCreator
	WebChatMessageLimiter  WebChatMessageLimiter
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
	mux.HandleFunc("GET /web/v1/conversations/{conversationID}", h.requirePrincipal(h.getConversation))
	mux.HandleFunc("GET /web/v1/conversations/{conversationID}/messages", h.requirePrincipal(h.listConversationMessages))
	mux.HandleFunc("GET /web/v1/image-models", h.requirePrincipal(h.listImageModels))
	mux.HandleFunc("GET /web/v1/image-jobs", h.requirePrincipal(h.listImageJobs))
	mux.HandleFunc("GET /web/v1/image-jobs/{jobID}", h.requirePrincipal(h.getImageJob))
	mux.HandleFunc("GET /web/v1/image-jobs/{jobID}/result", h.requirePrincipal(h.getImageJobResult))
	mux.HandleFunc("GET /web/v1/image-artifacts/{artifactID}", h.requirePrincipal(h.getImageArtifact))
	mux.HandleFunc("POST /web/v1/conversations", h.requireUnsafePrincipal(h.createConversation))
	mux.HandleFunc("POST /web/v1/conversations/{conversationID}/messages", h.requireUnsafePrincipal(h.createConversationMessage))
	mux.HandleFunc("PATCH /web/v1/conversations/{conversationID}", h.requireUnsafePrincipal(h.renameConversation))
	mux.HandleFunc("DELETE /web/v1/conversations/{conversationID}", h.requireUnsafePrincipal(h.archiveConversation))
	mux.HandleFunc("POST /web/v1/image-jobs/prepare", h.requireUnsafePrincipal(h.prepareImageJob))
	mux.HandleFunc("POST /web/v1/image-jobs/{jobID}/activate", h.requireUnsafePrincipal(h.activateImageJob))
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

func (h *Handler) listImageModels(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.ImagePricing == nil || len(h.cfg.ImageModels) == 0 {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	items := make([]safeImageModel, 0, len(h.cfg.ImageModels))
	for _, model := range h.cfg.ImageModels {
		safeModel, ok := newSafeImageModel(model)
		if !ok {
			continue
		}
		items = append(items, safeModel)
	}
	writeJSON(w, http.StatusOK, safeImageModelList{Items: items})
}

func (h *Handler) prepareImageJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobs == nil || h.deps.ImageBalance == nil || h.deps.ImagePricing == nil || h.deps.ImageJobIdempotency == nil || h.deps.ImageJobPrepareLimiter == nil || len(h.cfg.ImageModels) == 0 {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	var req struct {
		Prompt       string `json:"prompt"`
		ModelID      string `json:"model_id"`
		ImageQuality string `json:"image_quality"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid image generation request")
		return
	}
	idempotencyKey, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || idempotencyKey == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid idempotency key")
		return
	}
	resolver := imagegeneration.NewResolver(h.cfg.ImageModels, h.deps.ImagePricing)
	publicIntent, err := resolver.ResolvePublic(imagegeneration.Request{
		ModelID: strings.TrimSpace(req.ModelID),
		Quality: strings.TrimSpace(req.ImageQuality),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image generation request")
		return
	}
	if existing, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, idempotencyKey.String()); err == nil {
		safeJob, ok := preparedWebImageJobReplay(existing, principal.AccountID, idempotencyKey.String(), req.Prompt, publicIntent)
		if !ok {
			writeError(w, http.StatusConflict, "image generation preparation conflict")
			return
		}
		balance, err := h.deps.ImageBalance.BalanceForEstimate(r.Context(), principal.AccountID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
		writeJSON(w, http.StatusCreated, safeImageJobPreparation{
			Job:       safeJob,
			Balance:   balance,
			CanAfford: balance >= safeJob.CostEstimate,
		})
		return
	} else if !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	allowed, err := h.deps.ImageJobPrepareLimiter.Allow(r.Context(), "account:"+principal.AccountID.String())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "image generation preparation rate limited")
		return
	}

	resolution, err := resolver.Resolve(imagegeneration.Request{
		ModelID: strings.TrimSpace(req.ModelID),
		Quality: strings.TrimSpace(req.ImageQuality),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image generation request")
		return
	}
	balance, err := h.deps.ImageBalance.BalanceForEstimate(r.Context(), principal.AccountID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	params, err := json.Marshal(webImageJobParams{
		Prompt:       req.Prompt,
		ModelID:      resolution.Worker.ModelID,
		ModelName:    resolution.Worker.ModelName,
		Provider:     resolution.Worker.Provider,
		ModelCode:    resolution.Worker.ModelCode,
		Size:         resolution.Worker.Size,
		Resolution:   resolution.Worker.Resolution,
		ImageQuality: resolution.Worker.ImageQuality,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	job, err := h.deps.ImageJobs.PrepareAccountJob(r.Context(), joborchestrator.PrepareAccountJobInput{
		AccountID:           principal.AccountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      idempotencyKey.String(),
		CorrelationID:       "web-image:" + idempotencyKey.String(),
		Params:              params,
		CostEstimateCredits: resolution.PricingSnapshot.InternalCredits,
		PricingSnapshot:     resolution.PricingSnapshot,
	})
	if errors.Is(err, domain.ErrPreparedJobLimitExceeded) {
		writeError(w, http.StatusTooManyRequests, "image generation preparation rate limited")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		// The preflight can race with the first request for this key. Re-read only
		// through the authenticated account scope so a concurrent, valid stored
		// preparation is returned with its immutable price rather than becoming a
		// false 409 after the catalog changed.
		if existing, lookupErr := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, idempotencyKey.String()); lookupErr == nil {
			safeJob, ok := preparedWebImageJobReplay(existing, principal.AccountID, idempotencyKey.String(), req.Prompt, publicIntent)
			if !ok {
				writeError(w, http.StatusConflict, "image generation preparation conflict")
				return
			}
			balance, balanceErr := h.deps.ImageBalance.BalanceForEstimate(r.Context(), principal.AccountID)
			if balanceErr != nil {
				writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
				return
			}
			writeJSON(w, http.StatusCreated, safeImageJobPreparation{
				Job:       safeJob,
				Balance:   balance,
				CanAfford: balance >= safeJob.CostEstimate,
			})
			return
		} else if !errors.Is(lookupErr, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
		writeError(w, http.StatusConflict, "image generation preparation conflict")
		return
	}
	if err != nil || job == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	safeJob, ok := newSafeImageJob(job)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, safeImageJobPreparation{
		Job:       safeJob,
		Balance:   balance,
		CanAfford: balance >= safeJob.CostEstimate,
	})
}

func (h *Handler) activateImageJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobs == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	jobID, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid image job id")
		return
	}
	job, err := h.deps.ImageJobs.ActivatePreparedAccountJob(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image job not found")
		return
	}
	if err != nil && !errors.Is(err, domain.ErrInsufficientCredits) {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, "image generation activation conflict")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	safeJob, ok := newSafeImageJob(job)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	status := http.StatusOK
	if errors.Is(err, domain.ErrInsufficientCredits) {
		status = http.StatusPaymentRequired
	}
	writeJSON(w, status, safeImageJobActivation{Job: safeJob})
}

func (h *Handler) getImageJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobReader == nil || h.deps.ImageJobExpiry == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	jobID, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid image job id")
		return
	}
	if _, err := h.deps.ImageJobExpiry.ReconcileJob(r.Context(), principal.AccountID, jobID); err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) || job == nil {
		writeError(w, http.StatusNotFound, "image job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	safeJob, ok := newSafeImageJob(job)
	if !ok {
		writeError(w, http.StatusNotFound, "image job not found")
		return
	}
	writeJSON(w, http.StatusOK, safeImageJobActivation{Job: safeJob})
}

func (h *Handler) listImageJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobHistory == nil || h.deps.ImageJobExpiry == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	limitValues, limitProvided := r.URL.Query()["limit"]
	limit, err := imageJobHistoryLimit(limitValues, limitProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image job limit")
		return
	}
	cursorValues, cursorProvided := r.URL.Query()["cursor"]
	after, err := imageJobHistoryCursor(cursorValues, cursorProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image job cursor")
		return
	}
	if _, err := h.deps.ImageJobExpiry.ReconcileAccount(r.Context(), principal.AccountID, preparedjobexpiry.DefaultAccountReconcileLimit); err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	filter := domain.JobFilter{
		AccountID: &principal.AccountID,
		Source:    "web",
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
	}
	jobs, err := h.deps.ImageJobHistory.ListCursor(r.Context(), filter, limit+1, after)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	if reconciled, err := h.reconcileVisiblePreparedImageJobs(r.Context(), principal.AccountID, jobs, time.Now()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	} else if reconciled {
		jobs, err = h.deps.ImageJobHistory.ListCursor(r.Context(), filter, limit+1, after)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	items := make([]safeImageJob, 0, len(jobs))
	for _, job := range jobs {
		safeJob, ok := newSafeImageJob(job)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
		items = append(items, safeJob)
	}
	var nextCursor *string
	if hasMore {
		cursor, ok := encodeImageJobHistoryCursor(domain.JobCursor{
			CreatedAt: jobs[len(jobs)-1].CreatedAt,
			ID:        jobs[len(jobs)-1].ID,
		})
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
		nextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, safeImageJobList{Items: items, HasMore: hasMore, NextCursor: nextCursor})
}

// reconcileVisiblePreparedImageJobs closes the small race between the bounded
// account reconciliation and the history read. A legacy account can have more
// stale rows than the account pass limit; reconciling only due jobs visible in
// this page ensures the browser never receives a stale prepared status while
// keeping every request bounded by the page size.
func (h *Handler) reconcileVisiblePreparedImageJobs(ctx context.Context, accountID uuid.UUID, jobs []*domain.Job, now time.Time) (bool, error) {
	reconciled := false
	for _, job := range jobs {
		if job == nil || job.Status != domain.JobStatusPrepared || job.ExpiresAt == nil || job.ExpiresAt.After(now) {
			continue
		}
		if _, err := h.deps.ImageJobExpiry.ReconcileJob(ctx, accountID, job.ID); err != nil {
			return false, err
		}
		reconciled = true
	}
	return reconciled, nil
}

func (h *Handler) getImageJobResult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobReader == nil || h.deps.ImageResults == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	jobID, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid image job id")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) || job == nil {
		writeError(w, http.StatusNotFound, "image job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	if _, ok := newSafeImageJob(job); !ok {
		writeError(w, http.StatusNotFound, "image job not found")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image result not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	safeResult, ok := newSafeImageJobResult(result, jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "image result not found")
		return
	}
	writeJSON(w, http.StatusOK, safeResult)
}

// getImageArtifact verifies the complete account-owned result chain before
// delivering the output. When the configured store can read objects, the
// bytes are returned from this same origin so browser CSP never needs to trust
// a private object-store hostname. Older signer-only deployments retain the
// strictly attested redirect fallback.
func (h *Handler) getImageArtifact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageArtifacts == nil || h.deps.ImageJobReader == nil || h.deps.ImageResults == nil || h.deps.ImageArtifactURLSigner == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	artifactID, err := uuid.Parse(r.PathValue("artifactID"))
	if err != nil || artifactID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid image artifact id")
		return
	}
	artifact, err := h.deps.ImageArtifacts.GetByIDForAccount(r.Context(), principal.AccountID, artifactID)
	if errors.Is(err, domain.ErrNotFound) || artifact == nil {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	if artifact.ID != artifactID || artifact.OwnerAccountID != principal.AccountID || artifact.JobID == nil || *artifact.JobID == uuid.Nil || artifact.Kind != domain.ArtifactKindOutput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady || strings.TrimSpace(artifact.StorageBucket) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, *artifact.JobID)
	if errors.Is(err, domain.ErrNotFound) || job == nil {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	if _, ok := newSafeImageJob(job); !ok {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), principal.AccountID, job.ID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	safeResult, ok := newSafeImageJobResult(result, job.ID)
	if !ok || !safeResultContainsArtifact(safeResult, artifactID) {
		writeError(w, http.StatusNotFound, "image artifact not found")
		return
	}
	if objectReader, ok := h.deps.ImageArtifactURLSigner.(ImageArtifactObjectReader); ok {
		data, err := objectReader.GetObject(r.Context(), artifact.StorageBucket, artifact.StorageKey)
		if err != nil || len(data) == 0 || int64(len(data)) != artifact.SizeBytes {
			writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
			return
		}
		w.Header().Set("Content-Type", artifact.MimeType)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}
	signedURL, err := h.deps.ImageArtifactURLSigner.PresignedGetURL(r.Context(), artifact.StorageBucket, artifact.StorageKey, webImageArtifactURLTTL)
	redirectOrigin, allowed := h.cfg.ImageArtifactRedirectPolicy.originFor(signedURL, artifact.StorageBucket)
	if err != nil || !allowed {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return
	}
	w.Header().Set(ImageArtifactRedirectOriginHeader, redirectOrigin)
	http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
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
	conversations, err := h.deps.Conversations.ListActiveByAccountSource(r.Context(), principal.AccountID, domain.ConversationSourceWeb, limit, 0)
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

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conversationID, err := uuid.Parse(strings.TrimSpace(r.PathValue("conversationID")))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	conversation, err := h.deps.Conversations.GetByIDForAccount(r.Context(), principal.AccountID, conversationID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && conversation != nil && (conversation.Source != domain.ConversationSourceWeb || conversation.Status != domain.ConversationActive)) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if err != nil || conversation == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	writeJSON(w, http.StatusOK, newSafeConversation(*conversation))
}

const maxConversationTitleRunes = 120

type renameConversationRequest struct {
	Title string `json:"title"`
}

func validConversationTitle(raw string) (string, bool) {
	title := strings.TrimSpace(raw)
	return title, title != "" && utf8.RuneCountInString(title) <= maxConversationTitleRunes
}

func (h *Handler) renameConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conversationID, err := uuid.Parse(strings.TrimSpace(r.PathValue("conversationID")))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	var req renameConversationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	title, valid := validConversationTitle(req.Title)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid conversation title")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	conversation, err := h.deps.Conversations.RenameActiveConversationForAccount(r.Context(), principal.AccountID, conversationID, domain.ConversationSourceWeb, title)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if err != nil || conversation == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	writeJSON(w, http.StatusOK, newSafeConversation(*conversation))
}

func (h *Handler) archiveConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conversationID, err := uuid.Parse(strings.TrimSpace(r.PathValue("conversationID")))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	err = h.deps.Conversations.ArchiveConversationForAccount(r.Context(), principal.AccountID, conversationID, domain.ConversationSourceWeb)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listConversationMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conversationID, err := uuid.Parse(r.PathValue("conversationID"))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	afterSeqValues, afterSeqProvided := r.URL.Query()["after_seq"]
	afterSeq, err := conversationMessageAfterSeq(afterSeqValues, afterSeqProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message cursor")
		return
	}
	beforeSeqValues, beforeSeqProvided := r.URL.Query()["before_seq"]
	beforeSeq, err := conversationMessageBeforeSeq(beforeSeqValues, beforeSeqProvided)
	if err != nil || (afterSeqProvided && beforeSeqProvided) {
		writeError(w, http.StatusBadRequest, "invalid message cursor")
		return
	}
	limitValues, limitProvided := r.URL.Query()["limit"]
	limit, err := conversationMessageLimit(limitValues, limitProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message limit")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	conversation, err := h.deps.Conversations.GetByIDForAccount(r.Context(), principal.AccountID, conversationID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && conversation != nil && conversation.Source != domain.ConversationSourceWeb) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if err != nil || conversation == nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	var messages []*domain.ConversationMessage
	var hasMoreBefore *bool
	if beforeSeqProvided || !afterSeqProvided {
		if !beforeSeqProvided {
			beforeSeq = maxConversationMessageSeq
		}
		var hasMore bool
		messages, hasMore, err = h.listRecentConversationMessages(r.Context(), conversation.ID, beforeSeq, limit)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
			return
		}
		hasMoreBefore = &hasMore
	} else {
		messages, err = h.deps.Conversations.ListMessagesAfter(r.Context(), conversation.ID, afterSeq, limit)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
		return
	}
	items := make([]safeConversationMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.ConversationID != conversation.ID {
			writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
			return
		}
		safeMessage, ok := newSafeConversationMessage(*message)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "conversations unavailable")
			return
		}
		items = append(items, safeMessage)
	}
	writeJSON(w, http.StatusOK, safeConversationMessageList{Items: items, HasMoreBefore: hasMoreBefore})
}

func (h *Handler) listRecentConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]*domain.ConversationMessage, bool, error) {
	messages, err := h.deps.Conversations.ListRecentMessagesBefore(ctx, conversationID, beforeSeq, 0, limit+1)
	if err != nil {
		return nil, false, err
	}
	hasMoreBefore := len(messages) > limit
	if hasMoreBefore {
		messages = messages[1:]
	}
	return messages, hasMoreBefore, nil
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
		TitleOrigin:      domain.ConversationTitleOriginAutoPending,
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

func (h *Handler) createConversationMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conversationID, err := uuid.Parse(strings.TrimSpace(r.PathValue("conversationID")))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid chat message request")
		return
	}
	idempotencyKey, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || idempotencyKey == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid idempotency key")
		return
	}
	if h.deps.Conversations == nil || h.deps.WebChatJobs == nil || h.deps.WebChatMessageLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	conversation, err := h.deps.Conversations.GetByIDForAccount(r.Context(), principal.AccountID, conversationID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && conversation != nil && (conversation.AccountID != principal.AccountID || conversation.Source != domain.ConversationSourceWeb || conversation.Status != domain.ConversationActive)) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if err != nil || conversation == nil {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	allowed, err := h.deps.WebChatMessageLimiter.Allow(r.Context(), "account:"+principal.AccountID.String())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "chat message rate limited")
		return
	}
	model, ok := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, "")
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	orchestrationKey := "web-chat:" + principal.AccountID.String() + ":" + idempotencyKey.String()
	params, err := json.Marshal(webChatJobParams{
		Prompt:             req.Prompt,
		ModelID:            model.ModelID,
		ModelName:          model.ModelName,
		Provider:           model.Provider,
		ModelCode:          model.ModelCode,
		ConversationID:     conversation.ID.String(),
		ConversationSource: domain.ConversationSourceWeb,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	job, err := h.deps.WebChatJobs.CreateJob(r.Context(), joborchestrator.CreateJobInput{
		UserID:                     uuid.Nil,
		AccountID:                  principal.AccountID,
		Source:                     "web",
		ChannelContext:             &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:                 domain.ResultModeAccountHistory,
		VKPeerID:                   0,
		CommandID:                  uuid.Nil,
		Operation:                  domain.OperationTextGenerate,
		Modality:                   domain.ModalityText,
		IdempotencyKey:             orchestrationKey,
		CorrelationID:              orchestrationKey,
		Params:                     params,
		ConversationTitleRequested: conversation.TitleOrigin == domain.ConversationTitleOriginAutoPending,
	})
	switch {
	case errors.Is(err, domain.ErrActiveJobLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "chat message rate limited")
		return
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeError(w, http.StatusPaymentRequired, "insufficient credits")
		return
	case errors.Is(err, domain.ErrCostCapExceeded):
		writeError(w, http.StatusBadRequest, "invalid chat message request")
		return
	case errors.Is(err, domain.ErrCapacityDegraded):
		w.Header().Set("Retry-After", "30")
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	expectedParams := webChatJobParams{
		Prompt:             req.Prompt,
		ModelID:            model.ModelID,
		ModelName:          model.ModelName,
		Provider:           model.Provider,
		ModelCode:          model.ModelCode,
		ConversationID:     conversation.ID.String(),
		ConversationSource: domain.ConversationSourceWeb,
	}
	if !validPersistedWebChatJob(job, principal.AccountID, orchestrationKey, expectedParams) {
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
		return
	}
	writePersistedWebChatJob(w, job)
}

func validPersistedWebChatJob(job *domain.Job, accountID uuid.UUID, orchestrationKey string, expectedParams webChatJobParams) bool {
	if job == nil || job.ID == uuid.Nil || job.UserID != uuid.Nil || job.AccountID != accountID || job.Source != "web" ||
		job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" ||
		job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil || job.VKPeerID != 0 || job.CommandID != uuid.Nil ||
		job.OperationType != domain.OperationTextGenerate || job.Modality != domain.ModalityText ||
		len(job.InputArtifactIDs) != 0 ||
		job.IdempotencyKey != orchestrationKey || job.CorrelationID != orchestrationKey {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(job.Params)))
	decoder.DisallowUnknownFields()
	var persistedParams webChatJobParams
	if err := decoder.Decode(&persistedParams); err != nil || persistedParams != expectedParams {
		return false
	}
	var extra struct{}
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func writePersistedWebChatJob(w http.ResponseWriter, job *domain.Job) {
	safeJob := safeWebChatJob{JobID: job.ID, Status: job.Status}
	switch job.Status {
	case domain.JobStatusQueued:
		writeJSON(w, http.StatusCreated, safeJob)
	case domain.JobStatusReceived,
		domain.JobStatusValidated,
		domain.JobStatusCreditsReserved,
		domain.JobStatusDispatchingProvider,
		domain.JobStatusProviderSubmitted,
		domain.JobStatusProviderPending,
		domain.JobStatusProviderProcessing,
		domain.JobStatusProviderSucceeded,
		domain.JobStatusPostprocessing,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
		domain.JobStatusFailedRetryable,
		domain.JobStatusSucceeded:
		writeJSON(w, http.StatusOK, safeJob)
	case domain.JobStatusAwaitingPayment:
		writeError(w, http.StatusPaymentRequired, "insufficient credits")
	case domain.JobStatusRejected,
		domain.JobStatusFailedTerminal,
		domain.JobStatusCancelled,
		domain.JobStatusExpired,
		domain.JobStatusRefunded:
		writeError(w, http.StatusConflict, "chat message unavailable")
	default:
		writeError(w, http.StatusServiceUnavailable, "chat message unavailable")
	}
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

func imageJobHistoryLimit(values []string, provided bool) (int, error) {
	const (
		defaultImageJobHistoryLimit = 20
		maxImageJobHistoryLimit     = 50
	)
	if !provided {
		return defaultImageJobHistoryLimit, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, errors.New("invalid image job limit")
	}
	limit, err := strconv.Atoi(values[0])
	if err != nil || limit < 1 || limit > maxImageJobHistoryLimit {
		return 0, errors.New("invalid image job limit")
	}
	return limit, nil
}

func imageJobHistoryCursor(values []string, provided bool) (*domain.JobCursor, error) {
	const maxImageJobHistoryCursorLength = 256
	if !provided {
		return nil, nil
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > maxImageJobHistoryCursorLength {
		return nil, errors.New("invalid image job cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(values[0])
	if err != nil || len(raw) == 0 || len(raw) > maxImageJobHistoryCursorLength {
		return nil, errors.New("invalid image job cursor")
	}
	var payload struct {
		CreatedAt string `json:"created_at"`
		ID        string `json:"id"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errors.New("invalid image job cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid image job cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil || createdAt.IsZero() {
		return nil, errors.New("invalid image job cursor")
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil || id == uuid.Nil {
		return nil, errors.New("invalid image job cursor")
	}
	return &domain.JobCursor{CreatedAt: createdAt, ID: id}, nil
}

func encodeImageJobHistoryCursor(cursor domain.JobCursor) (string, bool) {
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return "", false
	}
	payload, err := json.Marshal(struct {
		CreatedAt string `json:"created_at"`
		ID        string `json:"id"`
	}{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID.String(),
	})
	if err != nil {
		return "", false
	}
	return base64.RawURLEncoding.EncodeToString(payload), true
}

func conversationMessageAfterSeq(values []string, provided bool) (int64, error) {
	if !provided {
		return 0, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, errors.New("invalid message cursor")
	}
	afterSeq, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || afterSeq < 0 {
		return 0, errors.New("invalid message cursor")
	}
	return afterSeq, nil
}

func conversationMessageBeforeSeq(values []string, provided bool) (int64, error) {
	if !provided {
		return 0, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, errors.New("invalid message cursor")
	}
	beforeSeq, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || beforeSeq < 1 {
		return 0, errors.New("invalid message cursor")
	}
	return beforeSeq, nil
}

func conversationMessageLimit(values []string, provided bool) (int, error) {
	const (
		defaultMessageLimit = 100
		maxMessageLimit     = 100
	)
	if !provided {
		return defaultMessageLimit, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, errors.New("invalid message limit")
	}
	limit, err := strconv.Atoi(values[0])
	if err != nil || limit < 1 || limit > maxMessageLimit {
		return 0, errors.New("invalid message limit")
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

type safeConversationMessageList struct {
	Items         []safeConversationMessage `json:"items"`
	HasMoreBefore *bool                     `json:"has_more_before,omitempty"`
}

type safeConversationMessage struct {
	ID        uuid.UUID                      `json:"id"`
	Seq       int64                          `json:"seq"`
	Role      domain.ConversationMessageRole `json:"role"`
	Text      string                         `json:"text"`
	CreatedAt time.Time                      `json:"created_at"`
}

type safeWebChatJob struct {
	JobID  uuid.UUID        `json:"job_id"`
	Status domain.JobStatus `json:"status"`
}

type webChatJobParams struct {
	Prompt             string                    `json:"prompt"`
	ModelID            string                    `json:"model_id"`
	ModelName          string                    `json:"model_name"`
	Provider           domain.ProviderName       `json:"provider"`
	ModelCode          string                    `json:"model_code"`
	ConversationID     string                    `json:"conversation_id"`
	ConversationSource domain.ConversationSource `json:"conversation_source"`
}

type safeImageModelList struct {
	Items []safeImageModel `json:"items"`
}

type safeImageModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	QualityOptions         []string `json:"quality_options"`
	DefaultQuality         string   `json:"default_quality"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images"`
}

// webImageJobParams is stored with the job for the worker. It is deliberately
// separate from all safe browser response DTOs because it contains private
// provider routing fields.
type webImageJobParams struct {
	Prompt       string              `json:"prompt"`
	ModelID      string              `json:"model_id"`
	ModelName    string              `json:"model_name"`
	Provider     domain.ProviderName `json:"provider"`
	ModelCode    string              `json:"model_code"`
	Size         string              `json:"size"`
	Resolution   string              `json:"resolution"`
	ImageQuality string              `json:"image_quality"`
}

type safeImageJobPreparation struct {
	Job       safeImageJob `json:"job"`
	Balance   int64        `json:"balance"`
	CanAfford bool         `json:"can_afford"`
}

type safeImageJobActivation struct {
	Job safeImageJob `json:"job"`
}

type safeImageJobList struct {
	Items      []safeImageJob `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor *string        `json:"next_cursor"`
}

type safeImageJobResult struct {
	JobID     uuid.UUID                   `json:"job_id"`
	Status    domain.JobStatus            `json:"status"`
	Artifacts []safeImageArtifactMetadata `json:"artifacts"`
}

type safeImageArtifactMetadata struct {
	ID        uuid.UUID `json:"id"`
	MIMEType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
}

type safeImageJob struct {
	ID           uuid.UUID        `json:"id"`
	Status       domain.JobStatus `json:"status"`
	Prompt       string           `json:"prompt"`
	ModelID      string           `json:"model_id"`
	ModelName    string           `json:"model_name"`
	ImageQuality string           `json:"image_quality"`
	CostEstimate int64            `json:"cost_estimate"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

func newSafeConversation(conversation domain.Conversation) safeConversation {
	return safeConversation{
		ID:        conversation.ID,
		Title:     conversation.Title,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
	}
}

func newSafeConversationMessage(message domain.ConversationMessage) (safeConversationMessage, bool) {
	if message.ID == uuid.Nil || message.Seq < 1 || (message.Role != domain.ConversationRoleUser && message.Role != domain.ConversationRoleAssistant) {
		return safeConversationMessage{}, false
	}
	return safeConversationMessage{
		ID:        message.ID,
		Seq:       message.Seq,
		Role:      message.Role,
		Text:      message.Text,
		CreatedAt: message.CreatedAt,
	}, true
}

func newSafeImageModel(model imagegeneration.PublicModel) (safeImageModel, bool) {
	model.ID = strings.TrimSpace(model.ID)
	model.Name = strings.TrimSpace(model.Name)
	if model.ID == "" || model.Name == "" || !model.Enabled || !model.Ready {
		return safeImageModel{}, false
	}
	return safeImageModel{
		ID:                     model.ID,
		Name:                   model.Name,
		QualityOptions:         append([]string(nil), model.QualityOptions...),
		DefaultQuality:         model.DefaultQuality,
		SupportsReferenceImage: model.SupportsReferenceImage,
		MaxReferenceImages:     model.MaxReferenceImages,
	}, true
}

// preparedWebImageJobReplay verifies that an account-scoped idempotency row is
// the exact non-executable web image preparation represented by the same
// public browser intent. It deliberately trusts the persisted immutable price
// only after validating its full snapshot against that intent; current catalog
// pricing and private provider routing never participate in this comparison.
func preparedWebImageJobReplay(job *domain.Job, accountID uuid.UUID, idempotencyKey, prompt string, intent imagegeneration.PublicSelection) (safeImageJob, bool) {
	if job == nil || job.ID == uuid.Nil || job.AccountID != accountID || job.UserID != uuid.Nil || job.CommandID != uuid.Nil || job.VKPeerID != 0 ||
		job.Source != "web" || job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" ||
		job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil || job.Status != domain.JobStatusPrepared ||
		(job.ExpiresAt != nil && !job.ExpiresAt.After(time.Now())) ||
		job.IdempotencyKey != idempotencyKey || job.CorrelationID != "web-image:"+idempotencyKey || len(job.InputArtifactIDs) != 0 || len(job.OutputArtifactIDs) != 0 ||
		job.CostReserved != 0 || job.CostCaptured != 0 || job.ErrorCode != "" || job.ErrorMessage != "" || job.CostEstimate <= 0 || job.OperationType != domain.OperationImageGenerate || job.Modality != domain.ModalityImage {
		return safeImageJob{}, false
	}
	if err := job.ValidateResultContract(); err != nil {
		return safeImageJob{}, false
	}

	var snapshot pricingcatalog.PricingSnapshot
	if err := json.Unmarshal(job.PricingSnapshot, &snapshot); err != nil || !snapshot.Valid() || snapshot.InternalCredits != job.CostEstimate {
		return safeImageJob{}, false
	}
	key := snapshot.Key.Normalize()
	if key.Operation != domain.OperationImageGenerate || key.Modality != domain.ModalityImage || key.ImageModelID != intent.ModelID || key.Quality != intent.ImageQuality {
		return safeImageJob{}, false
	}

	var params webImageJobParams
	if err := json.Unmarshal(job.Params, &params); err != nil || params.Prompt != prompt || params.ModelID != intent.ModelID || params.ImageQuality != intent.ImageQuality ||
		strings.TrimSpace(params.ModelName) == "" || params.Provider == "" || strings.TrimSpace(params.ModelCode) == "" {
		return safeImageJob{}, false
	}
	return newSafeImageJob(job)
}

func newSafeImageJob(job *domain.Job) (safeImageJob, bool) {
	if job == nil || job.ID == uuid.Nil || job.Source != "web" || job.OperationType != domain.OperationImageGenerate || job.Modality != domain.ModalityImage || job.CostEstimate <= 0 {
		return safeImageJob{}, false
	}
	var params webImageJobParams
	if err := json.Unmarshal(job.Params, &params); err != nil || strings.TrimSpace(params.Prompt) == "" || strings.TrimSpace(params.ModelID) == "" || strings.TrimSpace(params.ModelName) == "" {
		return safeImageJob{}, false
	}
	return safeImageJob{
		ID:           job.ID,
		Status:       job.Status,
		Prompt:       params.Prompt,
		ModelID:      params.ModelID,
		ModelName:    params.ModelName,
		ImageQuality: params.ImageQuality,
		CostEstimate: job.CostEstimate,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}, true
}

func newSafeImageJobResult(result resultservice.Result, expectedJobID uuid.UUID) (safeImageJobResult, bool) {
	if expectedJobID == uuid.Nil || result.ID != expectedJobID || result.Operation != domain.OperationImageGenerate || result.Modality != domain.ModalityImage || result.Status != domain.JobStatusSucceeded || len(result.Artifacts) == 0 {
		return safeImageJobResult{}, false
	}
	artifacts := make([]safeImageArtifactMetadata, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		if artifact.ID == uuid.Nil || artifact.MediaType != domain.MediaTypeImage || strings.TrimSpace(artifact.MIMEType) == "" || artifact.SizeBytes < 1 {
			return safeImageJobResult{}, false
		}
		artifacts = append(artifacts, safeImageArtifactMetadata{
			ID:        artifact.ID,
			MIMEType:  artifact.MIMEType,
			SizeBytes: artifact.SizeBytes,
			Width:     artifact.Width,
			Height:    artifact.Height,
		})
	}
	return safeImageJobResult{JobID: expectedJobID, Status: result.Status, Artifacts: artifacts}, true
}

func safeResultContainsArtifact(result safeImageJobResult, artifactID uuid.UUID) bool {
	if artifactID == uuid.Nil {
		return false
	}
	for _, artifact := range result.Artifacts {
		if artifact.ID == artifactID {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
