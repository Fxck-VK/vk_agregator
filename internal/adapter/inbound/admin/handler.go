// Package admin exposes a read-only HTTP API for operators to inspect jobs,
// users and deliveries. It returns DTOs (never raw domain structs) and supports
// pagination and filtering on the jobs listing. It performs no mutations.
package admin

import (
	"context"
	"crypto/subtle"
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
	overviewStatusNotWired  = "not_wired"
)

type paymentOverviewReader interface {
	ListIntents(ctx context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error)
	ListEvents(ctx context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error)
}

// Config holds admin API settings.
type Config struct {
	// Token, when non-empty, must be presented in the X-Admin-Token header.
	Token string
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
		{
			ID:      "media_safety",
			Title:   "Media safety",
			Status:  overviewStatusNotWired,
			Summary: "Media policy metrics exist outside this UI; safe admin aggregation is a follow-up stage.",
		},
		h.paymentReconciliationOverviewCard(r.Context(), now),
	}
	writeJSON(w, http.StatusOK, OverviewDTO{GeneratedAt: now, Cards: cards})
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
