package outboxrelay

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/uow"
)

type trackingUOW struct {
	repos  uow.Repositories
	mu     sync.Mutex
	active bool
	calls  int
}

func (u *trackingUOW) Within(ctx context.Context, fn func(context.Context, uow.Repositories) error) error {
	u.mu.Lock()
	u.active = true
	u.calls++
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.active = false
		u.mu.Unlock()
	}()
	return fn(ctx, u.repos)
}

func (u *trackingUOW) isActive() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.active
}

func (u *trackingUOW) callCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls
}

type recordingPublisher struct {
	inner        *queue.MemoryPublisher
	beforeCall   func()
	enqueueErrs  []error
	publishErrs  []error
	enqueueCalls int
	publishCalls int
}

func newRecordingPublisher() *recordingPublisher {
	return &recordingPublisher{inner: queue.NewMemoryPublisher()}
}

func (p *recordingPublisher) Enqueue(ctx context.Context, task queue.Task) error {
	if p.beforeCall != nil {
		p.beforeCall()
	}
	p.enqueueCalls++
	if len(p.enqueueErrs) > 0 {
		err := p.enqueueErrs[0]
		p.enqueueErrs = p.enqueueErrs[1:]
		if err != nil {
			return err
		}
	}
	return p.inner.Enqueue(ctx, task)
}

func (p *recordingPublisher) PublishTo(ctx context.Context, stream string, task queue.Task) error {
	if p.beforeCall != nil {
		p.beforeCall()
	}
	p.publishCalls++
	if len(p.publishErrs) > 0 {
		err := p.publishErrs[0]
		p.publishErrs = p.publishErrs[1:]
		if err != nil {
			return err
		}
	}
	return p.inner.PublishTo(ctx, stream, task)
}

type failOnceMarkClaimedOutbox struct {
	domain.OutboxRepository
	err    error
	failed bool
}

func (r *failOnceMarkClaimedOutbox) MarkPublishedClaimed(
	ctx context.Context,
	id uuid.UUID,
	claimToken uuid.UUID,
	publishedAt time.Time,
) (bool, error) {
	if !r.failed {
		r.failed = true
		return false, r.err
	}
	return r.OutboxRepository.MarkPublishedClaimed(ctx, id, claimToken, publishedAt)
}

func TestDrainPublishHappensOutsideClaimTransaction(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := &trackingUOW{repos: uow.Repositories{Outbox: outbox}}
	publisher := newRecordingPublisher()
	publisher.beforeCall = func() {
		if manager.isActive() {
			t.Fatal("publisher called while unit of work is active")
		}
	}
	relay := testRelay(manager, publisher, &now)
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, sampleTask())); err != nil {
		t.Fatalf("add event: %v", err)
	}
	publishSamplesBefore := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success")

	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("Drain() = (%d, %v), want (1, nil)", got, err)
	}
	if got := manager.callCount(); got != 2 {
		t.Fatalf("unit-of-work calls = %d, want one claim and one resolve", got)
	}
	if got := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success"); got != publishSamplesBefore+1 {
		t.Fatalf("publish duration samples = %d, want %d", got, publishSamplesBefore+1)
	}
}

func TestDrainPublishDurationIncludesEarlierSameBatchPublication(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	const firstPublishDelay = 200 * time.Millisecond
	var enqueueCalls int
	publisher.beforeCall = func() {
		enqueueCalls++
		if enqueueCalls == 1 {
			time.Sleep(firstPublishDelay)
		}
	}
	relay := testRelay(manager, publisher, &now, WithBatchSize(2))
	for range 2 {
		if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, sampleTask())); err != nil {
			t.Fatalf("add event: %v", err)
		}
	}
	publishCountBefore := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success")
	publishSumBefore := relayHistogramSum(t, metrics.OutboxRelayPublishDuration, "queued", "success")

	if got, err := relay.Drain(ctx); err != nil || got != 2 {
		t.Fatalf("Drain() = (%d, %v), want (2, nil)", got, err)
	}

	if got := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success"); got != publishCountBefore+2 {
		t.Fatalf("publish duration samples = %d, want %d", got, publishCountBefore+2)
	}
	if got := relayHistogramSum(t, metrics.OutboxRelayPublishDuration, "queued", "success") - publishSumBefore; got < (3*firstPublishDelay)/2 {
		t.Fatalf("claim-to-acknowledged publish duration sum = %s, want at least %s so the second event includes earlier same-batch publication", got, (3*firstPublishDelay)/2)
	}
}

