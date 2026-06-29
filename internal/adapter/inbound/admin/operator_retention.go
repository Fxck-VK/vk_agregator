package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func (h *Handler) getOperatorRetentionStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance read model unavailable")
		return
	}
	now := time.Now().UTC()
	dto, err := h.operatorRetentionStatus(r.Context(), now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get retention status failed")
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) retentionOverviewCard(ctx context.Context, now time.Time) OverviewCardDTO {
	if h.deps.Maintenance == nil {
		return OverviewCardDTO{
			ID:      "retention",
			Title:   "Retention",
			Status:  overviewStatusNotWired,
			Summary: "Retention/operator read model is not wired.",
		}
	}
	status, err := h.deps.Maintenance.RetentionStatus(ctx, now)
	if err != nil {
		return OverviewCardDTO{
			ID:      "retention",
			Title:   "Retention",
			Status:  overviewStatusWarning,
			Summary: "Retention status query failed; check maintenance read model and migrations.",
			Metrics: []OverviewMetricDTO{{Label: "query", Value: "failed", Status: overviewStatusWarning}},
		}
	}
	var expired int64
	var hot int64
	for _, item := range status.Items {
		expired += item.ExpiredRows
		hot += item.TotalRows - item.DeletedRows
	}
	cardStatus := overviewStatusOK
	if expired > 0 {
		cardStatus = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "retention",
		Title:   "Retention",
		Status:  cardStatus,
		Summary: "Retention and analytics control-room endpoints are protected admin-only read models.",
		Metrics: []OverviewMetricDTO{
			{Label: "tracked rows", Value: strconv.FormatInt(hot, 10)},
			{Label: "expired rows", Value: strconv.FormatInt(expired, 10), Status: retentionMetricStatus(expired)},
		},
	}
}

func (h *Handler) getOperatorRetentionDryRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance read model unavailable")
		return
	}
	now := time.Now().UTC()
	limit := parsePositiveQueryInt(r, "limit", maxLimit)
	if limit > maxLimit {
		limit = maxLimit
	}
	dryRun, err := h.deps.Maintenance.RetentionDryRun(r.Context(), now, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get retention dry-run failed")
		return
	}
	writeJSON(w, http.StatusOK, OperatorRetentionDryRunDTO{
		GeneratedAt: dryRun.GeneratedAt,
		Items:       newOperatorRetentionDryRunItems(dryRun, now),
		Notes: []string{
			"Dry-run only: this endpoint does not delete, redact or expire data.",
			"Counts are grouped by bounded labels and may be capped by the requested limit.",
		},
	})
}

func (h *Handler) postOperatorRetentionCleanup(w http.ResponseWriter, r *http.Request) {
	if h.deps.RetentionCleanup == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance service unavailable")
		return
	}
	if err := h.deps.RetentionCleanup.Cleanup(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "retention cleanup failed")
		return
	}
	now := time.Now().UTC()
	status, err := h.operatorRetentionStatus(r.Context(), now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get retention status failed")
		return
	}
	writeJSON(w, http.StatusOK, OperatorRetentionCleanupDTO{
		GeneratedAt:     now,
		Completed:       true,
		Retention:       status.Retention,
		OldestHotRows:   status.OldestHotRows,
		OrphanArtifacts: status.OrphanArtifacts,
		Notes: []string{
			"Cleanup completed through the maintenance service and this mutation is operator-audited.",
			"Financial tables such as ledger entries, payment intents, payment events and refunds are not cleaned automatically.",
			"Raw prompts, provider payloads, storage paths, owner ids and private URLs are not exposed.",
		},
	})
}

func (h *Handler) getOperatorAnalyticsStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance read model unavailable")
		return
	}
	status, err := h.deps.Maintenance.AnalyticsAggregationStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get analytics status failed")
		return
	}
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, OperatorAnalyticsStatusDTO{
		GeneratedAt: status.GeneratedAt,
		Items:       newOperatorAnalyticsStatusItems(status, now),
		Notes: []string{
			"Analytics tables are no-PII aggregates. Dashboards must not scan raw prompts/messages as their primary source.",
		},
	})
}

func (h *Handler) getOperatorHotRows(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance read model unavailable")
		return
	}
	report, err := h.deps.Maintenance.OldestHotRows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get oldest hot rows failed")
		return
	}
	writeJSON(w, http.StatusOK, OperatorOldestHotRowsDTO{
		GeneratedAt: report.GeneratedAt,
		Items:       newOperatorOldestHotRows(report),
		Notes: []string{
			"Oldest hot rows are age signals only. Entity identifiers and content bodies are hidden.",
		},
	})
}

func (h *Handler) getOperatorOrphanArtifacts(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance read model unavailable")
		return
	}
	now := time.Now().UTC()
	report, err := h.deps.Maintenance.OrphanArtifactsCount(r.Context(), now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get orphan artifacts failed")
		return
	}
	writeJSON(w, http.StatusOK, newOperatorOrphanArtifacts(report, now))
}

