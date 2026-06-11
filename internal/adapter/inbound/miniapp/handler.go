package miniapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

type contextKey int

const ctxVKUserIDKey contextKey = iota

// JobRateLimiter is the minimal limiter contract used by Mini App write-like
// endpoints after authentication. Endpoint-specific keys keep job submits and
// estimate requests from sharing a bucket.
type JobRateLimiter interface {
	Allow(key string) bool
}

// ReferralService is the shared backend referral service used by VK bot and
// the Mini App BFF. It must keep all rewards ledger-backed and idempotent.
type ReferralService interface {
	Stats(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, int, error)
	StatsDetailed(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, domain.ReferralStats, error)
	Apply(ctx context.Context, input referralservice.ApplyInput) (referralservice.ApplyResult, error)
	Activate(ctx context.Context, input referralservice.ActivateInput) (referralservice.ActivateResult, error)
}

// Config holds per-deployment miniapp settings.
type Config struct {
	// AppSecret is the VK App's protected key for verifying launch-params
	// signatures. When empty the check is skipped (dev/mock mode).
	AppSecret string
	// LaunchParamsMaxAge is the maximum allowed age of the vk_ts timestamp.
	// Zero disables the age check.
	LaunchParamsMaxAge time.Duration
	// JobRateLimiter bounds POST /miniapp/jobs and POST /miniapp/estimate after
	// launch params have been verified, keyed by the verified vk_user_id.
	JobRateLimiter JobRateLimiter
	// ImageReferenceEnabled allows validated image reference artifacts to flow
	// into image jobs. When false, references fail closed before job creation.
	ImageReferenceEnabled bool
	// ReferralLinkBase builds the user's public VK referral URL. If it contains
	// "{code}", the placeholder is replaced; otherwise ref=<code> is appended.
	ReferralLinkBase string
	// Referral signup reward amounts are exposed for UI copy only; the service
	// posts actual rewards through billing ledger entries.
	ReferralReferrerSignupRewardCredits int64
	ReferralReferredSignupRewardCredits int64
	// FrontendTelemetryEnabled accepts safe client-side telemetry events.
	FrontendTelemetryEnabled bool
	// FrontendTelemetryUserHashSecret hashes verified VK user ids for optional
	// debug logs without storing raw user identifiers.
	FrontendTelemetryUserHashSecret string
}

// ObjectReader loads and stores artifact bytes (S3/MinIO).
type ObjectReader interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// Deps are the collaborators needed by the miniapp handler.
type Deps struct {
	Users         domain.UserRepository
	Jobs          domain.JobRepository
	Conversations domain.ConversationRepository
	Artifacts     domain.ArtifactRepository
	Moderation    domain.ModerationResultRepository
	Objects       ObjectReader
	Billing       *billingservice.Service
	BillingRepo   domain.BillingRepository
	Payment       *paymentservice.Service
	Referrals     ReferralService
	Orchestrator  *joborchestrator.Orchestrator
	Logger        *slog.Logger
}

// Handler serves the /miniapp/* routes.
type Handler struct {
	cfg    Config
	deps   Deps
	logger *slog.Logger
}

// NewHandler builds a miniapp Handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{cfg: cfg, deps: deps, logger: logger}
}

// Routes returns an http.Handler with the miniapp routes registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /miniapp/estimate", h.auth(h.rateLimitMiniApp("miniapp_estimate", h.estimateJob)))
	mux.HandleFunc("POST /miniapp/chat/messages", h.auth(h.rateLimitMiniApp("miniapp_chat", h.createChatMessage)))
	mux.HandleFunc("GET /miniapp/chat/conversations", h.auth(h.listChatConversations))
	mux.HandleFunc("GET /miniapp/chat/conversations/{id}/messages", h.auth(h.listChatConversationMessages))
	mux.HandleFunc("POST /miniapp/jobs", h.auth(h.rateLimitMiniApp("miniapp_job", h.createJob)))
	mux.HandleFunc("GET /miniapp/jobs", h.auth(h.listJobs))
	mux.HandleFunc("GET /miniapp/jobs/{id}", h.auth(h.getJob))
	mux.HandleFunc("GET /miniapp/balance", h.auth(h.getBalance))
	mux.HandleFunc("GET /miniapp/referral", h.auth(h.getReferral))
	mux.HandleFunc("POST /miniapp/referral/accept", h.auth(h.rateLimitMiniApp("miniapp_referral", h.acceptReferral)))
	mux.HandleFunc("GET /miniapp/payment-products", h.auth(h.listPaymentProducts))
	mux.HandleFunc("POST /miniapp/payments/intents", h.auth(h.rateLimitMiniApp("miniapp_payment", h.createPaymentIntent)))
	mux.HandleFunc("GET /miniapp/payments", h.auth(h.listPayments))
	mux.HandleFunc("GET /miniapp/payments/{id}", h.auth(h.getPaymentIntent))
	mux.HandleFunc("POST /miniapp/artifacts", h.auth(h.rateLimitMiniApp("miniapp_artifact", h.createArtifact)))
	mux.HandleFunc("GET /miniapp/artifacts/{id}", h.auth(h.getArtifact))
	mux.HandleFunc("POST /miniapp/client-events", h.auth(h.rateLimitMiniApp("miniapp_client_events", h.clientEvent)))
	return mux
}

// auth is the middleware that verifies the VK launch-params signature and
// populates the request context with the verified vk_user_id. It returns 401
// for any signature failure without revealing details (audit S1).
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawParams := r.Header.Get("X-Launch-Params")
		if rawParams == "" {
			// Also accept query param for easier browser testing.
			rawParams = r.URL.Query().Get("launch_params")
		}

		if rawParams == "" && h.cfg.AppSecret == "" {
			// Dev/mock mode without params: require explicit vk_user_id header.
			raw := r.Header.Get("X-VK-User-ID")
			if raw == "" {
				metrics.AuthFailures.WithLabelValues("miniapp", "missing_dev_user").Inc()
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			uid, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || uid <= 0 {
				metrics.AuthFailures.WithLabelValues("miniapp", "invalid_dev_user").Inc()
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), ctxVKUserIDKey, uid)
			next(w, r.WithContext(ctx))
			return
		}

		params, err := VerifyLaunchParams(rawParams, h.cfg.AppSecret, h.cfg.LaunchParamsMaxAge)
		if err != nil {
			metrics.AuthFailures.WithLabelValues("miniapp", "launch_params_rejected").Inc()
			metrics.SignatureFailures.WithLabelValues("miniapp", "launch_params_rejected").Inc()
			// Do not expose the reason to the client (audit S1).
			if h.cfg.AppSecret != "" {
				h.logger.Warn("miniapp: launch params rejected",
					slog.String("reason", err.Error()))
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		vkUserID, err := VKUserIDFromParams(params)
		if err != nil {
			metrics.AuthFailures.WithLabelValues("miniapp", "missing_vk_user_id").Inc()
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), ctxVKUserIDKey, vkUserID)
		next(w, r.WithContext(ctx))
	}
}

// vkUserIDFromCtx retrieves the authenticated vk_user_id from the context.
func vkUserIDFromCtx(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxVKUserIDKey).(int64)
	return v, ok && v > 0
}

