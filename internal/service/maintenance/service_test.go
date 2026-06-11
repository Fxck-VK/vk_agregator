package maintenance

import (
	"context"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

type fakeStore struct {
	calls []time.Time
}

func (s *fakeStore) CleanupExpiredIdempotencyKeys(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *fakeStore) CleanupOutboxEvents(context.Context, time.Time) (int64, error) {
	return 0, nil
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
