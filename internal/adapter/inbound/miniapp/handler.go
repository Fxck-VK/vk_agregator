package miniapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

type contextKey int

const ctxVKUserIDKey contextKey = iota

// Config holds per-deployment miniapp settings.
type Config struct {
	// AppSecret is the VK App's protected key for verifying launch-params
	// signatures. When empty the check is skipped (dev/mock mode).
	AppSecret string
	// LaunchParamsMaxAge is the maximum allowed age of the vk_ts timestamp.
	// Zero disables the age check.
	LaunchParamsMaxAge time.Duration
}

// Deps are the collaborators needed by the miniapp handler.
type Deps struct {
	Users        domain.UserRepository
	Jobs         domain.JobRepository
	Billing      *billingservice.Service
	BillingRepo  domain.BillingRepository
	Orchestrator *joborchestrator.Orchestrator
	Logger       *slog.Logger
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
	mux.HandleFunc("POST /miniapp/jobs", h.auth(h.createJob))
	mux.HandleFunc("GET /miniapp/jobs", h.auth(h.listJobs))
	mux.HandleFunc("GET /miniapp/jobs/{id}", h.auth(h.getJob))
	mux.HandleFunc("GET /miniapp/balance", h.auth(h.getBalance))
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
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			uid, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || uid <= 0 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), ctxVKUserIDKey, uid)
			next(w, r.WithContext(ctx))
			return
		}

		params, err := VerifyLaunchParams(rawParams, h.cfg.AppSecret, h.cfg.LaunchParamsMaxAge)
		if err != nil {
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

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	var req CreateJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	opType, modality, ok := operationMeta(req.Operation)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported operation; allowed: text_generate, image_generate, video_generate")
		return
	}

	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Accept an optional client-supplied idempotency key. The key is scoped to
	// the user so one user cannot replay another user's key.
	clientKey := r.Header.Get("X-Idempotency-Key")
	if clientKey == "" {
		clientKey = uuid.New().String()
	}
	idemKey := fmt.Sprintf("miniapp_job:%d:%s", vkUserID, clientKey)
	correlationID := fmt.Sprintf("miniapp:%d:%s", vkUserID, clientKey)

	params, _ := json.Marshal(struct {
		Prompt string `json:"prompt"`
	}{Prompt: req.Prompt})

	job, err := h.deps.Orchestrator.CreateJob(r.Context(), joborchestrator.CreateJobInput{
		UserID:         user.ID,
		VKPeerID:       vkUserID, // peer_id = user_id for direct messages
		CommandID:      uuid.Nil, // no VK command for mini app path
		Operation:      opType,
		Modality:       modality,
		IdempotencyKey: idemKey,
		CorrelationID:  correlationID,
		Params:         params,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, newJobDTO(job))
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"error":          "insufficient_credits",
			"job_id":         job.ID,
			"status":         string(job.Status),
			"cost_estimate":  job.CostEstimate,
		})
	case errors.Is(err, domain.ErrCostCapExceeded):
		writeError(w, http.StatusBadRequest, "job cost exceeds platform limit")
	default:
		h.logger.Error("miniapp: create job failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
	}
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	defaultLimit = 20
	maxLimit     = 100
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