func (h *Handler) rateLimitMiniApp(keyPrefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.JobRateLimiter == nil {
			next(w, r)
			return
		}
		vkUserID, ok := vkUserIDFromCtx(r.Context())
		if !ok {
			metrics.AuthFailures.WithLabelValues("miniapp", "missing_context_user").Inc()
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !h.cfg.JobRateLimiter.Allow(keyPrefix + ":" + strconv.FormatInt(vkUserID, 10)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next(w, r)
	}
}

// ensureUser returns the existing user or creates a new one with a billing
// account, identical to the VK webhook handler's ensureUser.
func (h *Handler) ensureUser(ctx context.Context, vkUserID int64) (*domain.User, error) {
	user, err := h.deps.Users.GetByVKUserID(ctx, vkUserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	user = &domain.User{
		VKUserID: vkUserID,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := h.deps.Users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return h.deps.Users.GetByVKUserID(ctx, vkUserID)
		}
		return nil, err
	}
	if _, err := h.deps.Billing.EnsureAccount(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("ensure account: %w", err)
	}
	return user, nil
}

func (h *Handler) clientEvent(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		metrics.AuthFailures.WithLabelValues("miniapp", "missing_context_user").Inc()
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !h.cfg.FrontendTelemetryEnabled {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	var req ClientEventRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid client event")
		return
	}
	eventType, ok := safeClientEventType(req.EventType)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid client event")
		return
	}
	surface := safeClientLabel(req.Surface, "vk_mini_app")
	screen := safeClientLabel(req.Screen, "unknown")
	route := safeClientRoute(req.Route)
	if route == "rejected" {
		metrics.SuspiciousEvents.WithLabelValues("miniapp", "unsafe_client_event").Inc()
		writeError(w, http.StatusBadRequest, "invalid client event")
		return
	}
	status := safeClientStatus(req.Status)
	errorClass := safeClientLabel(req.ErrorClass, "unknown")
	step := safeClientLabel(req.Step, "unknown")
	reason := safeClientLabel(req.Reason, "unknown")

	metrics.FrontendEvents.WithLabelValues(surface, eventType).Inc()
	metrics.ObserveProductEvent(surface, "frontend", eventType, "unknown", "unknown", "observed")
	switch eventType {
	case "js_error":
		metrics.FrontendJSErrors.WithLabelValues(surface, screen, errorClass).Inc()
	case "api_failure":
		metrics.FrontendAPIFailures.WithLabelValues(surface, route, status).Inc()
	case "launch_failure":
		metrics.FrontendLaunchFailures.WithLabelValues(surface, reason).Inc()
	case "payment_flow_error":
		metrics.FrontendPaymentFlowErrors.WithLabelValues(step, errorClass).Inc()
	case "ui_event":
		metrics.ObserveProductFrontendUIDuration(surface, step, reason, req.DurationMS)
	}
	if eventType == "api_latency" || (eventType == "api_failure" && req.DurationMS > 0) {
		metrics.ObserveProductFrontendAPIDuration(surface, route, status, req.DurationMS)
	}
	if hash := h.clientUserHash(vkUserID); hash != "" {
		h.logger.Debug("miniapp client event",
			slog.String("event_type", eventType),
			slog.String("screen", screen),
			slog.String("route", route),
			slog.String("status", status),
			slog.String("user_hash", hash))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clientUserHash(vkUserID int64) string {
	secret := strings.TrimSpace(h.cfg.FrontendTelemetryUserHashSecret)
	if secret == "" || vkUserID <= 0 {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(vkUserID, 10)))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func safeClientEventType(value string) (string, bool) {
	switch safeClientLabel(value, "") {
	case "js_error":
		return "js_error", true
	case "api_failure":
		return "api_failure", true
	case "api_latency":
		return "api_latency", true
	case "launch_failure":
		return "launch_failure", true
	case "payment_flow_error":
		return "payment_flow_error", true
	case "ui_event":
		return "ui_event", true
	default:
		return "", false
	}
}

func safeClientRoute(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "launch") || strings.Contains(lower, "vk_user") || strings.Contains(lower, "sign") {
		return "rejected"
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if _, err := uuid.Parse(part); err == nil {
			parts[i] = ":id"
			continue
		}
		if allDigits(part) {
			parts[i] = ":id"
		}
	}
	route := strings.Join(parts, "/")
	switch route {
	case "/miniapp/estimate",
		"/miniapp/chat/messages",
		"/miniapp/chat/conversations",
		"/miniapp/jobs",
		"/miniapp/balance",
		"/miniapp/referral",
		"/miniapp/referral/accept",
		"/miniapp/payment-products",
		"/miniapp/payments/intents",
		"/miniapp/payments",
		"/miniapp/artifacts",
		"/miniapp/client-events":
		return route
	case "/miniapp/jobs/:id",
		"/miniapp/payments/:id",
		"/miniapp/artifacts/:id",
		"/miniapp/chat/conversations/:id/messages":
		return route
	default:
		return "other"
	}
}

func safeClientStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	if value == "network" || value == "timeout" {
		return value
	}
	if len(value) == 3 && allDigits(value) {
		return value
	}
	return "unknown"
}