func (h *Handler) operatorRetentionStatus(ctx context.Context, now time.Time) (OperatorRetentionStatusDTO, error) {
	retention, err := h.deps.Maintenance.RetentionStatus(ctx, now)
	if err != nil {
		return OperatorRetentionStatusDTO{}, err
	}
	hotRows, err := h.deps.Maintenance.OldestHotRows(ctx)
	if err != nil {
		return OperatorRetentionStatusDTO{}, err
	}
	orphan, err := h.deps.Maintenance.OrphanArtifactsCount(ctx, now)
	if err != nil {
		return OperatorRetentionStatusDTO{}, err
	}
	return OperatorRetentionStatusDTO{
		GeneratedAt:     now,
		Retention:       newOperatorRetentionTables(retention, now),
		OldestHotRows:   newOperatorOldestHotRows(hotRows),
		OrphanArtifacts: newOperatorOrphanArtifacts(orphan, now),
		Notes: []string{
			"Read-only operator view: cleanup is not executed by this endpoint.",
			"Rows expose only table/class counters and timestamps; raw prompts, payloads, ids and private storage paths are intentionally omitted.",
			"Financial tables such as ledger entries, payment intents, payment events and refunds are protected from automatic cleanup.",
		},
	}, nil
}

func newOperatorRetentionTables(status domain.RetentionStatus, now time.Time) []OperatorRetentionTableDTO {
	items := make([]OperatorRetentionTableDTO, 0, len(status.Items))
	for _, item := range status.Items {
		items = append(items, OperatorRetentionTableDTO{
			TableName:           item.TableName,
			RetentionClass:      string(item.RetentionClass),
			TotalRows:           item.TotalRows,
			ExpiredRows:         item.ExpiredRows,
			RedactedRows:        item.RedactedRows,
			DeletedRows:         item.DeletedRows,
			OldestHotAt:         item.OldestHotAt,
			OldestHotAgeSeconds: operatorAgeSeconds(now, item.OldestHotAt),
			OldestExpiredAt:     item.OldestExpiredAt,
		})
	}
	return items
}

func newOperatorRetentionDryRunItems(dryRun domain.RetentionDryRun, now time.Time) []OperatorRetentionDryRunDTOItem {
	items := make([]OperatorRetentionDryRunDTOItem, 0, len(dryRun.Items))
	for _, item := range dryRun.Items {
		items = append(items, OperatorRetentionDryRunDTOItem{
			Action:           item.Action,
			TableName:        item.TableName,
			RetentionClass:   string(item.RetentionClass),
			Count:            item.Count,
			Bytes:            item.Bytes,
			OldestAt:         item.OldestAt,
			OldestAgeSeconds: operatorAgeSeconds(now, item.OldestAt),
		})
	}
	return items
}

func newOperatorAnalyticsStatusItems(status domain.AnalyticsAggregationStatus, now time.Time) []OperatorAnalyticsStatusItemDTO {
	items := make([]OperatorAnalyticsStatusItemDTO, 0, len(status.Items))
	for _, item := range status.Items {
		items = append(items, OperatorAnalyticsStatusItemDTO{
			TableName:             item.TableName,
			Status:                item.Status,
			Rows:                  item.Rows,
			LatestActivityDate:    item.LatestActivityDate,
			LastUpdatedAt:         item.LastUpdatedAt,
			LastUpdatedAgeSeconds: operatorAgeSeconds(now, item.LastUpdatedAt),
		})
	}
	return items
}

func newOperatorOldestHotRows(report domain.OldestHotRowsReport) []OperatorOldestHotRowDTO {
	items := make([]OperatorOldestHotRowDTO, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, OperatorOldestHotRowDTO{
			TableName:      item.TableName,
			RetentionClass: string(item.RetentionClass),
			Count:          item.Count,
			OldestAt:       item.OldestAt,
			AgeSeconds:     item.AgeSeconds,
		})
	}
	return items
}

func newOperatorOrphanArtifacts(report domain.OrphanArtifactsReport, now time.Time) OperatorOrphanArtifactsDTO {
	items := make([]OperatorOrphanArtifactDTO, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, OperatorOrphanArtifactDTO{
			ArtifactTier:     string(item.ArtifactTier),
			LifecycleClass:   string(item.LifecycleClass),
			Status:           string(item.Status),
			MediaType:        string(item.MediaType),
			Count:            item.Count,
			Bytes:            item.Bytes,
			OldestAt:         item.OldestAt,
			OldestAgeSeconds: operatorAgeSeconds(now, item.OldestAt),
		})
	}
	return OperatorOrphanArtifactsDTO{
		GeneratedAt: report.GeneratedAt,
		Total:       report.Total,
		Bytes:       report.Bytes,
		Items:       items,
		Notes: []string{
			"Storage coordinates, owner ids and private URLs are not exposed.",
		},
	}
}

func operatorAgeSeconds(now time.Time, t *time.Time) int64 {
	if t == nil || now.Before(*t) {
		return 0
	}
	return int64(now.Sub(*t).Seconds())
}

func retentionMetricStatus(count int64) string {
	if count > 0 {
		return overviewStatusWarning
	}
	return overviewStatusOK
}
