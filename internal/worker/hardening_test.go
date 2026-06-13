package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/mock"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/worker"
)

// fakeReleaser records reservation releases.
type fakeReleaser struct{ released []uuid.UUID }

func (r *fakeReleaser) ReleaseForJob(_ context.Context, jobID uuid.UUID) error {
	r.released = append(r.released, jobID)
	return nil
}

func moderationHarness(t *testing.T, mod worker.Moderator, modResults domain.ModerationResultRepository, rel worker.ReservationReleaser) (*worker.GenerationWorker, *memory.JobRepo, *fakeStreams) {
	t.Helper()
	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	artRepo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	dl := stubDownloader{data: []byte("output"), contentType: "application/octet-stream"}
	artSvc := artifactservice.New(artRepo, store, "artifacts", artifactservice.WithDownloader(dl))
	streams := newFakeStreams()
	deps := worker.Deps{
		Jobs:       jobs,
		Tasks:      tasks,
		Artifacts:  artSvc,
		Providers:  worker.NewRegistry(mock.New()),
		Streams:    streams,
		Moderator:  mod,
		ModResults: modResults,
		Releaser:   rel,
	}
	return worker.NewGenerationWorker(deps), jobs, streams
}

func insertQueuedJob(t *testing.T, jobs *memory.JobRepo, op domain.OperationType, modality domain.Modality, prompt string) *domain.Job {
	t.Helper()
	params, _ := json.Marshal(map[string]string{"prompt": prompt})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OperationType:  op,
		Modality:       modality,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
		Params:         params,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job
}

// A blocked output must be rejected: no delivery is enqueued, the reservation is
// released (so no capture happens) and an audit verdict is recorded.
func TestOutputModerationBlocksDelivery(t *testing.T) {
	ctx := context.Background()
	modRepo := memory.NewModerationRepo()
	rel := &fakeReleaser{}
	gen, jobs, streams := moderationHarness(t, moderationservice.NewKeywordModerator(), modRepo, rel)

	job := insertQueuedJob(t, jobs, domain.OperationImageGenerate, domain.ModalityImage, "please render nsfw content")
	if err := gen.Process(ctx, queue.Task{JobID: job.ID, Operation: job.OperationType, Modality: job.Modality}); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusRejected {
		t.Fatalf("status = %q, want rejected", got.Status)
	}
	if n := len(streams.byStream[redisqueue.StreamDelivery]); n != 0 {
		t.Fatalf("expected no delivery enqueue for blocked output, got %d", n)
	}
	if len(rel.released) != 1 || rel.released[0] != job.ID {
		t.Fatalf("expected reservation release for blocked job, got %v", rel.released)
	}
	results, _ := modRepo.ListByJob(ctx, job.ID)
	if len(results) != 1 || results[0].Decision != domain.ModerationBlock {
		t.Fatalf("expected one block verdict, got %+v", results)
	}
}

func TestOutputModerationBlocksGeneratedTextBeforeDialogSave(t *testing.T) {
	ctx := context.Background()
	textCtx := &fakeTextContext{preparedPrompt: "context packet\n"}
	modRepo := memory.NewModerationRepo()
	h := newHarnessCore(t, mock.New(), textCtx, func(deps *worker.Deps) {
		deps.Moderator = moderationservice.NewKeywordModerator("mock generated text result")
		deps.ModResults = modRepo
	})

	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "clean prompt")
	rawParams, _ := json.Marshal(map[string]string{
		"prompt":              "clean prompt",
		"conversation_source": "miniapp",
		"external_thread_id":  "thread-a",
	})
	job.Params = rawParams
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job params: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusRejected {
		t.Fatalf("status = %q, want rejected", got.Status)
	}
	if textCtx.completeCalls != 0 {
		t.Fatalf("dialog answer was saved before output moderation allow: calls=%d answer=%q", textCtx.completeCalls, textCtx.completedAnswer)
	}
	if n := len(h.streams.byStream[redisqueue.StreamDelivery]); n != 0 {
		t.Fatalf("expected no delivery enqueue for blocked generated text, got %d", n)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for blocked job, got %v", h.releaser.released)
	}
	results, _ := modRepo.ListByJob(ctx, job.ID)
	if len(results) != 1 || results[0].Decision != domain.ModerationBlock {
		t.Fatalf("expected one generated-text block verdict, got %+v", results)
	}
}