func TestDrainDoesNotRecordAcknowledgedPublishDurationForFailedQueueCall(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	publisher.enqueueErrs = []error{errors.New("redis unavailable")}
	relay := testRelay(manager, publisher, &now)
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, sampleTask())); err != nil {
		t.Fatalf("add event: %v", err)
	}
	acknowledgedBefore := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success")

	if got, err := relay.Drain(ctx); err == nil || got != 0 {
		t.Fatalf("Drain() = (%d, %v), want (0, publication error)", got, err)
	}
	if got := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "queued", "success"); got != acknowledgedBefore {
		t.Fatalf("acknowledged publish duration samples = %d, want %d after failed queue call", got, acknowledgedBefore)
	}
}

func TestDrainDoesNotRecordAcknowledgedPublishDurationForAuditOnlyEvent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	relay := testRelay(manager, newRecordingPublisher(), &now)
	if err := outbox.Add(ctx, taskEvent(t, EventJobCreated, sampleTask())); err != nil {
		t.Fatalf("add event: %v", err)
	}
	acknowledgedBefore := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "created", "success")

	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("Drain() = (%d, %v), want (1, nil)", got, err)
	}
	if got := relayHistogramCount(t, metrics.OutboxRelayPublishDuration, "created", "success"); got != acknowledgedBefore {
		t.Fatalf("acknowledged publish duration samples = %d, want %d for audit-only event", got, acknowledgedBefore)
	}
}

func TestDrainCompetingRelaysLeaseDistinctEvents(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	sharedPublisher := queue.NewMemoryPublisher()
	firstPublisher := &recordingPublisher{inner: sharedPublisher}
	secondPublisher := &recordingPublisher{inner: sharedPublisher}
	publishStarted := make(chan struct{})
	releasePublish := make(chan struct{})
	var publishBarrier sync.Once
	firstPublisher.beforeCall = func() {
		publishBarrier.Do(func() { close(publishStarted) })
		<-releasePublish
	}
	first := testRelay(manager, firstPublisher, &now, WithOwner("relay-a"), WithBatchSize(1))
	second := testRelay(manager, secondPublisher, &now, WithOwner("relay-b"), WithBatchSize(1))
	firstTask := sampleTask()
	secondTask := sampleTask()
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, firstTask)); err != nil {
		t.Fatalf("add first event: %v", err)
	}
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, secondTask)); err != nil {
		t.Fatalf("add second event: %v", err)
	}

	type drainResult struct {
		count int
		err   error
	}
	firstResult := make(chan drainResult, 1)
	go func() {
		count, err := first.Drain(ctx)
		firstResult <- drainResult{count: count, err: err}
	}()

	select {
	case <-publishStarted:
	case <-time.After(2 * time.Second):
		close(releasePublish)
		t.Fatal("first relay did not reach publisher after claiming")
	}
	processing := 0
	pending := 0
	for _, event := range outbox.Events() {
		switch event.Status {
		case domain.OutboxStatusProcessing:
			processing++
			if event.ClaimOwner != "relay-a" {
				close(releasePublish)
				t.Fatalf("in-flight claim owner = %q, want relay-a", event.ClaimOwner)
			}
		case domain.OutboxPending:
			pending++
		}
	}
	if processing != 1 || pending != 1 {
		close(releasePublish)
		t.Fatalf("overlap state = processing:%d pending:%d, want 1 and 1", processing, pending)
	}

	if got, err := second.Drain(ctx); err != nil || got != 1 {
		close(releasePublish)
		t.Fatalf("second Drain() = (%d, %v), want (1, nil)", got, err)
	}
	close(releasePublish)
	select {
	case result := <-firstResult:
		if result.err != nil || result.count != 1 {
			t.Fatalf("first Drain() = (%d, %v), want (1, nil)", result.count, result.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first relay did not finish after publisher release")
	}

	tasks := sharedPublisher.Tasks(firstTask.QueueName())
	if len(tasks) != 2 {
		t.Fatalf("published tasks = %+v, want exactly two", tasks)
	}
	taskIDs := map[uuid.UUID]int{}
	for _, task := range tasks {
		taskIDs[task.JobID]++
	}
	if taskIDs[firstTask.JobID] != 1 || taskIDs[secondTask.JobID] != 1 {
		t.Fatalf("published task IDs = %+v, want each claimed exactly once", taskIDs)
	}
}

func TestDrainLeaseExpiryReplaysAfterPublishBeforeResolveCrash(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	markFailure := errors.New("simulated crash before claimed mark")
	failing := &failOnceMarkClaimedOutbox{OutboxRepository: outbox, err: markFailure}
	manager := &trackingUOW{repos: uow.Repositories{Outbox: failing}}
	publisher := newRecordingPublisher()
	relay := testRelay(manager, publisher, &now)
	task := sampleTask()
	recoveredBefore := relayCounterValue(t, metrics.OutboxRelayLeaseRecoveries, "result_ready", "recovered")
	if err := outbox.Add(ctx, taskEvent(t, EventJobResultReady, task)); err != nil {
		t.Fatalf("add event: %v", err)
	}

	if got, err := relay.Drain(ctx); !errors.Is(err, markFailure) || got != 0 {
		t.Fatalf("first Drain() = (%d, %v), want (0, mark failure)", got, err)
	}
	if got := len(publisher.inner.StreamTasks(redisqueue.StreamDelivery)); got != 1 {
		t.Fatalf("published tasks after crash = %d, want 1", got)
	}
	if got, err := relay.Drain(ctx); err != nil || got != 0 {
		t.Fatalf("Drain() before lease expiry = (%d, %v), want (0, nil)", got, err)
	}

	now = now.Add(time.Minute)
	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("Drain() after lease expiry = (%d, %v), want (1, nil)", got, err)
	}
	if got := len(publisher.inner.StreamTasks(redisqueue.StreamDelivery)); got != 2 {
		t.Fatalf("published tasks after lease recovery = %d, want at-least-once replay", got)
	}
	if got := relayCounterValue(t, metrics.OutboxRelayLeaseRecoveries, "result_ready", "recovered"); got != recoveredBefore+1 {
		t.Fatalf("lease recovery metric = %v, want %v", got, recoveredBefore+1)
	}
}

func TestDrainQuarantinesInvalidEventsAndContinuesBatch(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	relay := testRelay(manager, publisher, &now)
	valid := sampleTask()
	quarantinedBefore := relayCounterValue(t, metrics.OutboxRelayResolutions, "unknown", "quarantine", "invalid_event")
	mismatch := taskEvent(t, EventJobResultReady, sampleTask())
	mismatch.AggregateID = uuid.New()
	events := []*domain.OutboxEvent{
		{AggregateType: "job", AggregateID: uuid.New(), EventType: EventJobQueued, Payload: []byte("{")},
		taskEvent(t, "event.job.unknown", sampleTask()),
		mismatch,
		taskEvent(t, EventJobQueued, valid),
	}
	for _, event := range events {
		if err := outbox.Add(ctx, event); err != nil {
			t.Fatalf("add event: %v", err)
		}
	}

	got, err := relay.Drain(ctx)
	if err == nil || got != 1 {
		t.Fatalf("Drain() = (%d, %v), want one published event and first invalid-event error", got, err)
	}
	stored := outbox.Events()
	failed := 0
	published := 0
	for _, event := range stored {
		switch event.Status {
		case domain.OutboxFailed:
			failed++
			if event.LastErrorCode != "invalid_event" || event.FailedAt == nil {
				t.Fatalf("quarantined event = %+v, want bounded invalid_event failure", event)
			}
		case domain.OutboxPublished:
			published++
		default:
			t.Fatalf("event left unresolved: %+v", event)
		}
	}
	if failed != 3 || published != 1 {
		t.Fatalf("states = failed:%d published:%d, want 3 and 1", failed, published)
	}
	if got := len(publisher.inner.Tasks(valid.QueueName())); got != 1 {
		t.Fatalf("valid tasks published = %d, want 1", got)
	}
	if got := relayCounterValue(t, metrics.OutboxRelayResolutions, "unknown", "quarantine", "invalid_event"); got < quarantinedBefore+1 {
		t.Fatalf("unknown quarantine metric = %v, want at least %v", got, quarantinedBefore+1)
	}
}

func TestDrainPublishFailureRetriesWithDeterministicBackoff(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	publishFailure := errors.New("redis unavailable")
	publisher.enqueueErrs = []error{publishFailure}
	relay := testRelay(manager, publisher, &now, WithRetryBackoff(func(attempt int) time.Duration {
		return time.Duration(attempt) * 10 * time.Second
	}))
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, sampleTask())); err != nil {
		t.Fatalf("add event: %v", err)
	}

	if got, err := relay.Drain(ctx); !errors.Is(err, publishFailure) || got != 0 {
		t.Fatalf("first Drain() = (%d, %v), want (0, publish failure)", got, err)
	}
	stored := outbox.Events()[0]
	wantRetryAt := now.Add(10 * time.Second)
	if stored.Status != domain.OutboxPending || stored.Attempts != 1 ||
		!stored.NextAttemptAt.Equal(wantRetryAt) || stored.LastErrorCode != "publish_error" {
		t.Fatalf("retry state = %+v, want pending attempt 1 at %s", stored, wantRetryAt)
	}
	now = now.Add(9 * time.Second)
	if got, err := relay.Drain(ctx); err != nil || got != 0 {
		t.Fatalf("early Drain() = (%d, %v), want (0, nil)", got, err)
	}
	now = now.Add(time.Second)
	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("retry Drain() = (%d, %v), want (1, nil)", got, err)
	}
}

func TestDrainPublishFailureQuarantinesExhaustedRetryBudget(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	publishFailure := errors.New("redis unavailable")
	publisher.enqueueErrs = []error{publishFailure}
	relay := testRelay(manager, publisher, &now, WithMaxAttempts(3))
	event := taskEvent(t, EventJobQueued, sampleTask())
	event.Attempts = 2
	if err := outbox.Add(ctx, event); err != nil {
		t.Fatalf("add event: %v", err)
	}

	if got, err := relay.Drain(ctx); !errors.Is(err, publishFailure) || got != 0 {
		t.Fatalf("Drain() = (%d, %v), want (0, publish failure)", got, err)
	}
	stored := outbox.Events()[0]
	if stored.Status != domain.OutboxFailed || stored.Attempts != 3 ||
		stored.LastErrorCode != "publish_error" || stored.FailedAt == nil {
		t.Fatalf("exhausted state = %+v, want terminal publish_error quarantine", stored)
	}
}

func TestDrainTreatsCreatedAsExplicitAuditOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	relay := testRelay(manager, publisher, &now)
	if err := outbox.Add(ctx, taskEvent(t, EventJobCreated, sampleTask())); err != nil {
		t.Fatalf("add event: %v", err)
	}

	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("Drain() = (%d, %v), want (1, nil)", got, err)
	}
	if publisher.enqueueCalls != 0 || publisher.publishCalls != 0 {
		t.Fatalf("audit-only event called publisher: enqueue=%d publish=%d", publisher.enqueueCalls, publisher.publishCalls)
	}
	stored := outbox.Events()[0]
	if stored.Status != domain.OutboxPublished || stored.PublishedAt == nil {
		t.Fatalf("audit-only event = %+v, want published", stored)
	}
}

func TestDrainPublishesConversationTitleOnlyToDedicatedStream(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	relay := testRelay(manager, publisher, &now)
	task := queue.Task{
		JobID:         uuid.New(),
		Operation:     domain.OperationTextGenerate,
		Modality:      domain.ModalityText,
		CorrelationID: "conversation-title",
	}
	if err := outbox.Add(ctx, conversationTitleEvent(t, task, uuid.New())); err != nil {
		t.Fatalf("add title event: %v", err)
	}

	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("Drain() = (%d, %v), want (1, nil)", got, err)
	}
	if publisher.enqueueCalls != 0 {
		t.Fatalf("title event used normal generation enqueue %d times", publisher.enqueueCalls)
	}
	if publisher.publishCalls != 1 {
		t.Fatalf("title event explicit publish calls = %d, want 1", publisher.publishCalls)
	}
	if titleTasks := publisher.inner.StreamTasks(redisqueue.StreamConversationTitle); len(titleTasks) != 1 || titleTasks[0].JobID != task.JobID {
		t.Fatalf("title stream tasks = %+v, want task for %s", titleTasks, task.JobID)
	}
}

func TestDrainQuarantinesConversationTitlePayloadWithRequestData(t *testing.T) {
	for _, field := range []string{"prompt", "params"} {
		t.Run(field, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			outbox := memory.NewOutboxRepo()
			manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
			publisher := newRecordingPublisher()
			relay := testRelay(manager, publisher, &now)
			task := queue.Task{
				JobID:     uuid.New(),
				Operation: domain.OperationTextGenerate,
				Modality:  domain.ModalityText,
			}
			event := conversationTitleEvent(t, task, uuid.New())
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode title payload: %v", err)
			}
			payload[field] = json.RawMessage(`{"secret":"must-not-enter-title-stream"}`)
			var err error
			event.Payload, err = json.Marshal(payload)
			if err != nil {
				t.Fatalf("encode unsafe title payload: %v", err)
			}
			if err := outbox.Add(ctx, event); err != nil {
				t.Fatalf("add unsafe title event: %v", err)
			}

			if got, err := relay.Drain(ctx); err == nil || got != 0 {
				t.Fatalf("Drain() = (%d, %v), want (0, invalid-event error)", got, err)
			}
			if publisher.enqueueCalls != 0 || publisher.publishCalls != 0 {
				t.Fatalf("unsafe title event reached publisher: enqueue=%d publish=%d", publisher.enqueueCalls, publisher.publishCalls)
			}
			stored := outbox.Events()[0]
			if stored.Status != domain.OutboxFailed || stored.LastErrorCode != "invalid_event" {
				t.Fatalf("unsafe title event state = %+v, want invalid_event quarantine", stored)
			}
		})
	}
}

func TestTitleStreamPublishFailureDoesNotRepublishNormalGeneration(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	outbox := memory.NewOutboxRepo()
	manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
	publisher := newRecordingPublisher()
	titlePublishFailure := errors.New("title stream unavailable")
	publisher.publishErrs = []error{titlePublishFailure}
	relay := testRelay(manager, publisher, &now, WithBatchSize(2), WithRetryBackoff(func(int) time.Duration { return 0 }))
	normalTask := sampleTask()
	titleTask := queue.Task{
		JobID:         uuid.New(),
		Operation:     domain.OperationTextGenerate,
		Modality:      domain.ModalityText,
		CorrelationID: "conversation-title-failure",
	}
	if err := outbox.Add(ctx, taskEvent(t, EventJobQueued, normalTask)); err != nil {
		t.Fatalf("add normal event: %v", err)
	}
	if err := outbox.Add(ctx, conversationTitleEvent(t, titleTask, uuid.New())); err != nil {
		t.Fatalf("add title event: %v", err)
	}

	if got, err := relay.Drain(ctx); got != 1 || !errors.Is(err, titlePublishFailure) {
		t.Fatalf("first Drain() = (%d, %v), want (1, title publish failure)", got, err)
	}
	if publisher.enqueueCalls != 1 {
		t.Fatalf("normal generation enqueues after title failure = %d, want 1", publisher.enqueueCalls)
	}
	if publisher.publishCalls != 1 {
		t.Fatalf("title stream publishes after failure = %d, want 1", publisher.publishCalls)
	}

	if got, err := relay.Drain(ctx); err != nil || got != 1 {
		t.Fatalf("second Drain() = (%d, %v), want (1, nil)", got, err)
	}
	if publisher.enqueueCalls != 1 {
		t.Fatalf("normal generation was republished after title retry: %d calls", publisher.enqueueCalls)
	}
	if publisher.publishCalls != 2 {
		t.Fatalf("title stream publish retries = %d, want 2", publisher.publishCalls)
	}
}

func TestDrainQuarantinesInvalidCreatedEventEnvelope(t *testing.T) {
	validTask := sampleTask()
	for _, test := range []struct {
		name  string
		event func(*testing.T) *domain.OutboxEvent
	}{
		{
			name: "malformed JSON",
			event: func(*testing.T) *domain.OutboxEvent {
				return &domain.OutboxEvent{
					AggregateType: "job",
					AggregateID:   uuid.New(),
					EventType:     EventJobCreated,
					Payload:       []byte("{"),
				}
			},
		},
		{
			name: "missing job envelope",
			event: func(*testing.T) *domain.OutboxEvent {
				return &domain.OutboxEvent{
					AggregateType: "job",
					AggregateID:   uuid.New(),
					EventType:     EventJobCreated,
					Payload:       []byte(`{}`),
				}
			},
		},
		{
			name: "invalid operation",
			event: func(t *testing.T) *domain.OutboxEvent {
				event := taskEvent(t, EventJobCreated, validTask)
				event.Payload = []byte(`{"job_id":"` + validTask.JobID.String() + `","operation":"invalid","modality":"image"}`)
				return event
			},
		},
		{
			name: "invalid modality",
			event: func(t *testing.T) *domain.OutboxEvent {
				event := taskEvent(t, EventJobCreated, validTask)
				event.Payload = []byte(`{"job_id":"` + validTask.JobID.String() + `","operation":"image_generate","modality":"invalid"}`)
				return event
			},
		},
		{
			name: "aggregate type mismatch",
			event: func(t *testing.T) *domain.OutboxEvent {
				event := taskEvent(t, EventJobCreated, validTask)
				event.AggregateType = "payment"
				return event
			},
		},
		{
			name: "aggregate id mismatch",
			event: func(t *testing.T) *domain.OutboxEvent {
				event := taskEvent(t, EventJobCreated, validTask)
				event.AggregateID = uuid.New()
				return event
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
			outbox := memory.NewOutboxRepo()
			manager := memory.NewUnitOfWork(memory.NewJobRepo(), outbox, nil)
			publisher := newRecordingPublisher()
			relay := testRelay(manager, publisher, &now)
			if err := outbox.Add(ctx, test.event(t)); err != nil {
				t.Fatalf("add event: %v", err)
			}

			if got, err := relay.Drain(ctx); err == nil || got != 0 {
				t.Fatalf("Drain() = (%d, %v), want (0, invalid-event error)", got, err)
			}
			if publisher.enqueueCalls != 0 || publisher.publishCalls != 0 {
				t.Fatalf("invalid audit event called publisher: enqueue=%d publish=%d", publisher.enqueueCalls, publisher.publishCalls)
			}
			stored := outbox.Events()[0]
			if stored.Status != domain.OutboxFailed || stored.LastErrorCode != "invalid_event" || stored.FailedAt == nil {
				t.Fatalf("invalid created event = %+v, want terminal invalid_event quarantine", stored)
			}
		})
	}
}

func TestRelayDefaultsAreBounded(t *testing.T) {
	relay := New(nil, nil)
	if relay.batch != 100 {
		t.Fatalf("default batch = %d, want 100", relay.batch)
	}
	if relay.leaseDuration < 10*time.Second || relay.leaseDuration > 5*time.Minute {
		t.Fatalf("default lease = %s, want enough for one Redis call and bounded", relay.leaseDuration)
	}
	if relay.maxAttempts < 1 || relay.maxAttempts > 10 {
		t.Fatalf("default max attempts = %d, want within [1, 10]", relay.maxAttempts)
	}
	if relay.owner == "" {
		t.Fatal("default owner must be non-empty")
	}
	if relay.retryBackoff == nil || relay.now == nil {
		t.Fatal("default retry backoff and clock must be configured")
	}
}

func testRelay(manager uow.Manager, publisher StreamPublisher, now *time.Time, opts ...Option) *Relay {
	base := []Option{
		WithNow(func() time.Time { return *now }),
		WithLeaseDuration(time.Minute),
		WithMaxAttempts(3),
		WithOwner("relay-test"),
	}
	return New(manager, publisher, append(base, opts...)...)
}

func sampleTask() queue.Task {
	return queue.Task{
		JobID:         uuid.New(),
		Operation:     domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		CorrelationID: "corr",
		Traceparent:   "trace",
	}
}

func taskEvent(t *testing.T, eventType string, task queue.Task) *domain.OutboxEvent {
	t.Helper()
	payload, err := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		CorrelationID string               `json:"correlation_id,omitempty"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{task.JobID, task.Operation, task.Modality, task.CorrelationID, task.Traceparent})
	if err != nil {
		t.Fatalf("marshal task payload: %v", err)
	}
	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   task.JobID,
		EventType:     eventType,
		Payload:       payload,
	}
}

func conversationTitleEvent(t *testing.T, task queue.Task, accountID uuid.UUID) *domain.OutboxEvent {
	t.Helper()
	payload, err := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		AccountID     uuid.UUID            `json:"account_id"`
		Source        string               `json:"source"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		CorrelationID string               `json:"correlation_id,omitempty"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{task.JobID, accountID, "web", task.Operation, task.Modality, task.CorrelationID, task.Traceparent})
	if err != nil {
		t.Fatalf("marshal title event payload: %v", err)
	}
	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   task.JobID,
		EventType:     EventConversationTitleQueued,
		Payload:       payload,
	}
}

func relayCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	metricCounter, err := counter.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := metricCounter.Write(&metric); err != nil {
		t.Fatalf("counter.Write() error = %v", err)
	}
	return metric.GetCounter().GetValue()
}

func relayHistogramCount(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	observer, err := histogram.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	metricWriter, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not implement prometheus.Metric")
	}
	var metric dto.Metric
	if err := metricWriter.Write(&metric); err != nil {
		t.Fatalf("histogram.Write() error = %v", err)
	}
	return metric.GetHistogram().GetSampleCount()
}

func relayHistogramSum(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) time.Duration {
	t.Helper()
	observer, err := histogram.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	metricWriter, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not implement prometheus.Metric")
	}
	var metric dto.Metric
	if err := metricWriter.Write(&metric); err != nil {
		t.Fatalf("histogram.Write() error = %v", err)
	}
	return time.Duration(metric.GetHistogram().GetSampleSum() * float64(time.Second))
}