func safeClientLabel(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		if fallback == "" {
			return ""
		}
		return fallback
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '/' || r == ':' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return fallback
	}
	return out
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// operationMeta maps an operation string to (OperationType, Modality). Returns
// false if the operation is not supported via the mini app.
func operationMeta(op string) (domain.OperationType, domain.Modality, bool) {
	switch op {
	case "text_generate":
		return domain.OperationTextGenerate, domain.ModalityText, true
	case "image_generate":
		return domain.OperationImageGenerate, domain.ModalityImage, true
	case "video_generate":
		return domain.OperationVideoGenerate, domain.ModalityVideo, true
	default:
		return "", "", false
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) readJobRequest(w http.ResponseWriter, r *http.Request) (CreateJobRequest, domain.OperationType, domain.Modality, miniAppModelSpec, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}
	var req CreateJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}

	opType, modality, ok := operationMeta(req.Operation)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported operation; allowed: text_generate, image_generate, video_generate")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}
	model, ok := resolveMiniAppModel(opType, req.ModelID)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported model")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}
	if req.DurationSec != 0 && opType != domain.OperationVideoGenerate {
		writeError(w, http.StatusBadRequest, "duration_sec is only supported for video_generate")
		return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
	}
	if opType == domain.OperationVideoGenerate {
		duration, ok := normalizeVideoDurationSec(req.DurationSec)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid video duration; allowed: 3, 5, 10")
			return CreateJobRequest{}, "", "", miniAppModelSpec{}, false
		}
		req.DurationSec = duration
	}
	return req, opType, modality, model, true
}

func normalizeVideoDurationSec(sec int) (int, bool) {
	switch sec {
	case 0:
		return 5, true
	case 3, 5, 10:
		return sec, true
	default:
		return 0, false
	}
}