// An allowed output proceeds to delivery as before.
func TestOutputModerationAllowsDelivery(t *testing.T) {
	ctx := context.Background()
	gen, jobs, streams := moderationHarness(t, moderationservice.NewKeywordModerator(), memory.NewModerationRepo(), &fakeReleaser{})

	job := insertQueuedJob(t, jobs, domain.OperationImageGenerate, domain.ModalityImage, "a friendly cat")
	if err := gen.Process(ctx, queue.Task{JobID: job.ID, Operation: job.OperationType, Modality: job.Modality}); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if n := len(streams.byStream[redisqueue.StreamDelivery]); n != 1 {
		t.Fatalf("expected one delivery enqueue, got %d", n)
	}
}

// A persistently failing retryable provider is dead-lettered once the retry
// budget is exhausted, instead of re-enqueuing forever.
func TestExhaustedRetryRoutesToDLQ(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, mock.TriggerProviderError)

	for i := 0; i < 5; i++ {
		if err := h.gen.Process(ctx, taskFor(job)); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if h.reload(t, job.ID).Status == domain.JobStatusFailedTerminal {
			break
		}
	}
	if got := h.reload(t, job.ID); got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if n := len(h.streams.byStream[redisqueue.StreamDLQ]); n == 0 {
		t.Fatalf("expected exhausted task routed to DLQ, got none")
	}
}

func TestGenerationWorkerResumesDurableSyncProviderResultAfterSubmitCrash(t *testing.T) {
	ctx := context.Background()
	provider := &durableSyncProvider{}
	var failingJobs *failStatusOnceJobRepo
	h := newHarnessCore(t, provider, nil, func(deps *worker.Deps) {
		failingJobs = &failStatusOnceJobRepo{
			JobRepository: deps.Jobs,
			failTo:        domain.JobStatusProviderSubmitted,
		}
		deps.Jobs = failingJobs
	})
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "clean prompt")

	err := h.gen.Process(ctx, taskFor(job))
	if err == nil || !errors.Is(err, errInjectedStatusCrash) {
		t.Fatalf("first process error = %v, want injected crash", err)
	}
	if got := h.reload(t, job.ID); got.Status != domain.JobStatusDispatchingProvider {
		t.Fatalf("status after injected crash = %q, want dispatching_provider", got.Status)
	}
	tasks, err := h.tasks.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list provider tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Status != domain.ProviderTaskSucceeded || len(tasks[0].Result) == 0 {
		t.Fatalf("provider task was not durably completed: %+v", tasks)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("retry process: %v", err)
	}
	if got := h.reload(t, job.ID); got.Status != domain.JobStatusResultReady {
		t.Fatalf("status after retry = %q, want result_ready", got.Status)
	}
	if provider.pollCalls != 0 {
		t.Fatalf("retry used provider Poll instead of durable result: calls=%d", provider.pollCalls)
	}
	if provider.submitCalls != 1 {
		t.Fatalf("provider Submit calls = %d, want 1", provider.submitCalls)
	}
	if n := len(h.streams.byStream[redisqueue.StreamDelivery]); n != 1 {
		t.Fatalf("delivery enqueue count = %d, want 1", n)
	}
}

var errInjectedStatusCrash = errors.New("injected status crash")

type failStatusOnceJobRepo struct {
	domain.JobRepository
	failTo domain.JobStatus
	failed bool
}

func (r *failStatusOnceJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domain.JobStatus, errCode, errMessage string) error {
	if !r.failed && to == r.failTo {
		r.failed = true
		return errInjectedStatusCrash
	}
	return r.JobRepository.UpdateStatus(ctx, id, from, to, errCode, errMessage)
}

type durableSyncProvider struct {
	submitCalls int
	pollCalls   int
}

func (p *durableSyncProvider) Name() domain.ProviderName { return domain.ProviderMock }

func (p *durableSyncProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{{
		Operation:       domain.OperationTextGenerate,
		Modality:        domain.ModalityText,
		ModelCode:       "durable-sync",
		SupportsPolling: true,
	}}, nil
}

func (p *durableSyncProvider) Estimate(context.Context, domain.ProviderRequest) (domain.CostEstimate, error) {
	return domain.CostEstimate{AmountCredits: 1, Currency: "credits"}, nil
}

func (p *durableSyncProvider) Submit(_ context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	p.submitCalls++
	res := domain.ProviderTaskResult{
		Status:     domain.ProviderTaskSucceeded,
		OutputURLs: []string{"data:text/plain;base64,b2s="},
		Text:       "ok",
	}
	raw, _ := json.Marshal(res)
	return domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderMock,
		ModelCode:      "durable-sync",
		ExternalID:     "durable-sync-" + req.JobID.String(),
		Status:         domain.ProviderTaskSucceeded,
		Result:         raw,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func (p *durableSyncProvider) Poll(context.Context, domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	p.pollCalls++
	return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: domain.ProviderErrTaskNotFound}, nil
}

func (p *durableSyncProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }
