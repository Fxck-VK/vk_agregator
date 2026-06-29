package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func (h *Handler) getOperatorUsers(w http.ResponseWriter, r *http.Request) {
	if h.deps.Users == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	now := time.Now().UTC()
	userID, ok := parseOptionalUserID(w, r)
	if !ok {
		return
	}
	dto := OperatorUsersDTO{
		GeneratedAt: now,
		Notes: []string{
			"User lookup is read-only and omits raw VK ids, names, timezone, prompts and private URLs.",
		},
	}
	if userID == nil {
		dto.Notes = append(dto.Notes, "Set user_id to load one protected user summary; no broad PII-heavy user listing is exposed.")
		writeJSON(w, http.StatusOK, dto)
		return
	}
	user, err := h.deps.Users.GetByID(r.Context(), *userID)
	if err != nil {
		writeNotFoundOr500(w, err, "get operator user failed")
		return
	}
	dto.User = h.newOperatorUserSummary(r.Context(), user, now)
	dto.RecentJobs = h.operatorUserRecentJobs(r.Context(), user.ID, now)
	dto.Payment = h.operatorUserPaymentSummary(r.Context(), user.ID)
	dto.Referrals = h.operatorUserReferralSummary(r.Context(), user.ID)
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) getOperatorReferrals(w http.ResponseWriter, r *http.Request) {
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
	suspicious, err := h.deps.Referrals.ListSuspiciousReferralCodes(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list referral console suspicious failed")
		return
	}
	suspiciousDTOs := make([]SuspiciousReferralDTO, 0, len(suspicious))
	for _, item := range suspicious {
		base := newReferralStatsDTO(item)
		suspiciousDTOs = append(suspiciousDTOs, SuspiciousReferralDTO{
			ReferralStatsDTO: base,
			Reasons:          suspiciousReferralReasons(base, filter),
		})
	}
	distribution, err := h.deps.Referrals.ReferralStatusDistribution(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get referral distribution failed")
		return
	}
	var codeStats *ReferralStatsDTO
	if code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code"))); code != "" {
		stats, err := h.deps.Referrals.StatsByReferralCode(r.Context(), code)
		if err != nil {
			writeNotFoundOr500(w, err, "get referral code stats failed")
			return
		}
		dto := newReferralStatsDTO(stats)
		codeStats = &dto
	}
	writeJSON(w, http.StatusOK, OperatorReferralsDTO{
		GeneratedAt:        time.Now().UTC(),
		CodeStats:          codeStats,
		Distribution:       newReferralDistributionDTO(distribution),
		Suspicious:         suspiciousDTOs,
		SuspiciousCriteria: OperatorReferralSuspiciousCriteriaDTO{MinRegistered: filter.MinRegistered, MinTotal: filter.MinTotal},
		Pagination: pagination{
			Limit:   filter.Limit,
			Offset:  0,
			Count:   len(suspiciousDTOs),
			HasMore: len(suspiciousDTOs) == filter.Limit,
		},
		Notes: []string{"Referral console exposes aggregate code-level counters only; invited-user lists and VK ids are omitted."},
	})
}

func (h *Handler) getOperatorAudit(w http.ResponseWriter, r *http.Request) {
	if h.deps.Audits == nil {
		writeJSON(w, http.StatusOK, OperatorAuditLogDTO{
			GeneratedAt: time.Now().UTC(),
			Items:       []OperatorAuditEntryDTO{},
			Pagination:  pagination{Limit: defaultLimit},
			Notes:       []string{"Operator audit repository is not configured yet."},
		})
		return
	}
	limit, offset := parsePagination(r)
	filter, ok := parseOperatorAuditFilter(w, r)
	if !ok {
		return
	}
	entries, err := h.deps.Audits.List(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list operator audit failed")
		return
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	items := make([]OperatorAuditEntryDTO, 0, len(entries))
	for _, entry := range entries {
		items = append(items, newOperatorAuditEntryDTO(entry))
	}
	writeJSON(w, http.StatusOK, OperatorAuditLogDTO{
		GeneratedAt: time.Now().UTC(),
		Items:       items,
		Pagination:  pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
		Notes:       []string{"Audit rows are sanitized; raw request paths, tokens, payloads and ids are not stored."},
	})
}

func (h *Handler) recordOperatorAudit(r *http.Request, action, result string) {
	if h.deps.Audits == nil {
		return
	}
	entryID := uuid.New()
	targetType := operatorTargetType(action)
	identity := operatorIdentityFromContext(r.Context())
	entry := &domain.OperatorAuditEntry{
		ID:         entryID,
		ActorRef:   identity.ActorRef,
		Action:     sanitizeOperatorToken(action),
		TargetType: targetType,
		TargetRef:  safeStringRef("target", targetType+":"+r.URL.Path),
		Result:     operatorAuditResult(result),
		RequestRef: operatorRequestRef(r, entryID),
	}
	if entry.Action == "" {
		entry.Action = "unknown"
	}
	if entry.ActorRef == "" {
		entry.ActorRef = operatorActorRef(h.cfg.Token)
	}
	if entry.TargetType == "" {
		entry.TargetType = "admin"
	}
	if entry.TargetRef == "" {
		entry.TargetRef = safeUUIDRef("target", entryID)
	}
	_ = h.deps.Audits.Create(r.Context(), entry)
}

func (h *Handler) newOperatorUserSummary(ctx context.Context, user *domain.User, now time.Time) *OperatorUserSummaryDTO {
	if user == nil {
		return nil
	}
	return &OperatorUserSummaryDTO{
		UserRef:     safeUUIDRef("user", user.ID),
		Role:        string(user.Role),
		Status:      string(user.Status),
		Locale:      sanitizeOperatorToken(user.Locale),
		RiskClass:   userRiskClass(user.RiskLevel),
		FirstSeenAt: zeroTimeNil(user.FirstSeenAt),
		LastSeenAt:  zeroTimeNil(user.LastSeenAt),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Jobs:        h.operatorUserJobSummary(ctx, user.ID),
		AgeSeconds:  nonNegativeSeconds(now.Sub(user.CreatedAt)),
	}
}

func (h *Handler) operatorUserRecentJobs(ctx context.Context, userID uuid.UUID, now time.Time) []OperatorUserRecentJobDTO {
	if h.deps.Jobs == nil {
		return nil
	}
	jobs, err := h.deps.Jobs.ListByUser(ctx, userID, 10, 0)
	if err != nil {
		return nil
	}
	items := make([]OperatorUserRecentJobDTO, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, OperatorUserRecentJobDTO{
			DisplayID:    safeUUIDRef("job", job.ID),
			Operation:    string(job.OperationType),
			Modality:     string(job.Modality),
			Status:       string(job.Status),
			ErrorClass:   sanitizeOperatorToken(job.ErrorCode),
			CostReserved: job.CostReserved,
			CostCaptured: job.CostCaptured,
			CreatedAt:    job.CreatedAt,
			AgeSeconds:   nonNegativeSeconds(now.Sub(job.CreatedAt)),
		})
	}
	return items
}

func (h *Handler) operatorUserJobSummary(ctx context.Context, userID uuid.UUID) OperatorUserJobSummaryDTO {
	if h.deps.Jobs == nil {
		return OperatorUserJobSummaryDTO{Status: overviewStatusNotWired}
	}
	id := userID
	total := h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id})
	active := overviewCount{count: 0}
	for _, status := range domain.ActiveWorkJobStatuses() {
		active = combineOverviewCounts(active, h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Status: status}))
		if active.err || active.count >= maxLimit {
			break
		}
	}
	succeeded := h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Status: domain.JobStatusSucceeded})
	failed := combineOverviewCounts(
		h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Status: domain.JobStatusFailedRetryable}),
		h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Status: domain.JobStatusFailedTerminal}),
	)
	status := overviewStatusOK
	if total.err || active.err || succeeded.err || failed.err {
		status = "unknown"
	} else if failed.count > 0 {
		status = overviewStatusWarning
	}
	return OperatorUserJobSummaryDTO{
		Status:         status,
		Total:          total.display(),
		Active:         active.display(),
		Succeeded:      succeeded.display(),
		Failed:         failed.display(),
		TextJobs:       h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Modality: domain.ModalityText}).display(),
		ImageJobs:      h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Modality: domain.ModalityImage}).display(),
		VideoJobs:      h.countJobsByFilter(ctx, domain.JobFilter{UserID: &id, Modality: domain.ModalityVideo}).display(),
		RecentPageSize: 10,
	}
}

func (h *Handler) operatorUserPaymentSummary(ctx context.Context, userID uuid.UUID) OperatorUserPaymentSummaryDTO {
	if h.deps.Payment == nil {
		return OperatorUserPaymentSummaryDTO{Status: overviewStatusNotWired}
	}
	id := userID
	intents, err := h.deps.Payment.ListIntents(ctx, domain.PaymentIntentFilter{UserID: &id}, overviewCountLimit, 0)
	if err != nil {
		return OperatorUserPaymentSummaryDTO{Status: "unknown"}
	}
	dto := OperatorUserPaymentSummaryDTO{Status: overviewStatusOK, Total: boundedOverviewCount(len(intents))}
	for _, intent := range intents {
		switch intent.Status {
		case domain.PaymentIntentSucceeded:
			dto.Succeeded++
			dto.CreditsPurchased += intent.Credits
		case domain.PaymentIntentCreated, domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser:
			dto.Pending++
		case domain.PaymentIntentRefunded, domain.PaymentIntentPartiallyRefunded:
			dto.Refunded++
		case domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
			dto.Failed++
		}
	}
	if dto.Pending > 0 || dto.Failed > 0 {
		dto.Status = overviewStatusWarning
	}
	return dto
}

func (h *Handler) operatorUserReferralSummary(ctx context.Context, userID uuid.UUID) OperatorUserReferralSummaryDTO {
	if h.deps.Referrals == nil {
		return OperatorUserReferralSummaryDTO{Status: overviewStatusNotWired}
	}
	dto := OperatorUserReferralSummaryDTO{Status: overviewStatusOK}
	code, err := h.deps.Referrals.GetCodeByUserID(ctx, userID)
	if err == nil && code != nil {
		dto.Code = code.Code
		stats, serr := h.deps.Referrals.CountByReferrerStatus(ctx, userID)
		if serr != nil {
			dto.Status = "unknown"
		} else {
			dto.Invited = stats.Total()
			dto.Registered = stats.RegisteredCount
			dto.Activated = stats.ActivatedCount
			dto.Rewarded = stats.RewardedCount
		}
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		dto.Status = "unknown"
	}
	relation, rerr := h.deps.Referrals.GetReferralByReferredUserID(ctx, userID)
	if rerr == nil && relation != nil {
		dto.InvitedBy = &OperatorUserInvitedByDTO{
			Source:       string(relation.Source),
			Status:       string(relation.Status),
			RewardStatus: string(relation.RewardStatus),
		}
	} else if rerr != nil && !errors.Is(rerr, domain.ErrNotFound) {
		dto.Status = "unknown"
	}
	return dto
}

func parseOptionalUserID(w http.ResponseWriter, r *http.Request) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return nil, false
	}
	return &id, true
}

func parseOperatorAuditFilter(w http.ResponseWriter, r *http.Request) (domain.OperatorAuditFilter, bool) {
	var filter domain.OperatorAuditFilter
	query := r.URL.Query()
	if action := strings.TrimSpace(query.Get("action")); action != "" {
		filter.Action = sanitizeOperatorToken(action)
		if filter.Action == "" {
			writeError(w, http.StatusBadRequest, "invalid action")
			return filter, false
		}
	}
	if targetType := strings.TrimSpace(query.Get("target_type")); targetType != "" {
		filter.TargetType = sanitizeOperatorToken(targetType)
		if filter.TargetType == "" {
			writeError(w, http.StatusBadRequest, "invalid target_type")
			return filter, false
		}
	}
	if result := strings.TrimSpace(query.Get("result")); result != "" {
		switch result {
		case "success", "error":
			filter.Result = result
		default:
			writeError(w, http.StatusBadRequest, "invalid result")
			return filter, false
		}
	}
	return filter, true
}

func newReferralDistributionDTO(stats domain.ReferralStats) OperatorReferralDistributionDTO {
	return OperatorReferralDistributionDTO{
		RegisteredCount: stats.RegisteredCount,
		ActivatedCount:  stats.ActivatedCount,
		RewardedCount:   stats.RewardedCount,
		Total:           stats.Total(),
	}
}

func newOperatorAuditEntryDTO(entry *domain.OperatorAuditEntry) OperatorAuditEntryDTO {
	if entry == nil {
		return OperatorAuditEntryDTO{}
	}
	return OperatorAuditEntryDTO{
		DisplayID:  safeUUIDRef("audit", entry.ID),
		ActorRef:   sanitizeOperatorToken(entry.ActorRef),
		Action:     sanitizeOperatorToken(entry.Action),
		TargetType: sanitizeOperatorToken(entry.TargetType),
		TargetRef:  sanitizeOperatorToken(entry.TargetRef),
		Result:     operatorAuditResult(entry.Result),
		RequestRef: sanitizeOperatorToken(entry.RequestRef),
		CreatedAt:  entry.CreatedAt,
	}
}

func operatorActorRef(token string) string {
	if strings.TrimSpace(token) == "" {
		return "admin_dev"
	}
	return "admin_token"
}

func operatorAuditResult(result string) string {
	if result == "success" {
		return "success"
	}
	return "error"
}

func operatorRequestRef(r *http.Request, fallback uuid.UUID) string {
	for _, header := range []string{"X-Request-ID", "X-Correlation-ID"} {
		if raw := strings.TrimSpace(r.Header.Get(header)); raw != "" {
			return safeStringRef("request", raw)
		}
	}
	return safeUUIDRef("request", fallback)
}

func operatorTargetType(action string) string {
	value := strings.ToLower(action)
	switch {
	case strings.Contains(value, "job") || strings.Contains(value, "queue"):
		return "jobs"
	case strings.Contains(value, "user"):
		return "users"
	case strings.Contains(value, "referral"):
		return "referrals"
	case strings.Contains(value, "payment") || strings.Contains(value, "billing"):
		return "payments"
	case strings.Contains(value, "provider"):
		return "providers"
	case strings.Contains(value, "media"):
		return "media"
	case strings.Contains(value, "config"):
		return "config"
	case strings.Contains(value, "audit"):
		return "audit"
	case strings.Contains(value, "overview"):
		return "overview"
	default:
		return "admin"
	}
}

func userRiskClass(level int) string {
	switch {
	case level >= 80:
		return "high"
	case level >= 40:
		return "medium"
	default:
		return "low"
	}
}

func zeroTimeNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func nonNegativeSeconds(value time.Duration) int64 {
	seconds := int64(value.Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}
