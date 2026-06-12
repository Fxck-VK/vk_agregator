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

const defaultMediaCleanupLimit = 100

// Store is the database-side maintenance contract.
type Store interface {
	CleanupExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int64, error)
	CleanupOutboxEvents(ctx context.Context, cutoff time.Time) (int64, error)
	MediaCleanupCandidates(ctx context.Context, cutoff time.Time, limit int) ([]domain.MediaCleanupCandidate, error)
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
	MediaRetention                time.Duration
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

	if s.streams != nil {
		trimmed, err := s.streams.Trim(ctx)
		if err != nil {
			return err
		}
		for stream, n := range trimmed {
			metrics.StreamTrimmed.WithLabelValues(stream).Add(float64(n))
		}
	}
	mediaDeleted, err := s.cleanupMedia(ctx, now)
	if err != nil {
		return err
	}
	if err := s.ObserveProductStats(ctx); err != nil {
		s.log.WarnContext(ctx, "product stats observation failed", "error", err)
	}
	if idemDeleted > 0 || outboxDeleted > 0 || mediaDeleted > 0 {
		s.log.InfoContext(ctx, "maintenance cleanup completed",
			"idempotency_keys_deleted", idemDeleted,
			"outbox_events_deleted", outboxDeleted,
			"media_objects_deleted", mediaDeleted)
	}
	return nil
}

func (s *Service) cleanupMedia(ctx context.Context, now time.Time) (int64, error) {
	if s.cfg.MediaRetention <= 0 || s.mediaObjects == nil {
		return 0, nil
	}
	cutoff := now.Add(-s.cfg.MediaRetention)
	candidates, err := s.store.MediaCleanupCandidates(ctx, cutoff, s.cfg.MediaCleanupLimit)
	if err != nil {
		return 0, err
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
			return deleted, fmt.Errorf("maintenance: media cleanup mark deleted failed: %w", err)
		}
		deleted++
		metrics.MaintenanceDeleted.WithLabelValues("media_objects").Inc()
		metrics.ObserveMediaCleanupDeleted("success", variantType, "none")
	}
	return deleted, nil
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
