// Package maintenance runs operational cleanup and consistency checks.
package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

const (
	defaultMediaCleanupLimit          = 100
	defaultConversationCleanupLimit   = 500
	defaultConversationMessageTTL     = 90 * 24 * time.Hour
	defaultConversationSummaryTTL     = 180 * 24 * time.Hour
	defaultJobLogCleanupLimit         = 500
	defaultJobEventsTTL               = 30 * 24 * time.Hour
	defaultProviderPayloadTTL         = 7 * 24 * time.Hour
	defaultJobErrorAggregateLookback  = 30 * 24 * time.Hour
	defaultAnalyticsAggregateLookback = 7 * 24 * time.Hour
)

// Store is the database-side maintenance contract.
type Store interface {
	CleanupExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int64, error)
	CleanupOutboxEvents(ctx context.Context, cutoff time.Time) (int64, error)
	AggregateJobErrors(ctx context.Context, since time.Time) (int64, error)
	RefreshDailyAnalyticsAggregates(ctx context.Context, from, to time.Time) (int64, error)
	CleanupJobEvents(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	ExpireProviderPayloads(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error)
	RedactExpiredProviderPayloads(ctx context.Context, now time.Time, limit int) (int64, error)
	ExpireConversationMessages(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error)
	RedactExpiredConversationMessages(ctx context.Context, now time.Time, limit int) (int64, error)
	ExpireConversationSummaries(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error)
	RedactExpiredConversationSummaries(ctx context.Context, now time.Time, limit int) (int64, error)
	ExpireMediaArtifacts(ctx context.Context, policy domain.MediaCleanupPolicy, expiresAt time.Time, limit int) (int64, error)
	MediaCleanupCandidates(ctx context.Context, policy domain.MediaCleanupPolicy, limit int) ([]domain.MediaCleanupCandidate, error)
	MarkMediaCleanupDeleted(ctx context.Context, candidate domain.MediaCleanupCandidate) error
	ProductActiveUserCounts(ctx context.Context, since time.Time) ([]domain.ProductActiveUserCount, error)
	BalanceMismatches(ctx context.Context, limit int) ([]domain.BalanceMismatch, error)
}

// StreamTrimmer is the Redis-side maintenance contract.
type StreamTrimmer interface {
	Trim(ctx context.Context) (map[string]int64, error)
}

// MediaObjectStore is the object-storage side of inactive media cleanup.
type MediaObjectStore interface {
	DeleteObject(ctx context.Context, bucket, key string) error
}

// Config controls operational maintenance jobs.
type Config struct {
	Interval                      time.Duration
	OutboxRetention               time.Duration
	BillingReconciliationInterval time.Duration
	BillingReconciliationLimit    int
	JobEventsRetention            time.Duration
	ProviderPayloadRetention      time.Duration
	JobLogRetentionLimit          int
	JobErrorAggregateLookback     time.Duration
	AnalyticsAggregateLookback    time.Duration
	ConversationMessageRetention  time.Duration
	ConversationSummaryRetention  time.Duration
	ConversationRetentionLimit    int
	MediaFreeRetention            time.Duration
	MediaPaidRetention            time.Duration
	MediaOrphanRetention          time.Duration
	MediaTempUploadRetention      time.Duration
	MediaInputRetention           time.Duration
	MediaFailedRetention          time.Duration
	MediaOriginalRetention        time.Duration
	MediaVariantRetention         time.Duration
	MediaCleanupLimit             int
}

// Service runs cleanup and consistency jobs. It never mutates balances during
// reconciliation; it only emits metrics/logs so operators can investigate.
type Service struct {
	store        Store
	streams      StreamTrimmer
	mediaObjects MediaObjectStore
	cfg          Config
	log          *slog.Logger
	now          func() time.Time
}

// Option customizes Service.
type Option func(*Service)

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(s *Service) {
		if l != nil {
			s.log = l
		}
	}
}

// WithClock overrides the clock for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithMediaObjectStore enables inactive media-object cleanup.
func WithMediaObjectStore(store MediaObjectStore) Option {
	return func(s *Service) {
		s.mediaObjects = store
	}
}

// New builds a maintenance service.
func New(store Store, streams StreamTrimmer, cfg Config, opts ...Option) *Service {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Hour
	}
	if cfg.OutboxRetention <= 0 {
		cfg.OutboxRetention = 7 * 24 * time.Hour
	}
	if cfg.BillingReconciliationInterval <= 0 {
		cfg.BillingReconciliationInterval = 5 * time.Minute
	}
	if cfg.BillingReconciliationLimit <= 0 {
		cfg.BillingReconciliationLimit = 100
	}
	if cfg.JobEventsRetention <= 0 {
		cfg.JobEventsRetention = defaultJobEventsTTL
	}
	if cfg.ProviderPayloadRetention <= 0 {
		cfg.ProviderPayloadRetention = defaultProviderPayloadTTL
	}
	if cfg.JobLogRetentionLimit <= 0 {
		cfg.JobLogRetentionLimit = defaultJobLogCleanupLimit
	}
	if cfg.JobErrorAggregateLookback <= 0 {
		cfg.JobErrorAggregateLookback = defaultJobErrorAggregateLookback
	}
	if cfg.AnalyticsAggregateLookback <= 0 {
		cfg.AnalyticsAggregateLookback = defaultAnalyticsAggregateLookback
	}
	if cfg.ConversationMessageRetention <= 0 {
		cfg.ConversationMessageRetention = defaultConversationMessageTTL
	}
	if cfg.ConversationSummaryRetention <= 0 {
		cfg.ConversationSummaryRetention = defaultConversationSummaryTTL
	}
	if cfg.ConversationRetentionLimit <= 0 {
		cfg.ConversationRetentionLimit = defaultConversationCleanupLimit
	}
	if cfg.MediaCleanupLimit <= 0 {
		cfg.MediaCleanupLimit = defaultMediaCleanupLimit
	}
	s := &Service{
		store:   store,
		streams: streams,
		cfg:     cfg,
		log:     slog.Default(),
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run starts cleanup and reconciliation loops until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	cleanupTicker := time.NewTicker(s.cfg.Interval)
	reconcileTicker := time.NewTicker(s.cfg.BillingReconciliationInterval)
	defer cleanupTicker.Stop()
	defer reconcileTicker.Stop()

	s.runCleanup(ctx)
	s.runReconciliation(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanupTicker.C:
			s.runCleanup(ctx)
		case <-reconcileTicker.C:
			s.runReconciliation(ctx)
		}
	}
}

func (s *Service) runCleanup(ctx context.Context) {
	if err := s.Cleanup(ctx); err != nil {
		s.log.WarnContext(ctx, "maintenance cleanup failed", "error", err)
	}
}

func (s *Service) runReconciliation(ctx context.Context) {
	if err := s.ReconcileBilling(ctx); err != nil {
		s.log.WarnContext(ctx, "billing reconciliation failed", "error", err)
	}
}

// Cleanup deletes expired idempotency keys, terminal outbox rows past retention
// and trims Redis stream backlog.
func (s *Service) Cleanup(ctx context.Context) error {
	now := s.now()
	idemDeleted, err := s.store.CleanupExpiredIdempotencyKeys(ctx, now)
	if err != nil {
		return err
	}
	outboxDeleted, err := s.store.CleanupOutboxEvents(ctx, now.Add(-s.cfg.OutboxRetention))
	if err != nil {
		return err
	}
	metrics.MaintenanceDeleted.WithLabelValues("idempotency_keys").Add(float64(idemDeleted))
	metrics.MaintenanceDeleted.WithLabelValues("outbox_events").Add(float64(outboxDeleted))

	jobErrorsAggregated, jobEventsDeleted, providerPayloadsExpired, providerPayloadsRedacted, err := s.cleanupJobLogs(ctx, now)
	if err != nil {
		return err
	}
	metrics.MaintenanceDeleted.WithLabelValues("job_events").Add(float64(jobEventsDeleted))
	metrics.MaintenanceDeleted.WithLabelValues("provider_payloads_expired").Add(float64(providerPayloadsExpired))
	metrics.MaintenanceDeleted.WithLabelValues("provider_payloads_redacted").Add(float64(providerPayloadsRedacted))

	dailyAnalyticsRefreshed, err := s.refreshDailyAnalytics(ctx, now)
	if err != nil {
		return err
	}

	conversationExpired, conversationRedacted, summaryExpired, summaryRedacted, err := s.cleanupConversations(ctx, now)
	if err != nil {
		return err
	}
	metrics.MaintenanceDeleted.WithLabelValues("conversation_messages_expired").Add(float64(conversationExpired))
	metrics.MaintenanceDeleted.WithLabelValues("conversation_messages_redacted").Add(float64(conversationRedacted))
	metrics.MaintenanceDeleted.WithLabelValues("conversation_summaries_expired").Add(float64(summaryExpired))
	metrics.MaintenanceDeleted.WithLabelValues("conversation_summaries_redacted").Add(float64(summaryRedacted))

	if s.streams != nil {
		trimmed, err := s.streams.Trim(ctx)
		if err != nil {
			return err
		}
		for stream, n := range trimmed {
			metrics.StreamTrimmed.WithLabelValues(stream).Add(float64(n))
		}
	}
	mediaExpired, mediaDeleted, err := s.cleanupMedia(ctx, now)
	if err != nil {
		return err
	}
	if err := s.ObserveProductStats(ctx); err != nil {
		s.log.WarnContext(ctx, "product stats observation failed", "error", err)
	}
	if idemDeleted > 0 || outboxDeleted > 0 || mediaExpired > 0 || mediaDeleted > 0 ||
		jobErrorsAggregated > 0 || dailyAnalyticsRefreshed > 0 || jobEventsDeleted > 0 ||
		providerPayloadsExpired > 0 || providerPayloadsRedacted > 0 ||
		conversationExpired > 0 || conversationRedacted > 0 ||
		summaryExpired > 0 || summaryRedacted > 0 {
		s.log.InfoContext(ctx, "maintenance cleanup completed",
			"idempotency_keys_deleted", idemDeleted,
			"outbox_events_deleted", outboxDeleted,
			"job_errors_aggregated", jobErrorsAggregated,
			"daily_analytics_refreshed", dailyAnalyticsRefreshed,
			"job_events_deleted", jobEventsDeleted,
			"provider_payloads_expired", providerPayloadsExpired,
			"provider_payloads_redacted", providerPayloadsRedacted,
			"conversation_messages_expired", conversationExpired,
			"conversation_messages_redacted", conversationRedacted,
			"conversation_summaries_expired", summaryExpired,
			"conversation_summaries_redacted", summaryRedacted,
			"media_artifacts_expired", mediaExpired,
			"media_objects_deleted", mediaDeleted)
	}
	return nil
}

