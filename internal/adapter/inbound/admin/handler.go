// Package admin exposes a guarded HTTP API for operators to inspect and triage
// jobs, users and deliveries. It returns DTOs (never raw domain structs) and
// only allows narrow audited mutations such as guarded DLQ replay.
package admin

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/pricingcatalog"
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
	operatorDLQBatchReplayLimit    = 25
	defaultAdminRateLimit          = 120
)

var errOperatorReplayUnavailable = errors.New("operator replay unavailable")

type paymentOverviewReader interface {
	ListIntents(ctx context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error)
	ListEvents(ctx context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error)
	WebhookInboxStats(ctx context.Context, provider domain.PaymentProviderCode) (domain.PaymentWebhookInboxStats, error)
}

type maintenanceOverviewReader interface {
	RetentionStatus(ctx context.Context, now time.Time) (domain.RetentionStatus, error)
	RetentionDryRun(ctx context.Context, now time.Time, limit int) (domain.RetentionDryRun, error)
	AnalyticsAggregationStatus(ctx context.Context) (domain.AnalyticsAggregationStatus, error)
	OldestHotRows(ctx context.Context) (domain.OldestHotRowsReport, error)
	OrphanArtifactsCount(ctx context.Context, now time.Time) (domain.OrphanArtifactsReport, error)
}

type retentionCleanupRunner interface {
	Cleanup(ctx context.Context) error
}

// Config holds admin API settings.
type Config struct {
	// Token must be presented in the X-Admin-Token header. Empty tokens fail
	// closed; local/dev callers should set an explicit admin token too.
	Token string
	// Runtime is a sanitized non-secret snapshot of runtime policy/config used
	// by read-only operator views. It must never contain raw secrets or model IDs.
	Runtime RuntimeSnapshot
	// DefaultRole is the backend-enforced role for the legacy single-token
	// model. Empty/invalid values default to owner for backward compatibility.
	DefaultRole domain.OperatorRole
	// RateLimit bounds protected admin endpoints per authenticated actor.
	RateLimit AdminRateLimitConfig
}

// AdminRateLimitConfig controls protected admin endpoint request limits.
type AdminRateLimitConfig struct {
	Disabled bool
	Limit    int
	Window   time.Duration
	Limiter  ratelimit.KeyLimiter
}

// Deps are the repositories the admin API reads from.
type Deps struct {
	Jobs       domain.JobRepository
	Users      domain.UserRepository
	Deliveries domain.DeliveryRepository
	Audits     domain.OperatorAuditRepository
	Referrals  domain.ReferralRepository
	// ProviderTasks is optional; when set, DLQ views can show bounded provider
	// attempt/error classes and enforce paid-provider replay guard rails.
	ProviderTasks domain.ProviderTaskRepository
	// UnitOfWork is required for guarded replay because job status and queued
	// outbox event must be persisted atomically.
	UnitOfWork uow.Manager
	// Billing is optional; when set, user responses include the credit balance.
	Billing domain.BillingRepository
	// Payment is optional; when set, overview can report safe payment/webhook
	// backlog counters without exposing raw provider payloads.
	Payment          paymentOverviewReader
	Maintenance      maintenanceOverviewReader
	RetentionCleanup retentionCleanupRunner
	// PricingCache is the single runtime generation pricing catalog used by
	// Mini App, VK bot and job creation paths.
	PricingCache *pricingcatalog.RuntimeCatalogCache
}

// Handler serves the admin endpoints.
type Handler struct {
	cfg         Config
	deps        Deps
	rateMu      sync.Mutex
	rateBuckets map[string]adminRateBucket
}

// NewHandler builds an admin Handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	cfg.RateLimit = normalizeAdminRateLimit(cfg.RateLimit)
	return &Handler{cfg: cfg, deps: deps, rateBuckets: map[string]adminRateBucket{}}
}

// Routes returns an http.Handler with the admin routes registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/overview", h.protected("admin_overview_get", domain.OperatorPermissionSystemStatusRead, h.getOverview))
	mux.HandleFunc("GET /admin/access/operator", h.protected("admin_operator_access_get", domain.OperatorPermissionAccessRead, h.getOperatorAccess))
	mux.HandleFunc("GET /admin/jobs/operator", h.protected("admin_operator_jobs_list", domain.OperatorPermissionJobsSafeRead, h.listOperatorJobs))
	mux.HandleFunc("GET /admin/jobs/queue", h.protected("admin_operator_queue_get", domain.OperatorPermissionQueueRead, h.getOperatorQueue))
	mux.HandleFunc("GET /admin/jobs/dlq", h.protected("admin_operator_dlq_list", domain.OperatorPermissionQueueRead, h.listOperatorDLQ))
	mux.HandleFunc("POST /admin/jobs/dlq/replay", h.protected("admin_operator_dlq_batch_replay", domain.OperatorPermissionDLQReplay, h.replayOperatorDLQBatch))
	mux.HandleFunc("POST /admin/jobs/{id}/replay", h.protected("admin_operator_job_replay", domain.OperatorPermissionDLQReplay, h.replayOperatorJob))
	mux.HandleFunc("GET /admin/jobs/{id}/operator", h.protected("admin_operator_job_get", domain.OperatorPermissionJobsSafeRead, h.getOperatorJob))
	mux.HandleFunc("GET /admin/providers/operator", h.protected("admin_operator_providers_get", domain.OperatorPermissionProviderHealth, h.getOperatorProviders))
	mux.HandleFunc("GET /admin/pricing/operator", h.protected("admin_operator_pricing_get", domain.OperatorPermissionSystemStatusRead, h.getOperatorPricing))
	mux.HandleFunc("GET /admin/media-safety/operator", h.protected("admin_operator_media_safety_get", domain.OperatorPermissionProviderHealth, h.getOperatorMediaSafety))
	mux.HandleFunc("GET /admin/config-health/operator", h.protected("admin_operator_config_health_get", domain.OperatorPermissionSystemStatusRead, h.getOperatorConfigHealth))
	mux.HandleFunc("GET /admin/retention/operator/status", h.protected("admin_operator_retention_status_get", domain.OperatorPermissionRetentionRead, h.getOperatorRetentionStatus))
	mux.HandleFunc("GET /admin/retention/operator/dry-run", h.protected("admin_operator_retention_dry_run_get", domain.OperatorPermissionRetentionDryRun, h.getOperatorRetentionDryRun))
	mux.HandleFunc("POST /admin/retention/operator/run-cleanup", h.protected("admin_operator_retention_cleanup_post", domain.OperatorPermissionRetentionCleanup, h.postOperatorRetentionCleanup))
	mux.HandleFunc("GET /admin/analytics/operator/status", h.protected("admin_operator_analytics_status_get", domain.OperatorPermissionAnalyticsRead, h.getOperatorAnalyticsStatus))
	mux.HandleFunc("GET /admin/data/operator/hot-rows", h.protected("admin_operator_hot_rows_get", domain.OperatorPermissionRetentionRead, h.getOperatorHotRows))
	mux.HandleFunc("GET /admin/artifacts/operator/orphans", h.protected("admin_operator_orphan_artifacts_get", domain.OperatorPermissionRetentionRead, h.getOperatorOrphanArtifacts))
	mux.HandleFunc("GET /admin/users/operator", h.protected("admin_operator_users_get", domain.OperatorPermissionUsersSafeRead, h.getOperatorUsers))
	mux.HandleFunc("GET /admin/referrals/operator", h.protected("admin_operator_referrals_get", domain.OperatorPermissionReferralsRead, h.getOperatorReferrals))
	mux.HandleFunc("GET /admin/audit/operator", h.protected("admin_operator_audit_get", domain.OperatorPermissionAuditRead, h.getOperatorAudit))
	mux.HandleFunc("GET /admin/jobs", h.protected("admin_jobs_list", domain.OperatorPermissionRolePolicyManage, h.listJobs))
	mux.HandleFunc("GET /admin/jobs/{id}", h.protected("admin_job_get", domain.OperatorPermissionRolePolicyManage, h.getJob))
	mux.HandleFunc("GET /admin/users/{id}", h.protected("admin_user_get", domain.OperatorPermissionRolePolicyManage, h.getUser))
	mux.HandleFunc("GET /admin/deliveries/{id}", h.protected("admin_delivery_get", domain.OperatorPermissionRolePolicyManage, h.getDelivery))
	mux.HandleFunc("GET /admin/referrals/codes/{code}/stats", h.protected("admin_referral_stats_get", domain.OperatorPermissionReferralsRead, h.getReferralCodeStats))
	mux.HandleFunc("GET /admin/referrals/suspicious", h.protected("admin_referral_suspicious_list", domain.OperatorPermissionReferralsRead, h.listSuspiciousReferrals))
	mux.HandleFunc("POST /admin/referrals/codes/{code}/freeze", h.protected("admin_referral_freeze_future_flag", domain.OperatorPermissionRolePolicyManage, h.freezeReferralBonusFutureFlag))
	return mux
}

func (h *Handler) protected(action string, permission domain.OperatorPermission, next http.HandlerFunc) http.HandlerFunc {
	return h.auth(h.operatorAction(action, h.rateLimit(h.requirePermission(permission, next))))
}

// auth wraps a handler with the admin-token check.
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminTokenEqual(r.Header.Get("X-Admin-Token"), h.cfg.Token) {
			metrics.AuthFailures.WithLabelValues("admin_api", "invalid_admin_token").Inc()
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		identity := h.operatorIdentity()
		next(w, r.WithContext(context.WithValue(r.Context(), operatorIdentityContextKey{}, identity)))
	}
}

type operatorIdentityContextKey struct{}

type operatorIdentity struct {
	Role     domain.OperatorRole
	ActorRef string
}

func (h *Handler) operatorIdentity() operatorIdentity {
	role := h.cfg.DefaultRole
	if !role.Valid() {
		role = domain.OperatorRoleOwner
	}
	return operatorIdentity{
		Role:     role,
		ActorRef: safeStringRef("operator", h.cfg.Token),
	}
}

func operatorIdentityFromContext(ctx context.Context) operatorIdentity {
	identity, ok := ctx.Value(operatorIdentityContextKey{}).(operatorIdentity)
	if !ok || !identity.Role.Valid() {
		return operatorIdentity{Role: domain.OperatorRoleOwner, ActorRef: "operator_unknown"}
	}
	if strings.TrimSpace(identity.ActorRef) == "" {
		identity.ActorRef = "operator_unknown"
	}
	return identity
}

func (h *Handler) requirePermission(permission domain.OperatorPermission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := operatorIdentityFromContext(r.Context())
		if !identity.Role.HasPermission(permission) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

type adminRateBucket struct {
	Count    int
	ResetAt  time.Time
	LastSeen time.Time
}

func normalizeAdminRateLimit(in AdminRateLimitConfig) AdminRateLimitConfig {
	if in.Limit <= 0 {
		in.Limit = defaultAdminRateLimit
	}
	if in.Window <= 0 {
		in.Window = time.Minute
	}
	return in
}

func (h *Handler) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.RateLimit.Disabled {
			next(w, r)
			return
		}
		identity := operatorIdentityFromContext(r.Context())
		allowed, err := h.allowAdminRequest(r.Context(), "admin:"+identity.ActorRef, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
			return
		}
		if !allowed {
			writeError(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		next(w, r)
	}
}

func (h *Handler) allowAdminRequest(ctx context.Context, key string, now time.Time) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "operator_unknown"
	}
	if h.cfg.RateLimit.Limiter != nil {
		return h.cfg.RateLimit.Limiter.Allow(ctx, key)
	}
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	bucket := h.rateBuckets[key]
	if bucket.ResetAt.IsZero() || !now.Before(bucket.ResetAt) {
		bucket = adminRateBucket{ResetAt: now.Add(h.cfg.RateLimit.Window)}
	}
	bucket.Count++
	bucket.LastSeen = now
	h.rateBuckets[key] = bucket
	if len(h.rateBuckets) > 1024 {
		for candidate, candidateBucket := range h.rateBuckets {
			if now.After(candidateBucket.ResetAt) || now.Sub(candidateBucket.LastSeen) > h.cfg.RateLimit.Window*2 {
				delete(h.rateBuckets, candidate)
			}
		}
	}
	return bucket.Count <= h.cfg.RateLimit.Limit, nil
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
		h.recordOperatorAudit(r, action, result)
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
		h.retentionOverviewCard(r.Context(), now),
		h.paymentReconciliationOverviewCard(r.Context(), now),
	}
	writeJSON(w, http.StatusOK, OverviewDTO{GeneratedAt: now, Cards: cards})
}

