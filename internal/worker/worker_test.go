package worker_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/mock"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/worker"
)

// fakeStreams records tasks published per stream.
type fakeStreams struct {
	byStream map[string][]queue.Task
}

func newFakeStreams() *fakeStreams { return &fakeStreams{byStream: map[string][]queue.Task{}} }

func (f *fakeStreams) PublishTo(_ context.Context, stream string, task queue.Task) error {
	f.byStream[stream] = append(f.byStream[stream], task)
	return nil
}

// harness wires the in-memory adapters, a mock provider and the workers.
type harness struct {
	jobs     *memory.JobRepo
	tasks    *memory.ProviderTaskRepo
	artRepo  *memory.ArtifactRepo
	store    *memory.ObjectStore
	streams  *fakeStreams
	provider domain.Provider
	releaser *fakeReleaser
	gen      *worker.GenerationWorker
	poll     *worker.PollWorker
}

func newHarness(t *testing.T, opts ...mock.Option) *harness {
	t.Helper()
	return newHarnessWithProvider(t, mock.New(opts...), nil)
}

func newHarnessWithProvider(t *testing.T, provider domain.Provider, configure func(*worker.Deps)) *harness {
	t.Helper()
	return newHarnessCore(t, provider, nil, configure)
}

func newHarnessWithTextContext(t *testing.T, textContext worker.TextContext, opts ...mock.Option) *harness {
	t.Helper()
	return newHarnessCore(t, mock.New(opts...), textContext, nil)
}

func newHarnessCore(t *testing.T, provider domain.Provider, textContext worker.TextContext, configure func(*worker.Deps)) *harness {
	t.Helper()
	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	artRepo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	// Download anything to fixed bytes so SaveRemoteArtifact succeeds offline.
	dl := stubDownloader{data: []byte("output"), contentType: "application/octet-stream"}
	artSvc := artifactservice.New(artRepo, store, "artifacts", artifactservice.WithDownloader(dl))
	streams := newFakeStreams()
	releaser := &fakeReleaser{}
	deps := worker.Deps{
		Jobs:        jobs,
		Tasks:       tasks,
		Artifacts:   artSvc,
		Providers:   worker.NewRegistry(provider),
		Streams:     streams,
		TextContext: textContext,
		Releaser:    releaser,
	}
	if configure != nil {
		configure(&deps)
	}
	return &harness{
		jobs:     jobs,
		tasks:    tasks,
		artRepo:  artRepo,
		store:    store,
		streams:  streams,
		provider: provider,
		releaser: releaser,
		gen:      worker.NewGenerationWorker(deps),
		poll:     worker.NewPollWorker(deps),
	}
}

type fakeTextContext struct {
	preparedPrompt  string
	prepareCalls    int
	completeCalls   int
	completedAnswer string
	conversationID  uuid.UUID
}

func (f *fakeTextContext) Prepare(_ context.Context, _ *domain.Job, prompt string) (dialogcontext.Prepared, error) {
	f.prepareCalls++
	if f.conversationID == uuid.Nil {
		f.conversationID = uuid.New()
	}
	return dialogcontext.Prepared{
		ConversationID:  f.conversationID,
		Prompt:          f.preparedPrompt + prompt,
		MaxOutputTokens: 800,
	}, nil
}

func (f *fakeTextContext) Complete(_ context.Context, _ *domain.Job, conversationID uuid.UUID, answer string) error {
	f.completeCalls++
	if conversationID != f.conversationID {
		return nil
	}
	f.completedAnswer = answer
	return nil
}

type stubDownloader struct {
	data        []byte
	contentType string
}

func (d stubDownloader) Download(_ context.Context, _ string) ([]byte, string, error) {
	return d.data, d.contentType, nil
}

// queueJob inserts a queued job and returns it.
func (h *harness) queueJob(t *testing.T, op domain.OperationType, mod domain.Modality, prompt string) *domain.Job {
	t.Helper()
	params, _ := json.Marshal(map[string]string{"prompt": prompt})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OperationType:  op,
		Modality:       mod,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         params,
	}
	if err := h.jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job
}

func (h *harness) reload(t *testing.T, id uuid.UUID) *domain.Job {
	t.Helper()
	j, err := h.jobs.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	return j
}

func taskFor(job *domain.Job) queue.Task {
	return queue.Task{JobID: job.ID, Operation: job.OperationType, Modality: job.Modality}
}