func (s *Service) refreshDailyAnalytics(ctx context.Context, now time.Time) (int64, error) {
	if s.cfg.AnalyticsAggregateLookback <= 0 {
		return 0, nil
	}
	from := analyticsDay(now.Add(-s.cfg.AnalyticsAggregateLookback))
	to := analyticsDay(now.Add(24 * time.Hour))
	return s.store.RefreshDailyAnalyticsAggregates(ctx, from, to)
}

func analyticsDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (s *Service) cleanupJobLogs(ctx context.Context, now time.Time) (int64, int64, int64, int64, error) {
	errorsAggregated, err := s.store.AggregateJobErrors(ctx, now.Add(-s.cfg.JobErrorAggregateLookback))
	if err != nil {
		return 0, 0, 0, 0, err
	}

	eventsDeleted, err := s.store.CleanupJobEvents(ctx, now.Add(-s.cfg.JobEventsRetention), s.cfg.JobLogRetentionLimit)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	payloadsExpired, err := s.store.ExpireProviderPayloads(ctx, now.Add(-s.cfg.ProviderPayloadRetention), now, s.cfg.JobLogRetentionLimit)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	payloadsRedacted, err := s.store.RedactExpiredProviderPayloads(ctx, now, s.cfg.JobLogRetentionLimit)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return errorsAggregated, eventsDeleted, payloadsExpired, payloadsRedacted, nil
}

func (s *Service) cleanupConversations(ctx context.Context, now time.Time) (int64, int64, int64, int64, error) {
	var messagesExpired, messagesRedacted, summariesExpired, summariesRedacted int64
	limit := s.cfg.ConversationRetentionLimit

	if s.cfg.ConversationMessageRetention > 0 {
		expired, err := s.store.ExpireConversationMessages(ctx, now.Add(-s.cfg.ConversationMessageRetention), now, limit)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		redacted, err := s.store.RedactExpiredConversationMessages(ctx, now, limit)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		messagesExpired = expired
		messagesRedacted = redacted
	}

	if s.cfg.ConversationSummaryRetention > 0 {
		expired, err := s.store.ExpireConversationSummaries(ctx, now.Add(-s.cfg.ConversationSummaryRetention), now, limit)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		redacted, err := s.store.RedactExpiredConversationSummaries(ctx, now, limit)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		summariesExpired = expired
		summariesRedacted = redacted
	}

	return messagesExpired, messagesRedacted, summariesExpired, summariesRedacted, nil
}

func (s *Service) cleanupMedia(ctx context.Context, now time.Time) (int64, int64, error) {
	if s.mediaObjects == nil {
		return 0, 0, nil
	}
	policy := s.mediaCleanupPolicy(now)
	if !policy.Enabled() {
		return 0, 0, nil
	}
	expired, err := s.store.ExpireMediaArtifacts(ctx, policy, now, s.cfg.MediaCleanupLimit)
	if err != nil {
		return 0, 0, err
	}
	if expired > 0 {
		metrics.MaintenanceDeleted.WithLabelValues("media_artifacts_expired").Add(float64(expired))
	}
	policy.ExpiredCutoff = now
	candidates, err := s.store.MediaCleanupCandidates(ctx, policy, s.cfg.MediaCleanupLimit)
	if err != nil {
		return expired, 0, err
	}

	var deleted int64
	for _, candidate := range candidates {
		if candidate.StorageBucket == "" || candidate.StorageKey == "" {
			continue
		}
		variantType := mediaCleanupVariantType(candidate)
		modality := metrics.ProductLabel(string(candidate.MediaType), "unknown")
		metrics.ObserveMediaBytes("cleanup", modality, variantType, candidate.SizeBytes)

		if err := s.mediaObjects.DeleteObject(ctx, candidate.StorageBucket, candidate.StorageKey); err != nil {
			metrics.ObserveMediaCleanupDeleted("error", variantType, "object_delete_failed")
			s.log.WarnContext(ctx, "media cleanup object delete failed",
				"kind", metrics.ProductLabel(string(candidate.Kind), "unknown"),
				"variant_type", variantType,
				"error_class", "object_delete_failed",
				"error", err)
			continue
		}
		if err := s.store.MarkMediaCleanupDeleted(ctx, candidate); err != nil {
			metrics.ObserveMediaCleanupDeleted("error", variantType, "db_mark_failed")
			return expired, deleted, fmt.Errorf("maintenance: media cleanup mark deleted failed: %w", err)
		}
		deleted++
		metrics.MaintenanceDeleted.WithLabelValues("media_objects").Inc()
		metrics.ObserveMediaCleanupDeleted("success", variantType, "none")
	}
	return expired, deleted, nil
}

func (s *Service) mediaCleanupPolicy(now time.Time) domain.MediaCleanupPolicy {
	var policy domain.MediaCleanupPolicy
	if s.cfg.MediaFreeRetention > 0 {
		policy.FreeArtifactCutoff = now.Add(-s.cfg.MediaFreeRetention)
	}
	if s.cfg.MediaPaidRetention > 0 {
		policy.PaidArtifactCutoff = now.Add(-s.cfg.MediaPaidRetention)
	}
	if s.cfg.MediaOrphanRetention > 0 {
		policy.OrphanArtifactCutoff = now.Add(-s.cfg.MediaOrphanRetention)
	}
	if s.cfg.MediaTempUploadRetention > 0 {
		policy.TempUploadCutoff = now.Add(-s.cfg.MediaTempUploadRetention)
		policy.TemporaryArtifactCutoff = policy.TempUploadCutoff
	}
	if s.cfg.MediaInputRetention > 0 {
		policy.InputReferenceCutoff = now.Add(-s.cfg.MediaInputRetention)
	}
	if s.cfg.MediaOriginalRetention > 0 {
		policy.ProviderOriginalCutoff = now.Add(-s.cfg.MediaOriginalRetention)
	}
	if s.cfg.MediaVariantRetention > 0 {
		policy.DeliveryVariantCutoff = now.Add(-s.cfg.MediaVariantRetention)
	}
	if s.cfg.MediaFailedRetention > 0 {
		policy.FailedDeletedCutoff = now.Add(-s.cfg.MediaFailedRetention)
	}
	return policy
}

func mediaCleanupVariantType(candidate domain.MediaCleanupCandidate) string {
	if candidate.Kind == domain.MediaCleanupOriginal {
		return string(domain.VariantOriginal)
	}
	if candidate.VariantType != "" {
		return string(candidate.VariantType)
	}
	return "unknown"
}

// ObserveProductStats updates low-cardinality product aggregate gauges.
func (s *Service) ObserveProductStats(ctx context.Context) error {
	now := s.now()
	windows := []struct {
		label    string
		duration time.Duration
	}{
		{label: "24h", duration: 24 * time.Hour},
		{label: "7d", duration: 7 * 24 * time.Hour},
	}
	metrics.ProductActiveUsers.Reset()
	for _, window := range windows {
		counts, err := s.store.ProductActiveUserCounts(ctx, now.Add(-window.duration))
		if err != nil {
			return err
		}
		for _, item := range counts {
			metrics.ProductActiveUsers.WithLabelValues(
				window.label,
				metrics.ProductLabel(item.Surface, "unknown"),
				metrics.ProductLabel(string(item.Operation), "unknown"),
				metrics.ProductLabel(string(item.Modality), "unknown"),
			).Set(float64(item.Count))
		}
	}
	return nil
}

// ReconcileBilling compares cached balances with the committed ledger
// projection and emits the mismatch metric.
func (s *Service) ReconcileBilling(ctx context.Context) error {
	mismatches, err := s.store.BalanceMismatches(ctx, s.cfg.BillingReconciliationLimit)
	if err != nil {
		return err
	}
	metrics.BillingMismatches.Set(float64(len(mismatches)))
	for _, m := range mismatches {
		s.log.ErrorContext(ctx, "billing balance mismatch",
			"account_id", m.AccountID,
			"user_id", m.UserID,
			"currency", m.Currency,
			"balance_cached", m.BalanceCached,
			"ledger_balance", m.LedgerBalance,
			"difference", m.Difference)
	}
	return nil
}
