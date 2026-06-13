package joborchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

type fixture struct {
	jobs    *memory.JobRepo
	outbox  *memory.OutboxRepo
	billing *billingservice.Service
	pub     *queue.MemoryPublisher
	relay   *outboxrelay.Relay
	orch    *joborchestrator.Orchestrator
}

func newFixture(opts ...billingservice.Option) *fixture {
	return newFixtureWithOrchestratorOptions(nil, opts...)
}

func newFixtureWithOrchestratorOptions(orchOpts []joborchestrator.Option, opts ...billingservice.Option) *fixture {
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	bill := memory.NewBillingRepo()
	billing := billingservice.New(bill, opts...)
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox, bill)
	return &fixture{
		jobs:    jobs,
		outbox:  outbox,
		billing: billing,
		pub:     pub,
		relay:   outboxrelay.New(uowMgr, pub),
		orch:    joborchestrator.New(jobs, uowMgr, billing, 0, orchOpts...),
	}
}

// drain publishes any queued outbox events to the in-memory queue, mirroring
// what the outbox relay does in the worker process.
func (f *fixture) drain(t *testing.T) {
	t.Helper()
	if _, err := f.relay.Drain(context.Background()); err != nil {
		t.Fatalf("relay drain: %v", err)
	}
}

func TestCreateJobHappyPath(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         userID,
		VKPeerID:       42,
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:e1",
		CorrelationID:  "corr-1",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if job.Status != domain.JobStatusQueued {
		t.Fatalf("status = %q, want queued", job.Status)
	}
	if job.CostEstimate != 50 || job.CostReserved != 50 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 50/50", job.CostEstimate, job.CostReserved)
	}

	// Job persisted and reservation reduced available balance.
	acc, _ := f.billing.EnsureAccount(ctx, userID)
	if acc.BalanceCached != billingservice.DefaultStartingBalance {
		t.Fatalf("balance changed before capture: %d", acc.BalanceCached)
	}

	// Outbox holds created + queued events.
	events := f.outbox.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 outbox events, got %d", len(events))
	}
	if events[0].EventType != "event.job.created" || events[1].EventType != "event.job.queued" {
		t.Fatalf("unexpected event types: %s, %s", events[0].EventType, events[1].EventType)
	}

	// The relay publishes the queued event onto the video queue.
	f.drain(t)
	tasks := f.pub.Tasks("queue.video.generate")
	if len(tasks) != 1 || tasks[0].JobID != job.ID {
		t.Fatalf("expected task for job on video queue, got %+v", tasks)
	}
}

func TestCreateJobIdempotent(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	in := joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		IdempotencyKey: "vk_job:1:dup",
	}

	first, err := f.orch.CreateJob(ctx, in)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := f.orch.CreateJob(ctx, in)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if first.ID != second.ID {
		t.Fatal("expected same job id for identical idempotency key")
	}
	f.drain(t)
	if f.pub.Len() != 1 {
		t.Fatalf("expected exactly 1 enqueued task, got %d", f.pub.Len())
	}
}

func TestCreateJobZeroCostQueuesWithoutReservation(t *testing.T) {
	f := newFixture(billingservice.WithPriceOverrides(map[string]int64{
		string(domain.OperationImageGenerate): 0,
	}))
	ctx := context.Background()
	userID := uuid.New()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         userID,
		VKPeerID:       42,
		CommandID:      uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		IdempotencyKey: "vk_job:1:free-image",
		CorrelationID:  "corr-free-image",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.Status != domain.JobStatusQueued {
		t.Fatalf("status = %q, want queued", job.Status)
	}
	if job.CostEstimate != 0 || job.CostReserved != 0 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 0/0", job.CostEstimate, job.CostReserved)
	}

	f.drain(t)
	tasks := f.pub.Tasks("queue.image.generate")
	if len(tasks) != 1 || tasks[0].JobID != job.ID {
		t.Fatalf("expected task for free image job, got %+v", tasks)
	}
}

func TestCreateJobInsufficientCredits(t *testing.T) {
	// Start accounts with only 5 credits so a 50-credit video job cannot be
	// reserved.
	f := newFixture(billingservice.WithStartingBalance(5))
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:poor",
	})
	if !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatalf("expected ErrInsufficientCredits, got %v", err)
	}
	if job == nil || job.Status != domain.JobStatusAwaitingPayment {
		t.Fatalf("expected job parked in awaiting_payment, got %+v", job)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", f.pub.Len())
	}
}

func TestCreateJobCapacityGuardRejectsBeforePersistenceReservationAndOutbox(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithCapacityGuard(joborchestrator.CapacityGuardFunc(func(context.Context, joborchestrator.CapacityCheckInput) error {
			return domain.ErrCapacityDegraded
		})),
	})
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:overloaded",
	})
	if !errors.Is(err, domain.ErrCapacityDegraded) {
		t.Fatalf("expected ErrCapacityDegraded, got job=%+v err=%v", job, err)
	}
	if job != nil {
		t.Fatalf("capacity rejection must not create job, got %+v", job)
	}
	jobs, listErr := f.jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if listErr != nil {
		t.Fatalf("list jobs: %v", listErr)
	}
	if len(jobs) != 0 {
		t.Fatalf("capacity rejection persisted jobs: %+v", jobs)
	}
	if events := f.outbox.Events(); len(events) != 0 {
		t.Fatalf("capacity rejection wrote outbox events: %+v", events)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("capacity rejection enqueued tasks, got %d", f.pub.Len())
	}
}

func TestCreateJobActiveVideoLimitRejectsBeforeReservation(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithMaxActiveVideoJobsPerUser(1),
	})
	ctx := context.Background()
	userID := uuid.New()
	existing := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "vk_job:1:existing-video",
	}
	if err := f.jobs.Create(ctx, existing); err != nil {
		t.Fatalf("seed active video job: %v", err)
	}

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         userID,
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:second-video",
	})
	if !errors.Is(err, domain.ErrActiveJobLimitExceeded) {
		t.Fatalf("expected ErrActiveJobLimitExceeded, got job=%+v err=%v", job, err)
	}
	if job != nil {
		t.Fatalf("active job rejection must not create job, got %+v", job)
	}
	jobs, listErr := f.jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if listErr != nil {
		t.Fatalf("list jobs: %v", listErr)
	}
	if len(jobs) != 1 || jobs[0].ID != existing.ID {
		t.Fatalf("active limit must leave only existing job, got %+v", jobs)
	}
	if events := f.outbox.Events(); len(events) != 0 {
		t.Fatalf("active limit wrote outbox events: %+v", events)
	}
}

func TestCreateJobIdempotentExistingBypassesCapacityGuard(t *testing.T) {
	var guardErr error
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithCapacityGuard(joborchestrator.CapacityGuardFunc(func(context.Context, joborchestrator.CapacityCheckInput) error {
			return guardErr
		})),
	})
	ctx := context.Background()
	in := joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:capacity-idempotent",
	}

	first, err := f.orch.CreateJob(ctx, in)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	guardErr = domain.ErrCapacityDegraded
	second, err := f.orch.CreateJob(ctx, in)
	if err != nil {
		t.Fatalf("idempotent create should bypass capacity guard, got %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected existing job %s, got %s", first.ID, second.ID)
	}
}
