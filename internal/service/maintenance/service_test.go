package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	dto "github.com/prometheus/client_model/go"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

type fakeStore struct {
	calls           []time.Time
	mediaCutoffs    []time.Time
	mediaCandidates []domain.MediaCleanupCandidate
	markedMedia     []domain.MediaCleanupCandidate
}

func (s *fakeStore) CleanupExpiredIdempotencyKeys(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *fakeStore) CleanupOutboxEvents(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *fakeStore) MediaCleanupCandidates(_ context.Context, cutoff time.Time, _ int) ([]domain.MediaCleanupCandidate, error) {
	s.mediaCutoffs = append(s.mediaCutoffs, cutoff)
	return s.mediaCandidates, nil
}

func (s *fakeStore) MarkMediaCleanupDeleted(_ context.Context, candidate domain.MediaCleanupCandidate) error {
	s.markedMedia = append(s.markedMedia, candidate)
	return nil
}

func (s *fakeStore) ProductActiveUserCounts(_ context.Context, since time.Time) ([]domain.ProductActiveUserCount, error) {
	s.calls = append(s.calls, since)
	return []domain.ProductActiveUserCount{
		{
			Surface:   "vk_bot",
			Operation: domain.OperationTextGenerate,
			Modality:  domain.ModalityText,
			Count:     7,
		},
	}, nil
}

func (s *fakeStore) BalanceMismatches(context.Context, int) ([]domain.BalanceMismatch, error) {
	return nil, nil
}

type fakeMediaObjectStore struct {
	deleted []domain.MediaCleanupCandidate
}

func (s *fakeMediaObjectStore) DeleteObject(_ context.Context, bucket, key string) error {
	s.deleted = append(s.deleted, domain.MediaCleanupCandidate{StorageBucket: bucket, StorageKey: key})
	return nil
}

func TestObserveProductStatsUpdatesActiveUserWindows(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{}, WithClock(func() time.Time { return now }))

	if err := svc.ObserveProductStats(context.Background()); err != nil {
		t.Fatalf("ObserveProductStats() error = %v", err)
	}

	if len(store.calls) != 2 {
		t.Fatalf("ProductActiveUserCounts calls = %d, want 2", len(store.calls))
	}
	if got := now.Sub(store.calls[0]); got != 24*time.Hour {
		t.Fatalf("first window = %s, want 24h", got)
	}
	if got := now.Sub(store.calls[1]); got != 7*24*time.Hour {
		t.Fatalf("second window = %s, want 7d", got)
	}
	if got := activeUsersGaugeValue(t, "24h", "vk_bot", "text_generate", "text"); got != 7 {
		t.Fatalf("24h active users = %v, want 7", got)
	}
	if got := activeUsersGaugeValue(t, "7d", "vk_bot", "text_generate", "text"); got != 7 {
		t.Fatalf("7d active users = %v, want 7", got)
	}
}

func TestCleanupDeletesOnlyConfiguredMediaCandidates(t *testing.T) {
	candidate := domain.MediaCleanupCandidate{
		Kind:          domain.MediaCleanupVariant,
		ArtifactID:    uuid.New(),
		VariantID:     uuid.New(),
		VariantType:   domain.VariantVKVideo,
		MediaType:     domain.MediaTypeVideo,
		StorageBucket: "artifacts",
		StorageKey:    "private/key.mp4",
		SizeBytes:     4096,
	}
	store := &fakeStore{mediaCandidates: []domain.MediaCleanupCandidate{candidate}}
	objects := &fakeMediaObjectStore{}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		MediaRetention:    24 * time.Hour,
		MediaCleanupLimit: 10,
	}, WithClock(func() time.Time { return now }), WithMediaObjectStore(objects))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.mediaCutoffs) != 1 {
		t.Fatalf("MediaCleanupCandidates calls = %d, want 1", len(store.mediaCutoffs))
	}
	if got := now.Sub(store.mediaCutoffs[0]); got != 24*time.Hour {
		t.Fatalf("media cleanup cutoff = %s, want 24h", got)
	}
	if len(objects.deleted) != 1 {
		t.Fatalf("deleted objects = %d, want 1", len(objects.deleted))
	}
	if objects.deleted[0].StorageBucket != candidate.StorageBucket || objects.deleted[0].StorageKey != candidate.StorageKey {
		t.Fatalf("deleted object mismatch: %+v", objects.deleted[0])
	}
	if len(store.markedMedia) != 1 || store.markedMedia[0].VariantID != candidate.VariantID {
		t.Fatalf("marked candidates = %+v, want variant %s", store.markedMedia, candidate.VariantID)
	}
}

func activeUsersGaugeValue(t *testing.T, labels ...string) float64 {
	t.Helper()
	gauge, err := metrics.ProductActiveUsers.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := gauge.Write(&metric); err != nil {
		t.Fatalf("gauge.Write() error = %v", err)
	}
	if metric.Gauge == nil {
		t.Fatal("metric gauge is nil")
	}
	return metric.Gauge.GetValue()
}
