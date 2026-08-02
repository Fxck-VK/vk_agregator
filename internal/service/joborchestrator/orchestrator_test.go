package joborchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

type fixture struct {
	jobs    *memory.JobRepo
	outbox  *memory.OutboxRepo
	bill    *memory.BillingRepo
	billing *billingservice.Service
	arts    *memory.ArtifactRepo
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
	arts := memory.NewArtifactRepo()
	billing := billingservice.New(bill, opts...)
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox, bill)
	return &fixture{
		jobs:    jobs,
		outbox:  outbox,
		bill:    bill,
		billing: billing,
		arts:    arts,
		pub:     pub,
		relay:   outboxrelay.New(uowMgr, pub),
		orch:    joborchestrator.New(jobs, uowMgr, billing, 0, append([]joborchestrator.Option{joborchestrator.WithArtifactRepository(arts)}, orchOpts...)...),
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
	const startingBalance int64 = 1000
	f := newFixture(billingservice.WithStartingBalance(startingBalance))
	ctx := context.Background()
	userID := uuid.New()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              userID,
		VKPeerID:            42,
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:e1",
		CorrelationID:       "corr-1",
		CostEstimateCredits: 50,
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
	if acc.BalanceCached != startingBalance {
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

func TestCreateJobRejectsImageWithoutBackendPrice(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		IdempotencyKey: "miniapp_job:1:nano-banana-2",
	})
	if !errors.Is(err, joborchestrator.ErrBackendPriceRequired) {
		t.Fatalf("expected ErrBackendPriceRequired, got job=%+v err=%v", job, err)
	}
	if job != nil {
		t.Fatalf("missing backend price must not create job, got %+v", job)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("missing backend price enqueued tasks, got %d", f.pub.Len())
	}
}

func TestCreateJobUsesBackendCostEstimateOverride(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "miniapp_job:1:priced-nano-banana-2",
		CostEstimateCredits: 15,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.CostEstimate != 15 || job.CostReserved != 15 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 15/15", job.CostEstimate, job.CostReserved)
	}
}

func TestCreateJobPersistsPricingSnapshotAndUsesSnapshotAmount(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new pricing catalog: %v", err)
	}
	snapshot, err := catalog.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageNanoBanana2,
		Quality:      pricingcatalog.ImageQuality1K,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:          uuid.New(),
		CommandID:       uuid.New(),
		Operation:       domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		IdempotencyKey:  "miniapp_job:1:priced-snapshot-nano-banana-2",
		PricingSnapshot: snapshot,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.CostEstimate != snapshot.InternalCredits || job.CostReserved != snapshot.InternalCredits {
		t.Fatalf("cost estimate/reserved = %d/%d, want %d/%d", job.CostEstimate, job.CostReserved, snapshot.InternalCredits, snapshot.InternalCredits)
	}
	if credits, ok := job.PricingSnapshotCredits(); !ok || credits != snapshot.InternalCredits {
		t.Fatalf("pricing snapshot credits = %d/%v, want %d/true", credits, ok, snapshot.InternalCredits)
	}
	stored, err := f.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("get stored job: %v", err)
	}
	var storedSnapshot pricingcatalog.PricingSnapshot
	if err := json.Unmarshal(stored.PricingSnapshot, &storedSnapshot); err != nil {
		t.Fatalf("decode stored snapshot: %v", err)
	}
	if !storedSnapshot.Valid() || storedSnapshot.InternalCredits != snapshot.InternalCredits || storedSnapshot.Key != snapshot.Key {
		t.Fatalf("unexpected stored snapshot: %+v", storedSnapshot)
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

func TestCreateJobAllowsFreeTextWithoutReservation(t *testing.T) {
	f := newFixture(billingservice.WithPriceOverrides(map[string]int64{
		string(domain.OperationTextGenerate): 0,
	}))
	ctx := context.Background()
	userID := uuid.New()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         userID,
		VKPeerID:       42,
		CommandID:      uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		IdempotencyKey: "vk_job:1:free-text",
		CorrelationID:  "corr-free-text",
	})
	if err != nil {
		t.Fatalf("create free text job: %v", err)
	}
	if job.CostEstimate != 0 || job.CostReserved != 0 || job.Status != domain.JobStatusQueued {
		t.Fatalf("free text job cost/status = %d/%d/%s, want 0/0/queued", job.CostEstimate, job.CostReserved, job.Status)
	}
	jobs, err := f.jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("free text persisted jobs=%+v err=%v", jobs, err)
	}
	f.drain(t)
	if f.pub.Len() != 1 {
		t.Fatalf("free text job enqueued tasks = %d, want 1", f.pub.Len())
	}
}

func TestCreateWebTextJobWithConversationTitleRequestAddsIndependentSafeOutboxEvent(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	accountID := uuid.New()
	const rawPrompt = "do-not-put-this-prompt-in-the-title-event"

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		AccountID:                  accountID,
		Source:                     "web",
		ChannelContext:             &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:                 domain.ResultModeAccountHistory,
		Operation:                  domain.OperationTextGenerate,
		Modality:                   domain.ModalityText,
		IdempotencyKey:             "web-chat:title-event",
		CorrelationID:              "web-chat-title-event",
		Params:                     json.RawMessage(`{"prompt":"` + rawPrompt + `"}`),
		ConversationTitleRequested: true,
	})
	if err != nil {
		t.Fatalf("create web text job: %v", err)
	}

	events := f.outbox.Events()
	if len(events) != 3 {
		t.Fatalf("outbox event count = %d, want created + queued + title", len(events))
	}
	var titleEvent *domain.OutboxEvent
	for i := range events {
		if events[i].EventType == outboxrelay.EventConversationTitleQueued {
			titleEvent = &events[i]
			break
		}
	}
	if titleEvent == nil {
		t.Fatalf("missing %q event in %+v", outboxrelay.EventConversationTitleQueued, events)
	}
	if titleEvent.AggregateType != "job" || titleEvent.AggregateID != job.ID {
		t.Fatalf("title event aggregate = %s/%s, want job/%s", titleEvent.AggregateType, titleEvent.AggregateID, job.ID)
	}
	if bytes.Contains(titleEvent.Payload, []byte(rawPrompt)) || bytes.Contains(titleEvent.Payload, []byte(`"params"`)) || bytes.Contains(titleEvent.Payload, []byte(`"prompt"`)) {
		t.Fatalf("title event leaked request data: %s", titleEvent.Payload)
	}

	f.drain(t)
	if normal := f.pub.Tasks("queue.text.generate"); len(normal) != 1 || normal[0].JobID != job.ID {
		t.Fatalf("normal generation tasks = %+v, want one task for %s", normal, job.ID)
	}
	if title := f.pub.StreamTasks(redisqueue.StreamConversationTitle); len(title) != 1 || title[0].JobID != job.ID {
		t.Fatalf("title stream tasks = %+v, want one task for %s", title, job.ID)
	}
}

func TestCreateJobRejectsConversationTitleRequestOutsideWebText(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		AccountID:                  uuid.New(),
		Source:                     "web",
		ChannelContext:             &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:                 domain.ResultModeAccountHistory,
		Operation:                  domain.OperationImageGenerate,
		Modality:                   domain.ModalityImage,
		IdempotencyKey:             "web-image:title-event",
		CostEstimateCredits:        1,
		ConversationTitleRequested: true,
	})
	if !errors.Is(err, joborchestrator.ErrInvalidConversationTitleRequest) {
		t.Fatalf("CreateJob() error = %v, want ErrInvalidConversationTitleRequest", err)
	}
	if job != nil {
		t.Fatalf("invalid title request created job: %+v", job)
	}
	if events := f.outbox.Events(); len(events) != 0 {
		t.Fatalf("invalid title request created outbox events: %+v", events)
	}
}

func TestCreateJobInsufficientCredits(t *testing.T) {
	// Start accounts with only 5 credits so a 50-credit video job cannot be
	// reserved.
	f := newFixture(billingservice.WithStartingBalance(5))
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:poor",
		CostEstimateCredits: 50,
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
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:overloaded",
		CostEstimateCredits: 50,
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

func TestCreateJobVideoRouteValidatorRejectsBeforePersistenceReservationAndOutbox(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithVideoRouteValidator(joborchestrator.VideoRouteValidatorFunc(func(context.Context, joborchestrator.VideoRouteCheckInput) error {
			return videorouter.ErrUnsupportedDuration
		})),
	})
	ctx := context.Background()

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		CommandID:      uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		IdempotencyKey: "vk_job:1:bad-route",
	})
	if !errors.Is(err, videorouter.ErrUnsupportedDuration) {
		t.Fatalf("expected ErrUnsupportedDuration, got job=%+v err=%v", job, err)
	}
	if job != nil {
		t.Fatalf("route rejection must not create job, got %+v", job)
	}
	jobs, listErr := f.jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if listErr != nil {
		t.Fatalf("list jobs: %v", listErr)
	}
	if len(jobs) != 0 {
		t.Fatalf("route rejection persisted jobs: %+v", jobs)
	}
	if events := f.outbox.Events(); len(events) != 0 {
		t.Fatalf("route rejection wrote outbox events: %+v", events)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("route rejection enqueued tasks, got %d", f.pub.Len())
	}
}

func TestCreateJobRejectsInvalidInputArtifactsBeforePersistenceReservationAndOutbox(t *testing.T) {
	tests := []struct {
		name string
		seed func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID
	}{
		{
			name: "missing artifact",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return uuid.New()
			},
		},
		{
			name: "foreign owner input artifact",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return seedInputArtifact(t, f, uuid.New(), domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/foreign.png")
			},
		},
		{
			name: "output artifact reused as input",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return seedInputArtifact(t, f, userID, domain.ArtifactKindOutput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/output.png")
			},
		},
		{
			name: "non-ready input artifact",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusStored, "artifacts", "refs/stored.png")
			},
		},
		{
			name: "non-image input artifact",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeVideo, domain.ArtifactStatusReady, "artifacts", "refs/video.mp4")
			},
		},
		{
			name: "storage-empty input artifact",
			seed: func(t *testing.T, f *fixture, userID uuid.UUID) uuid.UUID {
				return seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "", "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(billingservice.WithStartingBalance(1000))
			ctx := context.Background()
			userID := uuid.New()
			artifactID := tt.seed(t, f, userID)

			job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
				UserID:              userID,
				CommandID:           uuid.New(),
				Operation:           domain.OperationImageGenerate,
				Modality:            domain.ModalityImage,
				IdempotencyKey:      "vk_job:1:invalid-input:" + tt.name,
				InputArtifactIDs:    []uuid.UUID{artifactID},
				CostEstimateCredits: 50,
			})
			if !errors.Is(err, joborchestrator.ErrInvalidInputArtifact) {
				t.Fatalf("expected ErrInvalidInputArtifact, got job=%+v err=%v", job, err)
			}
			if job != nil {
				t.Fatalf("invalid input artifact must not create job, got %+v", job)
			}
			assertNoJobReservationOrTask(t, f, userID)
		})
	}
}

func TestCreateJobAcceptsReadyOwnedInputImageArtifact(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(1000))
	ctx := context.Background()
	userID := uuid.New()
	artifactID := seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/ready.png")

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              userID,
		CommandID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "vk_job:1:valid-input-image",
		InputArtifactIDs:    []uuid.UUID{artifactID},
		CostEstimateCredits: 50,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job == nil || job.Status != domain.JobStatusQueued || job.CostReserved != 50 {
		t.Fatalf("expected queued reserved job, got %+v", job)
	}
	if len(job.InputArtifactIDs) != 1 || job.InputArtifactIDs[0] != artifactID {
		t.Fatalf("job input artifacts = %+v, want [%s]", job.InputArtifactIDs, artifactID)
	}
	f.drain(t)
	if f.pub.Len() != 1 {
		t.Fatalf("valid input artifact job should enqueue once, got %d tasks", f.pub.Len())
	}
}

func TestCreateJobAcceptsInputArtifactOwnedBySameAccountAcrossLegacyUsers(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(1000))
	ctx := context.Background()
	accountID := uuid.New()
	artifactOwnerUserID := uuid.New()
	requestUserID := uuid.New()
	artifactID := seedInputArtifactForAccount(
		t,
		f,
		artifactOwnerUserID,
		accountID,
		domain.ArtifactKindInput,
		domain.MediaTypeImage,
		domain.ArtifactStatusReady,
		"artifacts",
		"refs/account-owned.png",
	)

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              requestUserID,
		AccountID:           accountID,
		CommandID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "vk_job:account-owned-input",
		InputArtifactIDs:    []uuid.UUID{artifactID},
		CostEstimateCredits: 50,
	})
	if err != nil {
		t.Fatalf("create job with account-owned artifact: %v", err)
	}
	if job == nil || job.AccountID != accountID {
		t.Fatalf("job account = %+v, want %s", job, accountID)
	}
}

func TestCreateJobRejectsInputArtifactOwnedByDifferentAccount(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(1000))
	ctx := context.Background()
	userID := uuid.New()
	artifactID := seedInputArtifactForAccount(
		t,
		f,
		userID,
		uuid.New(),
		domain.ArtifactKindInput,
		domain.MediaTypeImage,
		domain.ArtifactStatusReady,
		"artifacts",
		"refs/foreign-account.png",
	)

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              userID,
		AccountID:           uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "vk_job:foreign-account-input",
		InputArtifactIDs:    []uuid.UUID{artifactID},
		CostEstimateCredits: 50,
	})
	if !errors.Is(err, joborchestrator.ErrInvalidInputArtifact) {
		t.Fatalf("expected ErrInvalidInputArtifact, got job=%+v err=%v", job, err)
	}
	if job != nil {
		t.Fatalf("foreign account artifact must not create job, got %+v", job)
	}
}

func TestCreateJobResolvedVideoRouteUsesBackendEstimateBeforeReservation(t *testing.T) {
	catalog := newRouteCatalogForOrchestratorTest(t)
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithVideoRouteResolver(routeResolverForTest(catalog)),
	}, billingservice.WithStartingBalance(150))
	ctx := context.Background()

	params, _ := json.Marshal(map[string]any{
		"prompt":            "clean prompt",
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      10,
		"resolution":        "720p",
		"aspect_ratio":      "16:9",
	})
	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:route-expensive",
		Params:              params,
		CostEstimateCredits: 200,
	})
	if !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatalf("expected ErrInsufficientCredits, got job=%+v err=%v", job, err)
	}
	if job == nil || job.Status != domain.JobStatusAwaitingPayment {
		t.Fatalf("expected awaiting_payment job, got %+v", job)
	}
	if job.CostEstimate != 200 || job.CostReserved != 0 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 200/0", job.CostEstimate, job.CostReserved)
	}
	var out struct {
		Snapshot domain.VideoRouteSnapshot `json:"resolved_video_route"`
	}
	if err := json.Unmarshal(job.Params, &out); err != nil {
		t.Fatalf("unmarshal job params: %v", err)
	}
	if !out.Snapshot.Valid() || out.Snapshot.InternalCostCredits != 200 {
		t.Fatalf("missing route snapshot: %+v", out.Snapshot)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("awaiting_payment job must not enqueue, got %d tasks", f.pub.Len())
	}
}

func TestCreateJobResolvedVideoRouteReservesResolvedAmount(t *testing.T) {
	catalog := newRouteCatalogForOrchestratorTest(t)
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithVideoRouteResolver(routeResolverForTest(catalog)),
	}, billingservice.WithStartingBalance(1000))
	ctx := context.Background()

	params, _ := json.Marshal(map[string]any{
		"prompt":            "clean prompt",
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      10,
		"resolution":        "720p",
		"aspect_ratio":      "16:9",
	})
	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:route-reserve",
		Params:              params,
		CostEstimateCredits: 200,
	})
	if err != nil {
		t.Fatalf("create route job: %v", err)
	}
	if job.CostEstimate != 200 || job.CostReserved != 200 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 200/200", job.CostEstimate, job.CostReserved)
	}
	f.drain(t)
	if f.pub.Len() != 1 {
		t.Fatalf("reserved route job should enqueue once, got %d tasks", f.pub.Len())
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
		UserID:              userID,
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:second-video",
		CostEstimateCredits: 50,
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
	}, billingservice.WithStartingBalance(1000))
	ctx := context.Background()
	in := joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		CommandID:           uuid.New(),
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      "vk_job:1:capacity-idempotent",
		CostEstimateCredits: 50,
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

func TestPrepareAccountJobPersistsOnlyCreatedEventAndNeverEnqueues(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	accountID := uuid.New()
	artifactID := seedInputArtifactForAccount(t, f, uuid.Nil, accountID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/account-native.png")
	in := joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:prepare:one",
		CorrelationID:       "web-corr-one",
		InputArtifactIDs:    []uuid.UUID{artifactID},
		Params:              json.RawMessage(`{"quality":"high","count":1}`),
		CostEstimateCredits: 25,
	}

	job, err := f.orch.PrepareAccountJob(ctx, in)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if job.AccountID != accountID || job.UserID != uuid.Nil || job.CommandID != uuid.Nil || job.VKPeerID != 0 || job.Source != "web" || job.Status != domain.JobStatusPrepared {
		t.Fatalf("unexpected prepared job: %+v", job)
	}
	if job.CostEstimate != 25 || job.CostReserved != 0 || job.CostCaptured != 0 {
		t.Fatalf("prepared job costs = %d/%d/%d, want 25/0/0", job.CostEstimate, job.CostReserved, job.CostCaptured)
	}

	events := f.outbox.Events()
	if len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("prepared job events = %+v, want one created event", events)
	}
	if _, err := f.bill.GetAccountByOwner(ctx, accountID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("prepared job must not create a credit account/reservation, got %v", err)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("prepared job must not publish a worker task, got %d", f.pub.Len())
	}

	// Canonically equivalent parameters replay the same immutable prepared row.
	in.Params = json.RawMessage(`{"count":1,"quality":"high"}`)
	replayed, err := f.orch.PrepareAccountJob(ctx, in)
	if err != nil || replayed.ID != job.ID {
		t.Fatalf("equivalent replay = %+v, %v; want %s, nil", replayed, err, job.ID)
	}
	if events := f.outbox.Events(); len(events) != 1 {
		t.Fatalf("replay wrote another created event: %+v", events)
	}

	in.Params = json.RawMessage(`{"count":2,"quality":"high"}`)
	if _, err := f.orch.PrepareAccountJob(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed payload error = %v, want ErrConflict", err)
	}
	in.Params = json.RawMessage(`{"quality":"high","count":1}`)
	in.CostEstimateCredits = 26
	if _, err := f.orch.PrepareAccountJob(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed pricing error = %v, want ErrConflict", err)
	}
	in.CostEstimateCredits = 25
	in.AccountID = uuid.New()
	in.Params = json.RawMessage(`{"quality":"high","count":1}`)
	in.InputArtifactIDs = nil
	if _, err := f.orch.PrepareAccountJob(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-account key error = %v, want ErrConflict", err)
	}
}

func TestPrepareAccountJobCapsUnexpiredWebImageJobsPerAccount(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(2, time.Hour),
	})
	ctx := context.Background()
	accountID := uuid.New()

	for _, key := range []string{"web:prepare:cap:first", "web:prepare:cap:second"} {
		job, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
			AccountID:           accountID,
			Operation:           domain.OperationImageGenerate,
			Modality:            domain.ModalityImage,
			IdempotencyKey:      key,
			CostEstimateCredits: 25,
		})
		if err != nil || job == nil || job.Status != domain.JobStatusPrepared {
			t.Fatalf("prepare %q = %+v, %v", key, job, err)
		}
		if job.ExpiresAt == nil {
			t.Fatalf("prepare %q must receive a confirmation expiry", key)
		}
	}

	job, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:prepare:cap:third",
		CostEstimateCredits: 25,
	})
	if !errors.Is(err, domain.ErrPreparedJobLimitExceeded) || job != nil {
		t.Fatalf("third distinct prepare = %+v, %v; want nil ErrPreparedJobLimitExceeded", job, err)
	}
	if events := f.outbox.Events(); len(events) != 2 {
		t.Fatalf("cap rejection must not add an outbox event: %+v", events)
	}
}

func TestCreateJobRejectsPreparedAccountIdempotencyCollision(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	key := "cross-surface-prepared-key"
	prepared, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           uuid.New(),
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      key,
		CostEstimateCredits: 1,
	})
	if err != nil {
		t.Fatalf("prepare account job: %v", err)
	}

	job, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		VKPeerID:       42,
		CommandID:      uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		IdempotencyKey: key,
	})
	if !errors.Is(err, domain.ErrConflict) || job != nil {
		t.Fatalf("legacy collision = %+v, %v; want nil, ErrConflict", job, err)
	}
	if events := f.outbox.Events(); len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("collision changed outbox: %+v", events)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("collision published worker work: %d", f.pub.Len())
	}
	stored, err := f.jobs.GetByID(ctx, prepared.ID)
	if err != nil || stored.Status != domain.JobStatusPrepared || stored.CostReserved != 0 {
		t.Fatalf("prepared row mutated: %+v, %v", stored, err)
	}
}

func TestPrepareAccountJobRejectsLegacyIdempotencyCollision(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	key := "cross-surface-legacy-key"
	legacy, err := f.orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID:         uuid.New(),
		VKPeerID:       42,
		CommandID:      uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("create legacy job: %v", err)
	}

	prepared, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           uuid.New(),
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      key,
		CostEstimateCredits: 1,
	})
	if !errors.Is(err, domain.ErrConflict) || prepared != nil {
		t.Fatalf("prepared collision = %+v, %v; want nil, ErrConflict", prepared, err)
	}
	stored, err := f.jobs.GetByID(ctx, legacy.ID)
	if err != nil || stored.Status != domain.JobStatusQueued {
		t.Fatalf("legacy row mutated: %+v, %v", stored, err)
	}
}

func TestPrepareAccountJobRejectsInvalidSnapshotAndConflictsOnSnapshotReplayChange(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new pricing catalog: %v", err)
	}
	snapshot, err := catalog.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageNanoBanana2,
		Quality:      pricingcatalog.ImageQuality1K,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	invalid := snapshot
	invalid.Source = ""
	if _, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "prepared-invalid-pricing-snapshot",
		CostEstimateCredits: snapshot.InternalCredits,
		PricingSnapshot:     invalid,
	}); err == nil {
		t.Fatal("invalid nonzero pricing snapshot must be rejected")
	}

	zeroSnapshot, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           uuid.New(),
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      "prepared-omitted-pricing-snapshot",
		CostEstimateCredits: 1,
	})
	if err != nil || len(zeroSnapshot.PricingSnapshot) != 0 {
		t.Fatalf("omitted snapshot = %+v, %v; want no stored snapshot", zeroSnapshot, err)
	}

	in := joborchestrator.PrepareAccountJobInput{
		AccountID:       uuid.New(),
		Operation:       domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		IdempotencyKey:  "prepared-pricing-snapshot-replay",
		PricingSnapshot: snapshot,
	}
	if _, err := f.orch.PrepareAccountJob(ctx, in); err != nil {
		t.Fatalf("prepare valid snapshot: %v", err)
	}
	in.PricingSnapshot.Source = "alternate-catalog"
	if _, err := f.orch.PrepareAccountJob(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed valid snapshot replay error = %v, want ErrConflict", err)
	}
}

func TestPrepareAccountJobConcurrentReplayCreatesOneRecordAndEvent(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	in := joborchestrator.PrepareAccountJobInput{
		AccountID:           uuid.New(),
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:prepare:concurrent",
		CostEstimateCredits: 25,
	}
	start := make(chan struct{})
	results := make(chan *domain.Job, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			job, err := f.orch.PrepareAccountJob(ctx, in)
			results <- job
			errs <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	if err := <-errs; err != nil {
		t.Fatalf("first concurrent prepare: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("second concurrent prepare: %v", err)
	}
	if first.ID == uuid.Nil || first.ID != second.ID {
		t.Fatalf("concurrent jobs = %v and %v, want one id", first, second)
	}
	jobs, err := f.jobs.ListByAccount(ctx, in.AccountID, 10, 0)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("account jobs = %+v, %v; want exactly one", jobs, err)
	}
	if events := f.outbox.Events(); len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("concurrent prepare events = %+v", events)
	}
}

func TestPrepareAccountJobRequiresExactlyOwnedInputArtifact(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	accountID := uuid.New()
	foreignID := seedInputArtifactForAccount(t, f, uuid.Nil, uuid.New(), domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/foreign-native.png")
	ownerlessID := seedInputArtifactForAccount(t, f, uuid.Nil, uuid.Nil, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "artifacts", "refs/ownerless.png")

	for _, artifactID := range []uuid.UUID{foreignID, ownerlessID} {
		_, err := f.orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
			AccountID:           accountID,
			Operation:           domain.OperationImageGenerate,
			Modality:            domain.ModalityImage,
			IdempotencyKey:      "web:prepare:artifact:" + artifactID.String(),
			InputArtifactIDs:    []uuid.UUID{artifactID},
			CostEstimateCredits: 25,
		})
		if !errors.Is(err, joborchestrator.ErrInvalidInputArtifact) {
			t.Fatalf("artifact %s error = %v, want ErrInvalidInputArtifact", artifactID, err)
		}
	}
}

func seedInputArtifact(t *testing.T, f *fixture, ownerID uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, status domain.ArtifactStatus, bucket, key string) uuid.UUID {
	return seedInputArtifactForAccount(t, f, ownerID, uuid.Nil, kind, mediaType, status, bucket, key)
}

func seedInputArtifactForAccount(t *testing.T, f *fixture, ownerUserID, ownerAccountID uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, status domain.ArtifactStatus, bucket, key string) uuid.UUID {
	t.Helper()
	artifact := &domain.Artifact{
		ID:             uuid.New(),
		OwnerUserID:    ownerUserID,
		OwnerAccountID: ownerAccountID,
		Kind:           kind,
		MediaType:      mediaType,
		MimeType:       "image/png",
		Status:         status,
		StorageBucket:  bucket,
		StorageKey:     key,
	}
	if err := f.arts.Create(context.Background(), artifact); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	return artifact.ID
}

func assertNoJobReservationOrTask(t *testing.T, f *fixture, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	jobs, listErr := f.jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if listErr != nil {
		t.Fatalf("list jobs: %v", listErr)
	}
	if len(jobs) != 0 {
		t.Fatalf("rejection persisted jobs: %+v", jobs)
	}
	if _, err := f.bill.GetAccountByUser(ctx, userID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("rejection should not create billing account/reservation, err=%v", err)
	}
	if events := f.outbox.Events(); len(events) != 0 {
		t.Fatalf("rejection wrote outbox events: %+v", events)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatalf("rejection enqueued tasks, got %d", f.pub.Len())
	}
}

func newRouteCatalogForOrchestratorTest(t *testing.T) *videorouter.Catalog {
	t.Helper()
	catalog, err := videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: true,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderPoYo: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  true,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
		},
		EnabledRoutes: map[domain.VideoRouteAlias]bool{
			domain.VideoRouteKlingO3Standard: true,
		},
	})
	if err != nil {
		t.Fatalf("new route catalog: %v", err)
	}
	return catalog
}

func routeResolverForTest(catalog *videorouter.Catalog) joborchestrator.VideoRouteResolver {
	return joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
		resolution, err := catalog.Resolve(ctx, videorouter.Request{
			Source:           in.Source,
			Operation:        in.Operation,
			Modality:         in.Modality,
			Params:           in.Params,
			InputArtifactIDs: in.InputArtifactIDs,
		})
		if err != nil {
			return joborchestrator.VideoRouteResolution{}, err
		}
		return joborchestrator.VideoRouteResolution{
			Resolved:            resolution.Resolved,
			Params:              resolution.Params,
			Snapshot:            resolution.Snapshot,
			InternalCostCredits: resolution.InternalCostCredits,
		}, nil
	})
}
