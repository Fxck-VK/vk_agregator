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
)

type fixture struct {
	jobs    *memory.JobRepo
	outbox  *memory.OutboxRepo
	billing *billingservice.Service
	pub     *queue.MemoryPublisher
	orch    *joborchestrator.Orchestrator
}

func newFixture(opts ...billingservice.Option) *fixture {
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	billing := billingservice.New(memory.NewBillingRepo(), opts...)
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox)
	return &fixture{
		jobs:    jobs,
		outbox:  outbox,
		billing: billing,
		pub:     pub,
		orch:    joborchestrator.New(jobs, uowMgr, billing, pub),
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

	// Task enqueued on the video queue.
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
	if f.pub.Len() != 1 {
		t.Fatalf("expected exactly 1 enqueued task, got %d", f.pub.Len())
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
	if f.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", f.pub.Len())
	}
}