// Synchronous mock (completes after 1 poll): generation worker finishes the
// whole flow and enqueues delivery.
func TestGenerationSyncSuccess(t *testing.T) {
	h := newHarness(t) // completeAfterPolls default 1
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationImageGenerate, domain.ModalityImage, "a cat")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if len(got.OutputArtifactIDs) != 1 {
		t.Fatalf("expected 1 output artifact, got %d", len(got.OutputArtifactIDs))
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected one delivery enqueue, got %v", h.streams.byStream)
	}
	tasks, _ := h.tasks.ListByJob(ctx, job.ID)
	if len(tasks) != 1 || tasks[0].Status != domain.ProviderTaskSucceeded {
		t.Fatalf("unexpected provider tasks: %+v", tasks)
	}
}

// Asynchronous mock (needs 2 polls): generation submits and requeues to the
// poll stream; the poll worker then completes the flow.
func TestAsyncFlowViaPollWorker(t *testing.T) {
	h := newHarness(t, mock.WithCompleteAfterPolls(2))
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "a clip")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("gen process: %v", err)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusProviderProcessing {
		t.Fatalf("after gen status = %q, want provider_processing", got.Status)
	}
	pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
	if len(pollTasks) != 1 {
		t.Fatalf("expected one poll enqueue, got %d", len(pollTasks))
	}

	if err := h.poll.Process(ctx, pollTasks[0]); err != nil {
		t.Fatalf("poll process: %v", err)
	}
	got = h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("after poll status = %q, want result_ready", got.Status)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue after poll")
	}
}

// Idempotency: re-delivering the same generation task must not submit twice.
func TestGenerationIdempotentRedelivery(t *testing.T) {
	h := newHarness(t, mock.WithCompleteAfterPolls(2))
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "hi")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("gen 1: %v", err)
	}
	// Re-deliver: the job already has an in-flight task, so no new submission.
	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("gen 2: %v", err)
	}
	tasks, _ := h.tasks.ListByJob(ctx, job.ID)
	if len(tasks) != 1 {
		t.Fatalf("expected exactly one provider task after redelivery, got %d", len(tasks))
	}
}

func TestGenerationTextUsesDialogContext(t *testing.T) {
	textCtx := &fakeTextContext{preparedPrompt: "context packet\n"}
	h := newHarnessWithTextContext(t, textCtx)
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "hi")
	job.VKPeerID = 555
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update peer: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if textCtx.prepareCalls != 1 {
		t.Fatalf("prepare calls = %d, want 1", textCtx.prepareCalls)
	}
	if textCtx.completeCalls != 1 || !strings.Contains(textCtx.completedAnswer, "Mock generated text result") {
		t.Fatalf("complete calls=%d answer=%q", textCtx.completeCalls, textCtx.completedAnswer)
	}
	got := h.reload(t, job.ID)
	var params struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(got.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.ConversationID != textCtx.conversationID.String() {
		t.Fatalf("conversation id = %q, want %s", params.ConversationID, textCtx.conversationID)
	}
}

// Terminal error classification: a non-retryable failure ends the job.
func TestTerminalProviderError(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// Unsupported operation -> unsupported_capability (terminal) on Submit.
	job := h.queueJob(t, domain.OperationImageEdit, domain.ModalityImage, "edit this")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != string(domain.ProviderErrUnsupportedCapab) {
		t.Fatalf("error code = %q", got.ErrorCode)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for terminal failure, got %v", h.releaser.released)
	}
}

// Retryable error classification: a retryable failure re-queues the job for
// another attempt, and is capped so it eventually goes terminal.
func TestRetryableProviderErrorRequeues(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "please "+mock.TriggerRateLimit)

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusQueued {
		t.Fatalf("status = %q, want queued (re-enqueued)", got.Status)
	}
	if n := len(h.streams.byStream[redisqueue.StreamVideo]); n != 1 {
		t.Fatalf("expected re-enqueue to video stream, got %d", n)
	}
	if got.ErrorCode != string(domain.ProviderErrRateLimited) {
		t.Fatalf("error code = %q, want rate_limited", got.ErrorCode)
	}
}

func TestRetryableErrorBecomesTerminalAfterMaxAttempts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, mock.TriggerProviderError)

	// Drive several attempts; each retryable failure re-queues until the cap.
	for i := 0; i < 5; i++ {
		if err := h.gen.Process(ctx, taskFor(job)); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if h.reload(t, job.ID).Status == domain.JobStatusFailedTerminal {
			break
		}
	}
	if got := h.reload(t, job.ID); got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal after max attempts", got.Status)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected one reservation release after exhausted attempts, got %v", h.releaser.released)
	}
}

func TestProviderSubmitTimeoutBecomesTerminalAndReleasesReservation(t *testing.T) {
	h := newHarnessWithProvider(t, &timeoutProvider{name: domain.ProviderName("timeout")}, func(d *worker.Deps) {
		d.MaxAttempts = 1
		d.ProviderCallTimeout = time.Millisecond
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "slow")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != string(domain.ProviderErrTimeout) {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.ProviderErrTimeout)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for timed out job, got %v", h.releaser.released)
	}
}

func TestProcessUnknownJobIsAcked(t *testing.T) {
	h := newHarness(t)
	err := h.gen.Process(context.Background(), queue.Task{JobID: uuid.New(), Operation: domain.OperationTextGenerate, Modality: domain.ModalityText})
	if err != nil {
		t.Fatalf("unknown job should be a no-op ack, got %v", err)
	}
}

func TestProviderRegistryFallbackOnRetryableSubmit(t *testing.T) {
	primary := &routingProvider{
		name: domain.ProviderName("primary"),
		cost: 1,
		fail: routingError{class: domain.ProviderErrRateLimited},
	}
	fallback := &routingProvider{name: domain.ProviderName("fallback"), cost: 10}
	reg := worker.NewRegistry(primary, fallback)
	req := domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationTextGenerate,
		Modality:  domain.ModalityText,
		Prompt:    "hello",
	}

	provider, err := reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("for request: %v", err)
	}
	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit through router: %v", err)
	}
	if task.Provider != fallback.name {
		t.Fatalf("task provider = %q, want fallback", task.Provider)
	}
	if primary.submits != 1 || fallback.submits != 1 {
		t.Fatalf("submits primary=%d fallback=%d", primary.submits, fallback.submits)
	}
}

type routingProvider struct {
	name    domain.ProviderName
	cost    int64
	fail    error
	submits int
}

func (p *routingProvider) Name() domain.ProviderName { return p.name }

func (p *routingProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, ModelCode: string(p.name) + "-model"}}, nil
}

func (p *routingProvider) Estimate(context.Context, domain.ProviderRequest) (domain.CostEstimate, error) {
	return domain.CostEstimate{AmountCredits: p.cost, Currency: "credits"}, nil
}

func (p *routingProvider) Submit(_ context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	p.submits++
	if p.fail != nil {
		return domain.ProviderTask{}, p.fail
	}
	return domain.ProviderTask{
		JobID:      req.JobID,
		Provider:   p.name,
		ModelCode:  string(p.name) + "-model",
		ExternalID: string(p.name) + "-task",
		Status:     domain.ProviderTaskSucceeded,
	}, nil
}

func (p *routingProvider) Poll(context.Context, domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	return domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, OutputURLs: []string{"data:text/plain;base64,b2s="}}, nil
}

func (p *routingProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }

type routingError struct{ class domain.ProviderErrorClass }

func (e routingError) Error() string { return string(e.class) }

func (e routingError) ProviderErrorClass() domain.ProviderErrorClass { return e.class }

type timeoutProvider struct {
	name domain.ProviderName
}

func (p *timeoutProvider) Name() domain.ProviderName { return p.name }

func (p *timeoutProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{{
		Operation: domain.OperationTextGenerate,
		Modality:  domain.ModalityText,
		ModelCode: string(p.name) + "-model",
	}}, nil
}

func (p *timeoutProvider) Estimate(context.Context, domain.ProviderRequest) (domain.CostEstimate, error) {
	return domain.CostEstimate{AmountCredits: 10, Currency: "credits"}, nil
}

func (p *timeoutProvider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	<-ctx.Done()
	return domain.ProviderTask{JobID: req.JobID, Provider: p.name}, ctx.Err()
}

func (p *timeoutProvider) Poll(ctx context.Context, _ domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	<-ctx.Done()
	return domain.ProviderTaskResult{}, ctx.Err()
}

func (p *timeoutProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }
