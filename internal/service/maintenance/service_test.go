package maintenance

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	dto "github.com/prometheus/client_model/go"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

type fakeStore struct {
	calls                        []time.Time
	jobErrorAggregateSince       []time.Time
	analyticsAggregateWindows    []analyticsAggregateWindow
	jobEventCutoffs              []time.Time
	providerPayloadExpireCutoffs []time.Time
	providerPayloadRedactCalls   int
	messageExpireCutoffs         []time.Time
	summaryExpireCutoffs         []time.Time
	messageRedactCalls           int
	summaryRedactCalls           int
	mediaExpirePolicies          []domain.MediaCleanupPolicy
	mediaPolicies                []domain.MediaCleanupPolicy
	mediaCandidates              []domain.MediaCleanupCandidate
	markedMedia                  []domain.MediaCleanupCandidate
	operations                   []string
	clearMediaCandidatesOnMark   bool
}

type analyticsAggregateWindow struct {
	from time.Time
	to   time.Time
}

func (s *fakeStore) CleanupExpiredIdempotencyKeys(context.Context, time.Time) (int64, error) {
	s.operations = append(s.operations, "cleanup_idempotency_keys")
	return 0, nil
}

func (s *fakeStore) CleanupOutboxEvents(context.Context, time.Time) (int64, error) {
	s.operations = append(s.operations, "cleanup_outbox_events")
	return 0, nil
}

func (s *fakeStore) AggregateJobErrors(_ context.Context, since time.Time) (int64, error) {
	s.operations = append(s.operations, "aggregate_job_errors")
	s.jobErrorAggregateSince = append(s.jobErrorAggregateSince, since)
	return 0, nil
}

func (s *fakeStore) RefreshDailyAnalyticsAggregates(_ context.Context, from, to time.Time) (int64, error) {
	s.operations = append(s.operations, "refresh_daily_analytics")
	s.analyticsAggregateWindows = append(s.analyticsAggregateWindows, analyticsAggregateWindow{from: from, to: to})
	return 0, nil
}

func (s *fakeStore) CleanupJobEvents(_ context.Context, cutoff time.Time, _ int) (int64, error) {
	s.operations = append(s.operations, "cleanup_job_events")
	s.jobEventCutoffs = append(s.jobEventCutoffs, cutoff)
	return 0, nil
}

func (s *fakeStore) ExpireProviderPayloads(_ context.Context, cutoff, _ time.Time, _ int) (int64, error) {
	s.operations = append(s.operations, "expire_provider_payloads")
	s.providerPayloadExpireCutoffs = append(s.providerPayloadExpireCutoffs, cutoff)
	return 0, nil
}

func (s *fakeStore) RedactExpiredProviderPayloads(context.Context, time.Time, int) (int64, error) {
	s.operations = append(s.operations, "redact_provider_payloads")
	s.providerPayloadRedactCalls++
	return 0, nil
}

func (s *fakeStore) ExpireConversationMessages(_ context.Context, cutoff, _ time.Time, _ int) (int64, error) {
	s.operations = append(s.operations, "expire_conversation_messages")
	s.messageExpireCutoffs = append(s.messageExpireCutoffs, cutoff)
	return 0, nil
}

func (s *fakeStore) RedactExpiredConversationMessages(context.Context, time.Time, int) (int64, error) {
	s.operations = append(s.operations, "redact_conversation_messages")
	s.messageRedactCalls++
	return 0, nil
}

func (s *fakeStore) ExpireConversationSummaries(_ context.Context, cutoff, _ time.Time, _ int) (int64, error) {
	s.operations = append(s.operations, "expire_conversation_summaries")
	s.summaryExpireCutoffs = append(s.summaryExpireCutoffs, cutoff)
	return 0, nil
}

func (s *fakeStore) RedactExpiredConversationSummaries(context.Context, time.Time, int) (int64, error) {
	s.operations = append(s.operations, "redact_conversation_summaries")
	s.summaryRedactCalls++
	return 0, nil
}

func (s *fakeStore) ExpireMediaArtifacts(_ context.Context, policy domain.MediaCleanupPolicy, _ time.Time, _ int) (int64, error) {
	s.operations = append(s.operations, "expire_media_artifacts")
	s.mediaExpirePolicies = append(s.mediaExpirePolicies, policy)
	return 1, nil
}

func (s *fakeStore) MediaCleanupCandidates(_ context.Context, policy domain.MediaCleanupPolicy, _ int) ([]domain.MediaCleanupCandidate, error) {
	s.operations = append(s.operations, "media_cleanup_candidates")
	s.mediaPolicies = append(s.mediaPolicies, policy)
	return s.mediaCandidates, nil
}

func (s *fakeStore) MarkMediaCleanupDeleted(_ context.Context, candidate domain.MediaCleanupCandidate) error {
	s.operations = append(s.operations, "mark_media_deleted")
	s.markedMedia = append(s.markedMedia, candidate)
	if s.clearMediaCandidatesOnMark {
		s.mediaCandidates = nil
	}
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
	deleted    []domain.MediaCleanupCandidate
	operations *[]string
}

func (s *fakeMediaObjectStore) DeleteObject(_ context.Context, bucket, key string) error {
	if s.operations != nil {
		*s.operations = append(*s.operations, "delete_object")
	}
	s.deleted = append(s.deleted, domain.MediaCleanupCandidate{StorageBucket: bucket, StorageKey: key})
	return nil
}

func TestMaintenanceStoreContractHasNoFinancialCleanupMethods(t *testing.T) {
	storeType := reflect.TypeOf((*Store)(nil)).Elem()
	for i := 0; i < storeType.NumMethod(); i++ {
		name := storeType.Method(i).Name
		lower := strings.ToLower(name)
		isCleanupMutation := strings.HasPrefix(lower, "cleanup") ||
			strings.HasPrefix(lower, "expire") ||
			strings.HasPrefix(lower, "redact") ||
			strings.HasPrefix(lower, "mark") ||
			strings.Contains(lower, "delete")
		isFinancial := strings.Contains(lower, "ledger") ||
			strings.Contains(lower, "payment") ||
			strings.Contains(lower, "refund") ||
			strings.Contains(lower, "credit") ||
			strings.Contains(lower, "balance")
		if isCleanupMutation && isFinancial {
			t.Fatalf("maintenance store exposes protected financial cleanup method %q", name)
		}
	}
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
	objects := &fakeMediaObjectStore{operations: &store.operations}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		MediaFailedRetention: 24 * time.Hour,
		MediaCleanupLimit:    10,
	}, WithClock(func() time.Time { return now }), WithMediaObjectStore(objects))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.mediaPolicies) != 1 {
		t.Fatalf("MediaCleanupCandidates calls = %d, want 1", len(store.mediaPolicies))
	}
	if len(store.mediaExpirePolicies) != 1 {
		t.Fatalf("ExpireMediaArtifacts calls = %d, want 1", len(store.mediaExpirePolicies))
	}
	if store.mediaPolicies[0].ExpiredCutoff != now {
		t.Fatalf("expired cutoff = %s, want %s", store.mediaPolicies[0].ExpiredCutoff, now)
	}
	if got := now.Sub(store.mediaPolicies[0].FailedDeletedCutoff); got != 24*time.Hour {
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
	expireIndex := operationIndex(store.operations, "expire_media_artifacts")
	deleteIndex := operationIndex(store.operations, "delete_object")
	if expireIndex == -1 || deleteIndex == -1 || expireIndex > deleteIndex {
		t.Fatalf("operation order = %v, want DB expire mark before object delete", store.operations)
	}
}

func TestCleanupSkipsMediaWhenRetentionDisabled(t *testing.T) {
	store := &fakeStore{}
	objects := &fakeMediaObjectStore{}
	svc := New(store, nil, Config{}, WithMediaObjectStore(objects))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.mediaPolicies) != 0 {
		t.Fatalf("MediaCleanupCandidates calls = %d, want 0", len(store.mediaPolicies))
	}
	if len(objects.deleted) != 0 {
		t.Fatalf("deleted objects = %d, want 0", len(objects.deleted))
	}
}

func TestCleanupBuildsArtifactLifecyclePolicy(t *testing.T) {
	store := &fakeStore{}
	objects := &fakeMediaObjectStore{}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		MediaFreeRetention:       7 * 24 * time.Hour,
		MediaPaidRetention:       365 * 24 * time.Hour,
		MediaTempUploadRetention: 24 * time.Hour,
		MediaOrphanRetention:     3 * 24 * time.Hour,
		MediaCleanupLimit:        10,
	}, WithClock(func() time.Time { return now }), WithMediaObjectStore(objects))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.mediaExpirePolicies) != 1 {
		t.Fatalf("ExpireMediaArtifacts calls = %d, want 1", len(store.mediaExpirePolicies))
	}
	policy := store.mediaExpirePolicies[0]
	if got := now.Sub(policy.FreeArtifactCutoff); got != 7*24*time.Hour {
		t.Fatalf("free retention window = %s, want 168h", got)
	}
	if got := now.Sub(policy.PaidArtifactCutoff); got != 365*24*time.Hour {
		t.Fatalf("paid retention window = %s, want 8760h", got)
	}
	if got := now.Sub(policy.TemporaryArtifactCutoff); got != 24*time.Hour {
		t.Fatalf("temporary retention window = %s, want 24h", got)
	}
	if got := now.Sub(policy.OrphanArtifactCutoff); got != 3*24*time.Hour {
		t.Fatalf("orphan retention window = %s, want 72h", got)
	}
}

func TestCleanupAppliesConversationRetentionWindows(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		ConversationMessageRetention: 14 * 24 * time.Hour,
		ConversationSummaryRetention: 60 * 24 * time.Hour,
		ConversationRetentionLimit:   25,
	}, WithClock(func() time.Time { return now }))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.messageExpireCutoffs) != 1 {
		t.Fatalf("message retention calls = %d, want 1", len(store.messageExpireCutoffs))
	}
	if got := now.Sub(store.messageExpireCutoffs[0]); got != 14*24*time.Hour {
		t.Fatalf("message retention window = %s, want 336h", got)
	}
	if store.messageRedactCalls != 1 {
		t.Fatalf("message redact calls = %d, want 1", store.messageRedactCalls)
	}
	if len(store.summaryExpireCutoffs) != 1 {
		t.Fatalf("summary retention calls = %d, want 1", len(store.summaryExpireCutoffs))
	}
	if got := now.Sub(store.summaryExpireCutoffs[0]); got != 60*24*time.Hour {
		t.Fatalf("summary retention window = %s, want 1440h", got)
	}
	if store.summaryRedactCalls != 1 {
		t.Fatalf("summary redact calls = %d, want 1", store.summaryRedactCalls)
	}
}

func TestCleanupAppliesJobLogAndProviderPayloadRetention(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		JobEventsRetention:        14 * 24 * time.Hour,
		ProviderPayloadRetention:  3 * 24 * time.Hour,
		JobLogRetentionLimit:      25,
		JobErrorAggregateLookback: 10 * 24 * time.Hour,
	}, WithClock(func() time.Time { return now }))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.jobErrorAggregateSince) != 1 {
		t.Fatalf("AggregateJobErrors calls = %d, want 1", len(store.jobErrorAggregateSince))
	}
	if got := now.Sub(store.jobErrorAggregateSince[0]); got != 10*24*time.Hour {
		t.Fatalf("job error aggregate lookback = %s, want 240h", got)
	}
	if len(store.jobEventCutoffs) != 1 {
		t.Fatalf("CleanupJobEvents calls = %d, want 1", len(store.jobEventCutoffs))
	}
	if got := now.Sub(store.jobEventCutoffs[0]); got != 14*24*time.Hour {
		t.Fatalf("job event retention window = %s, want 336h", got)
	}
	if len(store.providerPayloadExpireCutoffs) != 1 {
		t.Fatalf("ExpireProviderPayloads calls = %d, want 1", len(store.providerPayloadExpireCutoffs))
	}
	if got := now.Sub(store.providerPayloadExpireCutoffs[0]); got != 3*24*time.Hour {
		t.Fatalf("provider payload retention window = %s, want 72h", got)
	}
	if store.providerPayloadRedactCalls != 1 {
		t.Fatalf("provider payload redact calls = %d, want 1", store.providerPayloadRedactCalls)
	}
}

func TestCleanupRefreshesDailyAnalyticsAggregates(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 11, 15, 30, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		AnalyticsAggregateLookback: 3 * 24 * time.Hour,
	}, WithClock(func() time.Time { return now }))

	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if len(store.analyticsAggregateWindows) != 1 {
		t.Fatalf("RefreshDailyAnalyticsAggregates calls = %d, want 1", len(store.analyticsAggregateWindows))
	}
	got := store.analyticsAggregateWindows[0]
	wantFrom := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	if !got.from.Equal(wantFrom) || !got.to.Equal(wantTo) {
		t.Fatalf("analytics window = [%s, %s), want [%s, %s)", got.from, got.to, wantFrom, wantTo)
	}
}

func TestCleanupRefreshesDailyAnalyticsWithStableWindowOnRepeat(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 11, 15, 30, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		AnalyticsAggregateLookback: 3 * 24 * time.Hour,
	}, WithClock(func() time.Time { return now }))

	for i := 0; i < 2; i++ {
		if err := svc.Cleanup(context.Background()); err != nil {
			t.Fatalf("Cleanup() run %d error = %v", i+1, err)
		}
	}

	if len(store.analyticsAggregateWindows) != 2 {
		t.Fatalf("RefreshDailyAnalyticsAggregates calls = %d, want 2", len(store.analyticsAggregateWindows))
	}
	if store.analyticsAggregateWindows[0] != store.analyticsAggregateWindows[1] {
		t.Fatalf("analytics windows differ on repeated run: %+v", store.analyticsAggregateWindows)
	}
}

func TestCleanupRepeatedRunDoesNotReDeleteMarkedMediaCandidate(t *testing.T) {
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
	store := &fakeStore{
		mediaCandidates:            []domain.MediaCleanupCandidate{candidate},
		clearMediaCandidatesOnMark: true,
	}
	objects := &fakeMediaObjectStore{operations: &store.operations}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, Config{
		MediaFailedRetention: 24 * time.Hour,
		MediaCleanupLimit:    10,
	}, WithClock(func() time.Time { return now }), WithMediaObjectStore(objects))

	for i := 0; i < 2; i++ {
		if err := svc.Cleanup(context.Background()); err != nil {
			t.Fatalf("Cleanup() run %d error = %v", i+1, err)
		}
	}

	if len(objects.deleted) != 1 {
		t.Fatalf("deleted objects = %d, want 1", len(objects.deleted))
	}
	if len(store.markedMedia) != 1 {
		t.Fatalf("marked media = %d, want 1", len(store.markedMedia))
	}
}

func operationIndex(operations []string, name string) int {
	for i, op := range operations {
		if op == name {
			return i
		}
	}
	return -1
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
