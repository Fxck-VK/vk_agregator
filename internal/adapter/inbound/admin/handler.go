// Package admin exposes a read-only HTTP API for operators to inspect jobs,
// users and deliveries. It returns DTOs (never raw domain structs) and supports
// pagination and filtering on the jobs listing. It performs no mutations.
package admin

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

const (
	defaultLimit = 20
	maxLimit     = 100

	defaultReferralSuspiciousMinRegistered = 10
	defaultReferralSuspiciousMinTotal      = 50

	overviewCountLimit      = maxLimit + 1
	overviewStalePaymentAge = 5 * time.Minute
	overviewStatusOK        = "ok"
	overviewStatusWarning   = "warning"
	overviewStatusCritical  = "critical"
	overviewStatusNotWired  = "not_wired"

	operatorQueueWarningThreshold  = 10
	operatorQueueCriticalThreshold = 50
)

type paymentOverviewReader interface {
	ListIntents(ctx context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error)
	ListEvents(ctx context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error)
}

// Config holds admin API settings.
type Config struct {
	// Token, when non-empty, must be presented in the X-Admin-Token header.
	Token string
	// Runtime is a sanitized non-secret snapshot of runtime policy/config used
	// by read-only operator views. It must never contain raw secrets or model IDs.
	Runtime RuntimeSnapshot
}

// Deps are the repositories the admin API reads from.
type Deps struct {
	Jobs       domain.JobRepository
	Users      domain.UserRepository
	Deliveries domain.DeliveryRepository
	Referrals  domain.ReferralRepository
	// Billing is optional; when set, user responses include the credit balance.
	Billing domain.BillingRepository
	// Payment is optional; when set, overview can report safe payment/webhook
	// backlog counters without exposing raw provider payloads.
	Payment paymentOverviewReader
}

// Handler serves the admin endpoints.
type Handler struct {
	cfg  Config
	deps Deps
}

// NewHandler builds an admin Handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	return &Handler{cfg: cfg, deps: deps}
}

// Routes returns an http.Handler with the admin routes registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/overview", h.auth(h.operatorAction("admin_overview_get", h.getOverview)))
	mux.HandleFunc("GET /admin/jobs/operator", h.auth(h.operatorAction("admin_operator_jobs_list", h.listOperatorJobs)))
	mux.HandleFunc("GET /admin/jobs/queue", h.auth(h.operatorAction("admin_operator_queue_get", h.getOperatorQueue)))
	mux.HandleFunc("GET /admin/jobs/{id}/operator", h.auth(h.operatorAction("admin_operator_job_get", h.getOperatorJob)))
	mux.HandleFunc("GET /admin/providers/operator", h.auth(h.operatorAction("admin_operator_providers_get", h.getOperatorProviders)))
	mux.HandleFunc("GET /admin/media-safety/operator", h.auth(h.operatorAction("admin_operator_media_safety_get", h.getOperatorMediaSafety)))
	mux.HandleFunc("GET /admin/config-health/operator", h.auth(h.operatorAction("admin_operator_config_health_get", h.getOperatorConfigHealth)))
	mux.HandleFunc("GET /admin/jobs", h.auth(h.operatorAction("admin_jobs_list", h.listJobs)))
	mux.HandleFunc("GET /admin/jobs/{id}", h.auth(h.operatorAction("admin_job_get", h.getJob)))
	mux.HandleFunc("GET /admin/users/{id}", h.auth(h.operatorAction("admin_user_get", h.getUser)))
	mux.HandleFunc("GET /admin/deliveries/{id}", h.auth(h.operatorAction("admin_delivery_get", h.getDelivery)))
	mux.HandleFunc("GET /admin/referrals/codes/{code}/stats", h.auth(h.operatorAction("admin_referral_stats_get", h.getReferralCodeStats)))
	mux.HandleFunc("GET /admin/referrals/suspicious", h.auth(h.operatorAction("admin_referral_suspicious_list", h.listSuspiciousReferrals)))
	mux.HandleFunc("POST /admin/referrals/codes/{code}/freeze", h.auth(h.operatorAction("admin_referral_freeze_future_flag", h.freezeReferralBonusFutureFlag)))
	return mux
}

// auth wraps a handler with the optional admin-token check.
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.Token != "" && !adminTokenEqual(r.Header.Get("X-Admin-Token"), h.cfg.Token) {
			metrics.AuthFailures.WithLabelValues("admin_api", "invalid_admin_token").Inc()
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

type actionStatusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *actionStatusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (h *Handler) operatorAction(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &actionStatusRecorder{ResponseWriter: w, code: http.StatusOK}
		next(rec, r)
		result := "success"
		if rec.code >= 400 {
			result = "error"
		}
		metrics.AdminActions.WithLabelValues(action, result).Inc()
	}
}

func adminTokenEqual(got, want string) bool {
	if want == "" || got == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) getOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	cards := []OverviewCardDTO{
		{
			ID:      "api",
			Title:   "API",
			Status:  overviewStatusOK,
			Summary: "Protected admin API is responding with a bounded read-only overview.",
			Metrics: []OverviewMetricDTO{{Label: "auth", Value: "admin token gate"}},
		},
		{
			ID:      "vk_bot",
			Title:   "VK Bot",
			Status:  overviewStatusNotWired,
			Summary: "Live VK control and delivery health needs a dedicated read-only source.",
		},
		{
			ID:      "miniapp",
			Title:   "Mini App",
			Status:  overviewStatusNotWired,
			Summary: "Mini App BFF health is mounted, but per-surface operator health is not wired yet.",
		},
		h.workerOverviewCard(r.Context()),
		h.paymentProcessingOverviewCard(r.Context(), now),
		h.queueBacklogOverviewCard(r.Context()),
		{
			ID:      "active_alerts",
			Title:   "Active alerts",
			Status:  overviewStatusNotWired,
			Summary: "Prometheus and Alertmanager stay private; an aggregated alert-status endpoint is pending.",
		},
		h.providerHealthOverviewCard(r.Context()),
		h.mediaSafetyOverviewCard(r.Context()),
		h.paymentReconciliationOverviewCard(r.Context(), now),
	}
	writeJSON(w, http.StatusOK, OverviewDTO{GeneratedAt: now, Cards: cards})
}

func (h *Handler) listOperatorJobs(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	filter, ok := parseOperatorJobFilter(w, r)
	if !ok {
		return
	}
	jobs, err := h.deps.Jobs.List(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list operator jobs failed")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	now := time.Now().UTC()
	items := make([]OperatorJobListItem, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, newOperatorJobListItem(job, now))
	}
	writeJSON(w, http.StatusOK, OperatorJobsDTO{
		GeneratedAt: now,
		Items:       items,
		Pagination:  pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
		Queue:       h.operatorQueueSummary(r.Context(), now),
	})
}

func (h *Handler) getOperatorQueue(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, h.operatorQueueSummary(r.Context(), now))
}

func (h *Handler) getOperatorJob(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	job, err := h.deps.Jobs.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get operator job failed")
		return
	}
	now := time.Now().UTC()
	detail := OperatorJobDetailDTO{
		Job:         newOperatorJobListItem(job, now),
		AllowedNext: jobStatusStrings(job.Status.AllowedNextStatuses()),
		Artifacts: OperatorJobArtifactsDTO{
			InputRefs:  safeUUIDRefs("artifact", job.InputArtifactIDs),
			OutputRefs: safeUUIDRefs("artifact", job.OutputArtifactIDs),
		},
	}
	if h.deps.Billing != nil {
		res, rerr := h.deps.Billing.GetReservationByJob(r.Context(), job.ID)
		if rerr != nil && !errors.Is(rerr, domain.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "get operator reservation failed")
			return
		}
		if res != nil {
			detail.Reservation = &OperatorReservationDTO{
				Status:    string(res.Status),
				Amount:    res.Amount,
				ExpiresAt: res.ExpiresAt,
				UpdatedAt: res.UpdatedAt,
			}
		}
	}
	deliveries, derr := h.operatorDeliveries(r.Context(), job.ID)
	if derr != nil {
		writeError(w, http.StatusInternalServerError, "get operator deliveries failed")
		return
	}
	detail.DeliveryEvents = deliveries
	detail.Delivery = summarizeOperatorDelivery(deliveries)
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	var filter domain.JobFilter
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		filter.UserID = &id
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.JobStatus(status)
	}
	if op := r.URL.Query().Get("operation"); op != "" {
		filter.Operation = domain.OperationType(op)
	}

	// Fetch one extra row to determine whether more pages exist.
	jobs, err := h.deps.Jobs.List(r.Context(), filter, limit+1, offset)
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
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	job, err := h.deps.Jobs.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get job failed")
		return
	}
	writeJSON(w, http.StatusOK, newJobDTO(job))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user, err := h.deps.Users.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get user failed")
		return
	}
	dto := newUserDTO(user)
	if h.deps.Billing != nil {
		if acc, aerr := h.deps.Billing.GetAccountByUser(r.Context(), user.ID, domain.CurrencyCredits); aerr == nil {
			dto.BalanceCredits = &acc.BalanceCached
		}
	}
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) getDelivery(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	del, err := h.deps.Deliveries.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get delivery failed")
		return
	}
	writeJSON(w, http.StatusOK, newDeliveryDTO(del))
}

func (h *Handler) getReferralCodeStats(w http.ResponseWriter, r *http.Request) {
	if h.deps.Referrals == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	code, ok := referralCodePathValue(w, r)
	if !ok {
		return
	}
	stats, err := h.deps.Referrals.StatsByReferralCode(r.Context(), code)
	if err != nil {
		writeNotFoundOr500(w, err, "get referral stats failed")
		return
	}
	writeJSON(w, http.StatusOK, newReferralStatsDTO(stats))
}

func (h *Handler) listSuspiciousReferrals(w http.ResponseWriter, r *http.Request) {
	if h.deps.Referrals == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, _ := parsePagination(r)
	filter := domain.ReferralSuspiciousFilter{
		Limit:         limit,
		MinRegistered: parsePositiveQueryInt(r, "min_registered", defaultReferralSuspiciousMinRegistered),
		MinTotal:      parsePositiveQueryInt(r, "min_total", defaultReferralSuspiciousMinTotal),
	}
	items, err := h.deps.Referrals.ListSuspiciousReferralCodes(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list suspicious referrals failed")
		return
	}
	dtos := make([]SuspiciousReferralDTO, 0, len(items))
	for _, item := range items {
		base := newReferralStatsDTO(item)
		dtos = append(dtos, SuspiciousReferralDTO{
			ReferralStatsDTO: base,
			Reasons:          suspiciousReferralReasons(base, filter),
		})
	}
	writeJSON(w, http.StatusOK, listResponse[SuspiciousReferralDTO]{
		Items: dtos,
		Pagination: pagination{
			Limit:   filter.Limit,
			Offset:  0,
			Count:   len(dtos),
			HasMore: len(dtos) == filter.Limit,
		},
	})
}

func (h *Handler) freezeReferralBonusFutureFlag(w http.ResponseWriter, r *http.Request) {
	if h.deps.Referrals == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	code, ok := referralCodePathValue(w, r)
	if !ok {
		return
	}
	if _, err := h.deps.Referrals.StatsByReferralCode(r.Context(), code); err != nil {
		writeNotFoundOr500(w, err, "get referral stats failed")
		return
	}
	writeJSON(w, http.StatusNotImplemented, referralFutureFlagDTO{
		Code:    code,
		Enabled: false,
		Status:  "future_flag",
		Message: "manual referral bonus freeze/cancel is reserved for a future workflow and performs no mutation",
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

func parsePositiveQueryInt(r *http.Request, key string, fallback int) int {
	if raw := r.URL.Query().Get(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return fallback
}

func parseOperatorJobFilter(w http.ResponseWriter, r *http.Request) (domain.JobFilter, bool) {
	var filter domain.JobFilter
	query := r.URL.Query()
	if status := strings.TrimSpace(query.Get("status")); status != "" {
		filter.Status = domain.JobStatus(status)
		if !filter.Status.Valid() {
			writeError(w, http.StatusBadRequest, "invalid status")
			return filter, false
		}
	}
	if kind := strings.TrimSpace(query.Get("kind")); kind != "" {
		if op := domain.OperationType(kind); op.Valid() {
			filter.Operation = op
		} else if modality := domain.Modality(kind); modality.Valid() {
			filter.Modality = modality
		} else {
			writeError(w, http.StatusBadRequest, "invalid kind")
			return filter, false
		}
	}
	if op := strings.TrimSpace(query.Get("operation")); op != "" {
		filter.Operation = domain.OperationType(op)
		if !filter.Operation.Valid() {
			writeError(w, http.StatusBadRequest, "invalid operation")
			return filter, false
		}
	}
	if modality := strings.TrimSpace(query.Get("modality")); modality != "" {
		filter.Modality = domain.Modality(modality)
		if !filter.Modality.Valid() {
			writeError(w, http.StatusBadRequest, "invalid modality")
			return filter, false
		}
	}
	if errorClass := strings.TrimSpace(query.Get("error_class")); errorClass != "" {
		filter.ErrorCode = sanitizeOperatorToken(errorClass)
		if filter.ErrorCode == "" {
			writeError(w, http.StatusBadRequest, "invalid error_class")
			return filter, false
		}
	}
	if corr := strings.TrimSpace(query.Get("correlation_id")); corr != "" {
		filter.CorrelationID = sanitizeOperatorToken(corr)
		if filter.CorrelationID == "" {
			writeError(w, http.StatusBadRequest, "invalid correlation_id")
			return filter, false
		}
	}
	if raw := strings.TrimSpace(query.Get("created_from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid created_from")
			return filter, false
		}
		filter.CreatedFrom = &t
	}
	if raw := strings.TrimSpace(query.Get("created_to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid created_to")
			return filter, false
		}
		filter.CreatedTo = &t
	}
	return filter, true
}

func sanitizeOperatorToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == '-' || ch == ':' || ch == '.' {
			continue
		}
		return ""
	}
	return value
}

func referralCodePathValue(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid code")
		return "", false
	}
	return code, true
}

func suspiciousReferralReasons(stats ReferralStatsDTO, filter domain.ReferralSuspiciousFilter) []string {
	var reasons []string
	if filter.MinRegistered > 0 && stats.RegisteredCount >= filter.MinRegistered {
		reasons = append(reasons, "many_registered_not_activated")
	}
	if filter.MinTotal > 0 && stats.InvitedCount >= filter.MinTotal {
		reasons = append(reasons, "high_referral_volume")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "aggregate_threshold")
	}
	return reasons
}

func (h *Handler) workerOverviewCard(ctx context.Context) OverviewCardDTO {
	if h.deps.Jobs == nil {
		return notWiredOverviewCard("workers", "Workers", "Job repository is not configured for worker health summary.")
	}
	queued := h.countJobs(ctx, domain.JobStatusQueued)
	processing := h.countJobsMany(ctx,
		domain.JobStatusDispatchingProvider,
		domain.JobStatusProviderSubmitted,
		domain.JobStatusProviderPending,
		domain.JobStatusProviderProcessing,
		domain.JobStatusPostprocessing,
		domain.JobStatusDelivering,
	)
	retryable := h.countJobs(ctx, domain.JobStatusFailedRetryable)
	terminalFailed := h.countJobs(ctx, domain.JobStatusFailedTerminal)
	status := overviewStatusOK
	if retryable.err || terminalFailed.err || queued.err || processing.err || retryable.count > 0 || terminalFailed.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "workers",
		Title:   "Workers",
		Status:  status,
		Summary: "Worker state is derived from bounded job-status snapshots.",
		Metrics: []OverviewMetricDTO{
			{Label: "queued jobs", Value: queued.display()},
			{Label: "processing jobs", Value: processing.display()},
			{Label: "retryable failures", Value: retryable.display(), Status: metricWarningWhenPositive(retryable)},
			{Label: "terminal failures", Value: terminalFailed.display(), Status: metricWarningWhenPositive(terminalFailed)},
		},
	}
}

func (h *Handler) operatorQueueSummary(ctx context.Context, now time.Time) OperatorQueueSummaryDTO {
	queued := h.countJobs(ctx, domain.JobStatusQueued)
	processing := h.countJobsMany(ctx,
		domain.JobStatusDispatchingProvider,
		domain.JobStatusProviderSubmitted,
		domain.JobStatusProviderPending,
		domain.JobStatusProviderProcessing,
		domain.JobStatusProviderSucceeded,
		domain.JobStatusPostprocessing,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
	)
	retryable := h.countJobs(ctx, domain.JobStatusFailedRetryable)
	terminalFailed := h.countJobs(ctx, domain.JobStatusFailedTerminal)
	degradation := "normal"
	if queued.err || processing.err || retryable.err || terminalFailed.err {
		degradation = "unknown"
	} else if queued.count >= operatorQueueCriticalThreshold || retryable.count >= operatorQueueWarningThreshold {
		degradation = "degraded"
	} else if queued.count > 0 || retryable.count > 0 || terminalFailed.count > 0 {
		degradation = "watch"
	}
	oldestAge := h.oldestQueuedAgeSeconds(ctx, now)
	return OperatorQueueSummaryDTO{
		GeneratedAt:      now,
		DegradationState: degradation,
		Backlog: []OperatorQueueMetricDTO{
			operatorQueueMetric("queued", queued, queueMetricStatus(queued, operatorQueueWarningThreshold, operatorQueueCriticalThreshold)),
			operatorQueueMetric("processing", processing, queueMetricStatus(processing, maxLimit, maxLimit)),
			operatorQueueMetric("retryable", retryable, queueMetricStatus(retryable, 1, operatorQueueWarningThreshold)),
			operatorQueueMetric("terminal_failed", terminalFailed, queueMetricStatus(terminalFailed, 1, operatorQueueWarningThreshold)),
		},
		OldestQueuedAgeSeconds: oldestAge,
		RetryCount:             retryable.count,
		DLQ: OperatorQueueNotWiredDTO{
			Status: overviewStatusNotWired,
			Reason: "Dedicated DLQ/Redis stream source is not exposed to admin API yet; terminal failures are shown separately.",
		},
		ProviderCircuit: OperatorQueueNotWiredDTO{
			Status: overviewStatusNotWired,
			Reason: "Provider circuit state needs a dedicated bounded provider health endpoint before UI can mark it healthy.",
		},
		Notes: []string{"Read-only snapshot from persisted job states; no retry or requeue actions are available here."},
	}
}

func (h *Handler) queueBacklogOverviewCard(ctx context.Context) OverviewCardDTO {
	if h.deps.Jobs == nil {
		return notWiredOverviewCard("queue_backlog", "Queue backlog", "Job repository is not configured for queue summary.")
	}
	queued := h.countJobs(ctx, domain.JobStatusQueued)
	status := overviewStatusOK
	if queued.err || queued.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "queue_backlog",
		Title:   "Queue backlog",
		Status:  status,
		Summary: "Shows queued jobs only; Redis stream and DLQ counters need a dedicated queue endpoint.",
		Metrics: []OverviewMetricDTO{{Label: "queued jobs", Value: queued.display(), Status: metricWarningWhenPositive(queued)}},
	}
}

func (h *Handler) providerHealthOverviewCard(ctx context.Context) OverviewCardDTO {
	if h.deps.Jobs == nil {
		return notWiredOverviewCard("provider_health", "Provider health", "Provider health source is not configured.")
	}
	providerFailed := h.countJobs(ctx, domain.JobStatusProviderFailed)
	status := overviewStatusNotWired
	if providerFailed.err || providerFailed.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "provider_health",
		Title:   "Provider health",
		Status:  status,
		Summary: "Provider-specific circuit state is pending; current card only shows bounded provider-failed jobs.",
		Metrics: []OverviewMetricDTO{{Label: "provider failed jobs", Value: providerFailed.display(), Status: metricWarningWhenPositive(providerFailed)}},
	}
}

func (h *Handler) paymentProcessingOverviewCard(ctx context.Context, now time.Time) OverviewCardDTO {
	if h.deps.Payment == nil {
		return notWiredOverviewCard("provider_webhook", "Provider webhook", "Payment provider webhook reader is not configured.")
	}
	unprocessed := h.countPaymentEvents(ctx, false)
	pending := h.countPaymentIntents(ctx, domain.PaymentIntentFilter{
		Statuses: []domain.PaymentIntentStatus{domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser},
	})
	staleBefore := now.Add(-overviewStalePaymentAge)
	stale := h.countPaymentIntents(ctx, domain.PaymentIntentFilter{
		Statuses:      []domain.PaymentIntentStatus{domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser},
		UpdatedBefore: &staleBefore,
	})
	status := overviewStatusOK
	if unprocessed.err || pending.err || stale.err || unprocessed.count > 0 || stale.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "provider_webhook",
		Title:   "Provider webhook/payment processing",
		Status:  status,
		Summary: "Payment processing health uses safe counts only and never exposes raw provider bodies.",
		Metrics: []OverviewMetricDTO{
			{Label: "unprocessed webhooks", Value: unprocessed.display(), Status: metricWarningWhenPositive(unprocessed)},
			{Label: "pending payment intents", Value: pending.display()},
			{Label: "stale payment intents", Value: stale.display(), Status: metricWarningWhenPositive(stale)},
		},
	}
}

func (h *Handler) paymentReconciliationOverviewCard(ctx context.Context, now time.Time) OverviewCardDTO {
	if h.deps.Payment == nil {
		return notWiredOverviewCard("payment_reconciliation", "Payment reconciliation", "Payment service is not configured for reconciliation summary.")
	}
	staleBefore := now.Add(-overviewStalePaymentAge)
	stale := h.countPaymentIntents(ctx, domain.PaymentIntentFilter{
		Statuses:      []domain.PaymentIntentStatus{domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser},
		UpdatedBefore: &staleBefore,
	})
	status := overviewStatusOK
	if stale.err || stale.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "payment_reconciliation",
		Title:   "Payment reconciliation",
		Status:  status,
		Summary: "Flags stale provider-backed payment intents that should be reconciled by the backend path.",
		Metrics: []OverviewMetricDTO{{Label: "stale payment intents", Value: stale.display(), Status: metricWarningWhenPositive(stale)}},
	}
}

func notWiredOverviewCard(id, title, summary string) OverviewCardDTO {
	return OverviewCardDTO{
		ID:      id,
		Title:   title,
		Status:  overviewStatusNotWired,
		Summary: summary,
	}
}

type overviewCount struct {
	count int
	err   bool
}

func (c overviewCount) display() string {
	if c.err {
		return "unavailable"
	}
	if c.count >= maxLimit {
		return strconv.Itoa(maxLimit) + "+"
	}
	return strconv.Itoa(c.count)
}

func (h *Handler) countJobs(ctx context.Context, status domain.JobStatus) overviewCount {
	jobs, err := h.deps.Jobs.List(ctx, domain.JobFilter{Status: status}, overviewCountLimit, 0)
	if err != nil {
		return overviewCount{err: true}
	}
	return overviewCount{count: boundedOverviewCount(len(jobs))}
}

func (h *Handler) countJobsMany(ctx context.Context, statuses ...domain.JobStatus) overviewCount {
	var total int
	for _, status := range statuses {
		c := h.countJobs(ctx, status)
		if c.err {
			return c
		}
		total += c.count
		if total >= maxLimit {
			return overviewCount{count: maxLimit}
		}
	}
	return overviewCount{count: total}
}

func (h *Handler) countPaymentIntents(ctx context.Context, filter domain.PaymentIntentFilter) overviewCount {
	intents, err := h.deps.Payment.ListIntents(ctx, filter, overviewCountLimit, 0)
	if err != nil {
		return overviewCount{err: true}
	}
	return overviewCount{count: boundedOverviewCount(len(intents))}
}

func (h *Handler) countPaymentEvents(ctx context.Context, processed bool) overviewCount {
	events, err := h.deps.Payment.ListEvents(ctx, domain.PaymentEventFilter{Processed: &processed}, overviewCountLimit, 0)
	if err != nil {
		return overviewCount{err: true}
	}
	return overviewCount{count: boundedOverviewCount(len(events))}
}

func boundedOverviewCount(count int) int {
	if count >= overviewCountLimit {
		return maxLimit
	}
	return count
}

func metricWarningWhenPositive(count overviewCount) string {
	if count.err || count.count > 0 {
		return overviewStatusWarning
	}
	return overviewStatusOK
}

func newOperatorJobListItem(job *domain.Job, now time.Time) OperatorJobListItem {
	age := int64(0)
	if !job.CreatedAt.IsZero() {
		age = int64(now.Sub(job.CreatedAt).Seconds())
		if age < 0 {
			age = 0
		}
	}
	return OperatorJobListItem{
		LookupID:       job.ID.String(),
		DisplayID:      safeUUIDRef("job", job.ID),
		CorrelationRef: safeStringRef("corr", job.CorrelationID),
		Operation:      string(job.OperationType),
		Modality:       string(job.Modality),
		Status:         string(job.Status),
		ErrorClass:     sanitizeOperatorToken(job.ErrorCode),
		CostEstimate:   job.CostEstimate,
		CostReserved:   job.CostReserved,
		CostCaptured:   job.CostCaptured,
		InputCount:     len(job.InputArtifactIDs),
		OutputCount:    len(job.OutputArtifactIDs),
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		AgeSeconds:     age,
	}
}

func (h *Handler) operatorDeliveries(ctx context.Context, jobID uuid.UUID) ([]OperatorDeliveryAttempt, error) {
	if h.deps.Deliveries == nil {
		return nil, nil
	}
	deliveries, err := h.deps.Deliveries.ListByJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	out := make([]OperatorDeliveryAttempt, 0, len(deliveries))
	for _, d := range deliveries {
		artifactRef := ""
		if d.ArtifactID != nil {
			artifactRef = safeUUIDRef("artifact", *d.ArtifactID)
		}
		out = append(out, OperatorDeliveryAttempt{
			Type:        string(d.Type),
			Status:      string(d.Status),
			AttemptNo:   d.AttemptNo,
			ErrorClass:  sanitizeOperatorToken(d.ErrorCode),
			ArtifactRef: artifactRef,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}
	return out, nil
}

func summarizeOperatorDelivery(events []OperatorDeliveryAttempt) OperatorDeliverySummary {
	if len(events) == 0 {
		return OperatorDeliverySummary{Status: "none"}
	}
	last := events[len(events)-1]
	maxAttempt := 0
	for _, event := range events {
		if event.AttemptNo > maxAttempt {
			maxAttempt = event.AttemptNo
		}
	}
	retryCount := maxAttempt - 1
	if retryCount < 0 {
		retryCount = 0
	}
	return OperatorDeliverySummary{
		Status:             last.Status,
		Attempts:           len(events),
		RetryCount:         retryCount,
		LastErrorClass:     last.ErrorClass,
		LastArtifactRef:    last.ArtifactRef,
		LastDeliveryType:   last.Type,
		LastDeliveryStatus: last.Status,
	}
}

func jobStatusStrings(statuses []domain.JobStatus) []string {
	out := make([]string, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, string(status))
	}
	return out
}

func safeUUIDRefs(prefix string, ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		ref := safeUUIDRef(prefix, id)
		if ref != "" {
			out = append(out, ref)
		}
	}
	return out
}

func safeUUIDRef(prefix string, id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return safeStringRef(prefix, id.String())
}

func safeStringRef(prefix, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return prefix + "_" + hex.EncodeToString(sum[:])[:12]
}

func operatorQueueMetric(label string, count overviewCount, status string) OperatorQueueMetricDTO {
	return OperatorQueueMetricDTO{Label: label, Value: count.display(), Status: status}
}

func queueMetricStatus(count overviewCount, warningThreshold, criticalThreshold int) string {
	if count.err {
		return "unknown"
	}
	if criticalThreshold > 0 && count.count >= criticalThreshold {
		return "critical"
	}
	if warningThreshold > 0 && count.count >= warningThreshold {
		return overviewStatusWarning
	}
	return overviewStatusOK
}

func (h *Handler) oldestQueuedAgeSeconds(ctx context.Context, now time.Time) *int64 {
	jobs, err := h.deps.Jobs.List(ctx, domain.JobFilter{Status: domain.JobStatusQueued}, overviewCountLimit, 0)
	if err != nil || len(jobs) == 0 {
		return nil
	}
	oldest := jobs[len(jobs)-1].CreatedAt
	if oldest.IsZero() {
		return nil
	}
	age := int64(now.Sub(oldest).Seconds())
	if age < 0 {
		age = 0
	}
	return &age
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeNotFoundOr500(w http.ResponseWriter, err error, msg string) {
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, msg)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