func (h *Handler) listOperatorJobs(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, _ := parsePagination(r)
	filter, ok := parseOperatorJobFilter(w, r)
	if !ok {
		return
	}
	cursorRaw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	cursor, ok := parseOperatorJobCursor(w, cursorRaw)
	if !ok {
		return
	}
	jobs, err := h.deps.Jobs.ListCursor(r.Context(), filter, limit+1, cursor)
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
	nextCursor := ""
	if hasMore && len(jobs) > 0 {
		nextCursor = encodeOperatorJobCursor(jobs[len(jobs)-1])
	}
	writeJSON(w, http.StatusOK, OperatorJobsDTO{
		GeneratedAt: now,
		Items:       items,
		Pagination:  pagination{Limit: limit, Count: len(items), HasMore: hasMore, Cursor: cursorRaw, NextCursor: nextCursor},
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

func (h *Handler) listOperatorDLQ(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit := parseOperatorDLQLimit(r.URL.Query().Get("limit"))
	status := domain.JobStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = domain.JobStatusFailedRetryable
	}
	if status != domain.JobStatusFailedRetryable && status != domain.JobStatusFailedTerminal {
		writeError(w, http.StatusBadRequest, "invalid dlq status")
		return
	}
	cursorRaw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	cursor, ok := parseOperatorJobCursor(w, cursorRaw)
	if !ok {
		return
	}
	filter := domain.JobFilter{Status: status}
	if errClass := sanitizeOperatorToken(r.URL.Query().Get("error_class")); errClass != "" {
		filter.ErrorCode = errClass
	}
	jobs, err := h.deps.Jobs.ListCursor(r.Context(), filter, limit+1, cursor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list operator dlq failed")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	now := time.Now().UTC()
	items := make([]OperatorDLQItemDTO, 0, len(jobs))
	for _, job := range jobs {
		item, ierr := h.operatorDLQItem(r.Context(), job, now, true, false)
		if ierr != nil {
			writeError(w, http.StatusInternalServerError, "build operator dlq failed")
			return
		}
		items = append(items, item)
	}
	nextCursor := ""
	if hasMore && len(jobs) > 0 {
		nextCursor = encodeOperatorJobCursor(jobs[len(jobs)-1])
	}
	writeJSON(w, http.StatusOK, OperatorDLQDTO{
		GeneratedAt: now,
		Items:       items,
		Pagination:  pagination{Limit: limit, Count: len(items), HasMore: hasMore, Cursor: cursorRaw, NextCursor: nextCursor},
		Replay:      operatorDLQReplayPolicy(),
		Notes: []string{
			"DLQ is derived from persisted failed jobs and bounded provider task metadata.",
			"Batch replay skips paid/provider jobs; use single replay with explicit override after operator triage.",
		},
	})
}

func (h *Handler) replayOperatorJob(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil || h.deps.UnitOfWork == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req OperatorDLQReplayRequestDTO
	if ok := decodeOptionalJSON(w, r, &req); !ok {
		return
	}
	job, err := h.deps.Jobs.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get operator replay job failed")
		return
	}
	result, replayed := h.replayOneOperatorJob(r.Context(), job, false, req.AllowPaidProvider)
	out := OperatorDLQReplayResultDTO{
		GeneratedAt: time.Now().UTC(),
		Requested:   1,
		BatchLimit:  operatorDLQBatchReplayLimit,
	}
	if replayed {
		out.Replayed = append(out.Replayed, result)
	} else {
		out.Skipped = append(out.Skipped, result)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) replayOperatorDLQBatch(w http.ResponseWriter, r *http.Request) {
	if h.deps.Jobs == nil || h.deps.UnitOfWork == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	var req OperatorDLQReplayRequestDTO
	if ok := decodeOptionalJSON(w, r, &req); !ok {
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > operatorDLQBatchReplayLimit {
		limit = operatorDLQBatchReplayLimit
	}
	jobs, ok := h.operatorReplayBatchCandidates(w, r, req, limit)
	if !ok {
		return
	}
	out := OperatorDLQReplayResultDTO{
		GeneratedAt: time.Now().UTC(),
		Requested:   len(jobs),
		BatchLimit:  operatorDLQBatchReplayLimit,
	}
	for _, job := range jobs {
		// Batch replay intentionally ignores AllowPaidProvider. Paid/provider
		// jobs require single-job operator triage.
		result, replayed := h.replayOneOperatorJob(r.Context(), job, true, false)
		if replayed {
			out.Replayed = append(out.Replayed, result)
		} else {
			out.Skipped = append(out.Skipped, result)
		}
	}
	writeJSON(w, http.StatusOK, out)
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
	if raw := strings.TrimSpace(query.Get("user_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return filter, false
		}
		filter.UserID = &id
	}
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
	if provider := strings.TrimSpace(query.Get("provider")); provider != "" {
		filter.Provider = sanitizeOperatorToken(provider)
		if filter.Provider == "" {
			writeError(w, http.StatusBadRequest, "invalid provider")
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

type operatorJobCursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func parseOperatorJobCursor(w http.ResponseWriter, raw string) (*domain.JobCursor, bool) {
	if raw == "" {
		return nil, true
	}
	if len(raw) > 1024 {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return nil, false
	}
	var payload operatorJobCursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil || payload.ID == uuid.Nil || payload.CreatedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return nil, false
	}
	return &domain.JobCursor{CreatedAt: payload.CreatedAt, ID: payload.ID}, true
}

func encodeOperatorJobCursor(job *domain.Job) string {
	if job == nil || job.ID == uuid.Nil || job.CreatedAt.IsZero() {
		return ""
	}
	payload := operatorJobCursorPayload{CreatedAt: job.CreatedAt.UTC(), ID: job.ID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
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
		DLQ: OperatorDLQSummaryDTO{
			Status:           queueMetricStatus(retryable, 1, operatorQueueWarningThreshold),
			Reason:           "DLQ is derived from persisted failed jobs; replay tools are guarded and audited.",
			RetryableCount:   retryable.count,
			TerminalCount:    terminalFailed.count,
			BatchReplayLimit: operatorDLQBatchReplayLimit,
		},
		ProviderCircuit: OperatorQueueNotWiredDTO{
			Status: overviewStatusNotWired,
			Reason: "Provider circuit state needs a dedicated bounded provider health endpoint before UI can mark it healthy.",
		},
		Notes: []string{"Snapshot from persisted job states; guarded DLQ replay is available via dedicated operator endpoints."},
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
		Summary: "Shows queued jobs and bounded persisted DLQ state; Redis stream counters stay private.",
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

func (h *Handler) operatorReplayBatchCandidates(w http.ResponseWriter, r *http.Request, req OperatorDLQReplayRequestDTO, limit int) ([]*domain.Job, bool) {
	if len(req.JobIDs) > 0 {
		if len(req.JobIDs) > operatorDLQBatchReplayLimit {
			writeError(w, http.StatusBadRequest, "too many job ids")
			return nil, false
		}
		jobs := make([]*domain.Job, 0, len(req.JobIDs))
		for _, raw := range req.JobIDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid job id")
				return nil, false
			}
			job, err := h.deps.Jobs.GetByID(r.Context(), id)
			if err != nil {
				writeNotFoundOr500(w, err, "get operator replay job failed")
				return nil, false
			}
			jobs = append(jobs, job)
		}
		return jobs, true
	}
	filter := domain.JobFilter{Status: domain.JobStatusFailedRetryable}
	if errClass := sanitizeOperatorToken(req.ErrorClass); errClass != "" {
		filter.ErrorCode = errClass
	}
	jobs, err := h.deps.Jobs.ListCursor(r.Context(), filter, limit, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list operator replay jobs failed")
		return nil, false
	}
	return jobs, true
}

func (h *Handler) replayOneOperatorJob(ctx context.Context, job *domain.Job, batch bool, allowPaidProvider bool) (OperatorDLQReplayItemDTO, bool) {
	item := OperatorDLQReplayItemDTO{
		LookupID:  job.ID.String(),
		DisplayID: safeUUIDRef("job", job.ID),
		Status:    string(job.Status),
		Result:    "skipped",
	}
	tasks, err := h.operatorProviderTasks(ctx, job.ID)
	if err != nil {
		item.Reason = "provider task lookup failed"
		return item, false
	}
	safe, reason := operatorReplaySafety(job, tasks, batch, allowPaidProvider)
	if !safe {
		item.Reason = reason
		return item, false
	}
	if err := h.requeueOperatorJob(ctx, job); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			item.Reason = "job status changed before replay"
			return item, false
		}
		if errors.Is(err, errOperatorReplayUnavailable) {
			item.Reason = "replay runtime is not configured"
			return item, false
		}
		item.Reason = "replay failed"
		return item, false
	}
	item.Status = string(domain.JobStatusQueued)
	item.Result = "replayed"
	return item, true
}

func (h *Handler) requeueOperatorJob(ctx context.Context, job *domain.Job) error {
	if h.deps.UnitOfWork == nil {
		return errOperatorReplayUnavailable
	}
	return h.deps.UnitOfWork.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if repos.Jobs == nil || repos.Outbox == nil {
			return errOperatorReplayUnavailable
		}
		if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusFailedRetryable, domain.JobStatusQueued, "", ""); err != nil {
			return err
		}
		queued := *job
		queued.Status = domain.JobStatusQueued
		queued.ErrorCode = ""
		queued.ErrorMessage = ""
		queued.UpdatedAt = time.Now().UTC()
		return repos.Outbox.Add(ctx, operatorJobQueuedOutboxEvent(ctx, &queued))
	})
}

func operatorJobQueuedOutboxEvent(ctx context.Context, job *domain.Job) *domain.OutboxEvent {
	payload, _ := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		Status        domain.JobStatus     `json:"status"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		UserID        uuid.UUID            `json:"user_id"`
		CorrelationID string               `json:"correlation_id"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{
		JobID:         job.ID,
		Status:        job.Status,
		Operation:     job.OperationType,
		Modality:      job.Modality,
		UserID:        job.UserID,
		CorrelationID: job.CorrelationID,
		Traceparent:   tracing.Traceparent(ctx),
	})
	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     outboxrelay.EventJobQueued,
		Payload:       payload,
	}
}