func (h *Handler) estimateJob(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req, opType, modality, model, ok := h.readJobRequest(w, r)
	if !ok {
		return
	}
	if len(req.ReferenceArtifactIDs) > 0 {
		user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not found")
			} else {
				writeError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		if !h.validateReferenceArtifacts(w, r, user.ID, opType, req.ReferenceArtifactIDs) {
			return
		}
		if !h.cfg.ImageReferenceEnabled {
			writeError(w, http.StatusBadRequest, "reference_artifacts_unsupported")
			return
		}
	}

	cost, err := h.deps.Billing.Estimate(opType)
	if err != nil {
		if errors.Is(err, billingservice.ErrUnknownOperation) {
			metrics.ObserveProductEvent("miniapp", "job", "estimate", string(opType), string(modality), "unsupported_operation")
			writeError(w, http.StatusBadRequest, "unsupported operation; allowed: text_generate, image_generate, video_generate")
		} else {
			metrics.ObserveProductEvent("miniapp", "job", "estimate", string(opType), string(modality), "error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	balance, err := h.balanceForEstimate(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: estimate balance failed", slog.String("error", err.Error()))
		metrics.ObserveProductEvent("miniapp", "job", "estimate", string(opType), string(modality), "error")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	metrics.ObserveProductEvent("miniapp", "job", "estimate", string(opType), string(modality), "success")
	writeJSON(w, http.StatusOK, EstimateDTO{
		Operation:      string(opType),
		ModelID:        miniAppResponseModelID(model),
		ModelName:      model.ModelName,
		CostEstimate:   cost,
		BalanceCredits: balance,
		EnoughCredits:  balance >= cost,
	})
}

func (h *Handler) balanceForEstimate(ctx context.Context, vkUserID int64) (int64, error) {
	user, err := h.deps.Users.GetByVKUserID(ctx, vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return h.deps.Billing.StartingBalance(), nil
		}
		return 0, err
	}
	return h.deps.Billing.BalanceForEstimate(ctx, user.ID)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req, opType, modality, model, ok := h.readJobRequest(w, r)
	if !ok {
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(req.ReferenceArtifactIDs) > 0 {
		if opType == domain.OperationVideoGenerate {
			writeError(w, http.StatusBadRequest, "reference_artifacts_unsupported")
			return
		}
		if !h.validateReferenceArtifacts(w, r, user.ID, opType, req.ReferenceArtifactIDs) {
			return
		}
		if !h.cfg.ImageReferenceEnabled {
			writeError(w, http.StatusBadRequest, "reference_artifacts_unsupported")
			return
		}
	}

	// Accept an optional client-supplied idempotency key. The key is scoped to
	// the user so one user cannot replay another user's key.
	clientKey := r.Header.Get("X-Idempotency-Key")
	if clientKey == "" {
		clientKey = uuid.New().String()
	}
	idemKey := fmt.Sprintf("miniapp_job:%d:%s", vkUserID, clientKey)
	correlationID := fmt.Sprintf("miniapp:%d:%s", vkUserID, clientKey)

	jobParams := miniAppJobParams{
		Prompt:               req.Prompt,
		ModelID:              model.ModelID,
		ModelName:            model.ModelName,
		ModelCode:            model.ModelCode,
		ReferenceArtifactIDs: req.ReferenceArtifactIDs,
	}
	if opType == domain.OperationVideoGenerate {
		jobParams.DurationSec = req.DurationSec
	}
	params, _ := json.Marshal(jobParams)
	metrics.ObserveProductPromptLength("miniapp", string(opType), string(modality), req.Prompt)

	job, err := h.deps.Orchestrator.CreateJob(r.Context(), joborchestrator.CreateJobInput{
		UserID:           user.ID,
		Source:           "miniapp",
		VKPeerID:         vkUserID, // peer_id = user_id for direct messages
		CommandID:        uuid.Nil, // no VK command for mini app path
		Operation:        opType,
		Modality:         modality,
		IdempotencyKey:   idemKey,
		CorrelationID:    correlationID,
		InputArtifactIDs: req.ReferenceArtifactIDs,
		Params:           params,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, newJobDTO(job))
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"error":         "insufficient_credits",
			"job_id":        job.ID,
			"status":        string(job.Status),
			"cost_estimate": job.CostEstimate,
		})
	case errors.Is(err, domain.ErrCostCapExceeded):
		writeError(w, http.StatusBadRequest, "job cost exceeds platform limit")
	default:
		h.logger.Error("miniapp: create job failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) readChatMessageRequest(w http.ResponseWriter, r *http.Request) (ChatMessageRequest, string, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return ChatMessageRequest{}, "", false
	}
	var req ChatMessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return ChatMessageRequest{}, "", false
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return ChatMessageRequest{}, "", false
	}
	conversationID, ok := normalizeConversationID(req.ConversationID)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return ChatMessageRequest{}, "", false
	}
	req.ConversationID = conversationID
	return req, conversationID, true
}

func (h *Handler) createChatMessage(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req, conversationID, ok := h.readChatMessageRequest(w, r)
	if !ok {
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	clientKey := r.Header.Get("X-Idempotency-Key")
	if clientKey == "" {
		clientKey = uuid.New().String()
	}
	idemKey := fmt.Sprintf("miniapp_chat:%d:%s", vkUserID, clientKey)
	correlationID := fmt.Sprintf("miniapp-chat:%d:%s", vkUserID, clientKey)

	params, _ := json.Marshal(miniAppJobParams{
		Prompt:             req.Prompt,
		ModelID:            miniAppChatModelID,
		ModelName:          miniAppChatPublicModelName,
		ConversationSource: domain.ConversationSourceMiniApp,
		ExternalThreadID:   conversationID,
	})
	metrics.ObserveProductPromptLength("miniapp", string(domain.OperationTextGenerate), string(domain.ModalityText), req.Prompt)

	job, err := h.deps.Orchestrator.CreateJob(r.Context(), joborchestrator.CreateJobInput{
		UserID:         user.ID,
		Source:         "miniapp",
		VKPeerID:       vkUserID,
		CommandID:      uuid.Nil,
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		IdempotencyKey: idemKey,
		CorrelationID:  correlationID,
		Params:         params,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, newChatJobDTO(job))
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"error":         "insufficient_credits",
			"job_id":        job.ID,
			"status":        string(job.Status),
			"cost_estimate": job.CostEstimate,
			"model_name":    miniAppChatPublicModelName,
		})
	case errors.Is(err, domain.ErrCostCapExceeded):
		writeError(w, http.StatusBadRequest, "job cost exceeds platform limit")
	default:
		h.logger.Error("miniapp: create chat job failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) listChatConversations(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, listResponse[ChatConversationDTO]{
				Items:      []ChatConversationDTO{},
				Pagination: pagination{Limit: defaultLimit, Offset: 0, Count: 0},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	limit, offset := parsePagination(r)
	conversations, err := h.deps.Conversations.ListByUserSource(r.Context(), user.ID, domain.ConversationSourceMiniApp, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	hasMore := len(conversations) > limit
	if hasMore {
		conversations = conversations[:limit]
	}

	items := make([]ChatConversationDTO, 0, len(conversations))
	for _, conversation := range conversations {
		items = append(items, h.newChatConversationDTO(r.Context(), conversation))
	}
	writeJSON(w, http.StatusOK, listResponse[ChatConversationDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) listChatConversationMessages(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	threadID, ok := normalizeConversationID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	conversation, err := h.deps.Conversations.GetActiveByReference(r.Context(), domain.ConversationRef{
		UserID:           user.ID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: threadID,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	limit, _ := parsePagination(r)
	afterSeq := parseAfterSeq(r)
	messages, err := h.deps.Conversations.ListMessagesAfter(r.Context(), conversation.ID, afterSeq, limit+1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	items := make([]ChatConversationMessageDTO, 0, len(messages))
	for _, message := range messages {
		items = append(items, newChatConversationMessageDTO(message))
	}
	writeJSON(w, http.StatusOK, listResponse[ChatConversationMessageDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: 0, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, listResponse[JobDTO]{
				Items:      []JobDTO{},
				Pagination: pagination{Limit: 20, Offset: 0, Count: 0},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	limit, offset := parsePagination(r)

	jobs, err := h.deps.Jobs.ListByUser(r.Context(), user.ID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list jobs failed")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}

	items := make([]JobDTO, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, newJobDTO(j))
	}
	writeJSON(w, http.StatusOK, listResponse[JobDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	jobID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	job, err := h.deps.Jobs.GetByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	// Ownership check: a user may only retrieve their own jobs (invariant).
	if job.UserID != user.ID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	writeJSON(w, http.StatusOK, newJobDTO(job))
}

func (h *Handler) getBalance(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	acc, err := h.deps.BillingRepo.GetAccountByUser(r.Context(), user.ID, domain.CurrencyCredits)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, BalanceDTO{BalanceCredits: 0})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, BalanceDTO{BalanceCredits: acc.BalanceCached})
}

func (h *Handler) getReferral(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Referrals == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	code, stats, err := h.deps.Referrals.StatsDetailed(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, ReferralDTO{
		Code:                        code.Code,
		InviteURL:                   buildReferralLink(h.cfg.ReferralLinkBase, code.Code),
		InvitedCount:                stats.Total(),
		RegisteredCount:             stats.RegisteredCount,
		ActivatedCount:              stats.ActivatedCount,
		RewardedCount:               stats.RewardedCount,
		ReferrerSignupRewardCredits: h.cfg.ReferralReferrerSignupRewardCredits,
		ReferredSignupRewardCredits: h.cfg.ReferralReferredSignupRewardCredits,
	})
}

func (h *Handler) acceptReferral(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Referrals == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	var req ApplyReferralRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	code := referralservice.NormalizeCode(req.Code)
	if code == "" {
		writeJSON(w, http.StatusOK, ApplyReferralDTO{InvalidCode: true})
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	result, err := h.deps.Referrals.Apply(r.Context(), referralservice.ApplyInput{
		Code:           code,
		ReferredUserID: user.ID,
		Source:         domain.ReferralSourceVKMiniApp,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !result.InvalidCode && !result.SelfReferral {
		if _, err := h.deps.Referrals.Activate(r.Context(), referralservice.ActivateInput{
			ReferredUserID: user.ID,
			Source:         domain.ReferralSourceVKMiniApp,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	writeJSON(w, http.StatusOK, ApplyReferralDTO{
		Applied:        result.Applied,
		AlreadyApplied: result.AlreadyApplied,
		InvalidCode:    result.InvalidCode,
		SelfReferral:   result.SelfReferral,
	})
}

func buildReferralLink(base string, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if strings.Contains(base, "{code}") {
		return strings.ReplaceAll(base, "{code}", url.QueryEscape(code))
	}
	return appendURLParam(base, "ref", code)
}

func appendURLParam(raw, key, value string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func (h *Handler) createPaymentIntent(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Payment == nil {
		metrics.ObserveProductEvent("miniapp", "payment", "intent_create", "top_up", "credits", "service_unavailable")
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	clientKey := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	if clientKey == "" {
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	var req CreatePaymentIntentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	result, err := h.deps.Payment.CreateIntent(r.Context(), paymentservice.CreateIntentInput{
		UserID:         user.ID,
		ProductCode:    req.ProductCode,
		ReceiptEmail:   req.ReceiptEmail,
		ReceiptPhone:   req.ReceiptPhone,
		IdempotencyKey: "miniapp_payment:" + strconv.FormatInt(vkUserID, 10) + ":" + clientKey,
		ReturnURL:      req.ReturnURL,
		Source:         "vk_miniapp",
		ForceNew:       req.ForceNew,
	})
	if err != nil {
		h.writePaymentError(w, err)
		return
	}
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	dto := newPaymentIntentDTO(result.Intent)
	if result.ReusedActive {
		dto.ReusedActivePayment = true
		dto.Notice = "У вас уже есть незавершенный платеж. После оплаты баланс обновится автоматически."
	}
	writeJSON(w, status, dto)
}

func (h *Handler) listPaymentProducts(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	products, err := h.deps.Payment.ListActiveProducts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	items := make([]PaymentProductDTO, 0, len(products))
	for _, product := range products {
		items = append(items, newPaymentProductDTO(product))
	}
	writeJSON(w, http.StatusOK, listResponse[PaymentProductDTO]{
		Items:      items,
		Pagination: pagination{Limit: len(items), Offset: 0, Count: len(items), HasMore: false},
	})
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, listResponse[PaymentIntentDTO]{
				Items:      []PaymentIntentDTO{},
				Pagination: pagination{Limit: defaultLimit, Offset: 0, Count: 0},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	limit, offset := parsePagination(r)
	intents, err := h.deps.Payment.ListIntentsByUser(r.Context(), user.ID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	hasMore := len(intents) > limit
	if hasMore {
		intents = intents[:limit]
	}
	items := make([]PaymentIntentDTO, 0, len(intents))
	for _, intent := range intents {
		items = append(items, newPaymentIntentDTO(intent))
	}
	writeJSON(w, http.StatusOK, listResponse[PaymentIntentDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) getPaymentIntent(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment id")
		return
	}
	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	intent, err := h.deps.Payment.GetIntent(r.Context(), user.ID, intentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, newPaymentIntentDTO(intent))
}

func (h *Handler) writePaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paymentservice.ErrInvalidInput),
		errors.Is(err, paymentservice.ErrReceiptContactRequired):
		writeError(w, http.StatusBadRequest, "invalid payment request")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, paymentservice.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		h.logger.Error("miniapp: payment provider failed", slog.String("error", err.Error()))
		writeError(w, http.StatusBadGateway, "payment provider error")
	}
}

func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request) {
	resultLabel := "success"
	defer func() {
		metrics.ObserveProductEvent("miniapp", "artifact", "access", "artifact_access", "artifact", resultLabel)
	}()
	if h.deps.Artifacts == nil || h.deps.Objects == nil {
		resultLabel = "service_unavailable"
		writeError(w, http.StatusServiceUnavailable, "artifact storage unavailable")
		return
	}

	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		resultLabel = "unauthorized"
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	artID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		resultLabel = "invalid_id"
		writeError(w, http.StatusBadRequest, "invalid artifact id")
		return
	}

	user, err := h.deps.Users.GetByVKUserID(r.Context(), vkUserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			resultLabel = "not_found"
			writeError(w, http.StatusNotFound, "not found")
		} else {
			resultLabel = "error"
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	art, err := h.deps.Artifacts.GetByID(r.Context(), artID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			resultLabel = "not_found"
			writeError(w, http.StatusNotFound, "not found")
		} else {
			resultLabel = "error"
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	if art.OwnerUserID != user.ID {
		resultLabel = "not_found"
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !h.artifactVisible(r.Context(), art, user.ID) {
		resultLabel = "not_visible"
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	data, err := h.deps.Objects.GetObject(r.Context(), art.StorageBucket, art.StorageKey)
	if err != nil {
		h.logger.Error("miniapp: get artifact object failed", slog.String("error", err.Error()))
		resultLabel = "error"
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	ct := art.MimeType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) artifactVisible(ctx context.Context, art *domain.Artifact, userID uuid.UUID) bool {
	if art == nil || art.JobID == nil || art.Kind != domain.ArtifactKindOutput || art.Status != domain.ArtifactStatusReady {
		return false
	}
	job, err := h.deps.Jobs.GetByID(ctx, *art.JobID)
	if err != nil || job.UserID != userID || job.Status != domain.JobStatusSucceeded {
		return false
	}
	if !uuidInSlice(job.OutputArtifactIDs, art.ID) || h.deps.Moderation == nil {
		return false
	}
	results, err := h.deps.Moderation.ListByJob(ctx, job.ID)
	if err != nil {
		h.logger.Error("miniapp: list moderation results failed", slog.String("error", err.Error()))
		return false
	}
	for _, result := range results {
		if result.Stage != domain.ModerationStageOutput || result.ArtifactID == nil || *result.ArtifactID != art.ID {
			continue
		}
		if result.Decision.Allowed() {
			return true
		}
	}
	return false
}

func uuidInSlice(ids []uuid.UUID, target uuid.UUID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (h *Handler) newChatConversationDTO(ctx context.Context, conversation *domain.Conversation) ChatConversationDTO {
	dto := ChatConversationDTO{
		ID:        conversation.ExternalThreadID,
		Title:     chatConversationTitle(conversation),
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
	}
	if dto.ID == "" {
		dto.ID = defaultConversationID
	}
	if h.deps.Conversations == nil {
		return dto
	}
	recent, err := h.deps.Conversations.ListRecentMessagesBefore(ctx, conversation.ID, 1<<62, 0, 1)
	if err != nil || len(recent) == 0 {
		return dto
	}
	last := recent[len(recent)-1]
	dto.LastMessagePreview = truncateChatText(last.Text, maxChatPreviewRunes)
	dto.LastMessageRole = chatMessageRole(last.Role)
	return dto
}

func newChatConversationMessageDTO(message *domain.ConversationMessage) ChatConversationMessageDTO {
	return ChatConversationMessageDTO{
		ID:        message.ID,
		JobID:     message.JobID,
		Seq:       message.Seq,
		Role:      chatMessageRole(message.Role),
		Text:      truncateChatText(message.Text, maxChatMessageRunes),
		CreatedAt: message.CreatedAt,
	}
}

func chatConversationTitle(conversation *domain.Conversation) string {
	title := strings.TrimSpace(conversation.Title)
	if title == "" {
		return "НейроХаб диалог"
	}
	return truncateChatText(title, maxChatTitleRunes)
}

func chatMessageRole(role domain.ConversationMessageRole) string {
	if role == domain.ConversationRoleAssistant {
		return "bot"
	}
	return "user"
}

func truncateChatText(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	defaultLimit        = 20
	maxLimit            = 100
	maxChatTitleRunes   = 80
	maxChatPreviewRunes = 160
	maxChatMessageRunes = 100_000
)

func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func parseAfterSeq(r *http.Request) int64 {
	raw := r.URL.Query().Get("after_seq")
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