func (h *Handler) operatorDLQItem(ctx context.Context, job *domain.Job, now time.Time, batch bool, allowPaidProvider bool) (OperatorDLQItemDTO, error) {
	tasks, err := h.operatorProviderTasks(ctx, job.ID)
	if err != nil {
		return OperatorDLQItemDTO{}, err
	}
	deliveries, err := h.operatorDeliveries(ctx, job.ID)
	if err != nil {
		return OperatorDLQItemDTO{}, err
	}
	deliverySummary := summarizeOperatorDelivery(deliveries)
	lastProviderClass := lastProviderErrorClass(tasks)
	lastErrorClass := sanitizeOperatorToken(job.ErrorCode)
	if lastErrorClass == "" {
		lastErrorClass = lastProviderClass
	}
	if lastErrorClass == "" {
		lastErrorClass = deliverySummary.LastErrorClass
	}
	safe, reason := operatorReplaySafety(job, tasks, batch, allowPaidProvider)
	return OperatorDLQItemDTO{
		Job:                 newOperatorJobListItem(job, now),
		AttemptCount:        operatorAttemptCount(tasks, deliveries),
		ProviderTaskCount:   len(tasks),
		LastErrorClass:      lastErrorClass,
		LastProviderClass:   lastProviderClass,
		SafeReplay:          safe,
		ReplayBlockedReason: reason,
		ReplayTarget:        string(domain.JobStatusQueued),
	}, nil
}

func (h *Handler) operatorProviderTasks(ctx context.Context, jobID uuid.UUID) ([]*domain.ProviderTask, error) {
	if h.deps.ProviderTasks == nil {
		return nil, nil
	}
	return h.deps.ProviderTasks.ListByJob(ctx, jobID)
}

func operatorReplaySafety(job *domain.Job, tasks []*domain.ProviderTask, batch bool, allowPaidProvider bool) (bool, string) {
	if job.Status != domain.JobStatusFailedRetryable {
		return false, "only failed_retryable jobs can be replayed"
	}
	if job.CostCaptured > 0 {
		return false, "captured jobs require manual financial triage"
	}
	if hasProviderReplayRisk(job, tasks) {
		if batch {
			return false, "batch replay skips paid/provider jobs"
		}
		if !allowPaidProvider {
			return false, "paid/provider replay requires explicit single-job override"
		}
	}
	return true, ""
}

func hasProviderReplayRisk(job *domain.Job, tasks []*domain.ProviderTask) bool {
	return job.CostEstimate > 0 || job.CostReserved > 0 || job.ProviderID != nil || job.ModelID != nil || len(tasks) > 0
}

func operatorAttemptCount(tasks []*domain.ProviderTask, deliveries []OperatorDeliveryAttempt) int {
	maxAttempt := 0
	for _, task := range tasks {
		if task.AttemptNo > maxAttempt {
			maxAttempt = task.AttemptNo
		}
	}
	for _, delivery := range deliveries {
		if delivery.AttemptNo > maxAttempt {
			maxAttempt = delivery.AttemptNo
		}
	}
	if maxAttempt <= 0 {
		return 1
	}
	return maxAttempt
}

func lastProviderErrorClass(tasks []*domain.ProviderTask) string {
	for i := len(tasks) - 1; i >= 0; i-- {
		if class := sanitizeOperatorToken(string(tasks[i].ErrorClass)); class != "" {
			return class
		}
	}
	return ""
}

func operatorDLQReplayPolicy() OperatorDLQReplayPolicyDTO {
	return OperatorDLQReplayPolicyDTO{
		SingleAllowedStatuses:  []string{string(domain.JobStatusFailedRetryable)},
		BatchLimit:             operatorDLQBatchReplayLimit,
		BatchSkipsPaidProvider: true,
		Notes: []string{
			"Batch replay skips paid/provider jobs and is capped.",
			"Single replay of paid/provider jobs requires explicit operator override.",
			"Captured jobs are blocked from replay and require financial triage.",
		},
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

func parseOperatorDLQLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return operatorDLQBatchReplayLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return operatorDLQBatchReplayLimit
	}
	if limit > operatorDLQBatchReplayLimit {
		return operatorDLQBatchReplayLimit
	}
	return limit
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
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
