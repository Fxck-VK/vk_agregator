package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/deepinfra"
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
		Jobs:         jobs,
		Tasks:        tasks,
		Artifacts:    artSvc,
		ArtifactRepo: artRepo,
		Objects:      store,
		Providers:    worker.NewRegistry(provider),
		Streams:      streams,
		TextContext:  textContext,
		Releaser:     releaser,
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

type fakeVideoProber struct {
	metadata domain.ArtifactMediaMetadata
	err      error
	calls    int
}

func (p *fakeVideoProber) ProbeVideo(_ context.Context, _ []byte, _ int64) (domain.ArtifactMediaMetadata, error) {
	p.calls++
	if p.err != nil {
		return p.metadata, p.err
	}
	if p.metadata.ProbeStatus == "" {
		p.metadata.ProbeStatus = domain.MediaProbePassed
	}
	return p.metadata, nil
}

type fakeVideoTranscoder struct {
	out      []byte
	metadata domain.ArtifactMediaMetadata
	err      error
	calls    int
}

func (t *fakeVideoTranscoder) TranscodeVKVideo(_ context.Context, _ []byte, _ domain.ArtifactMediaMetadata) ([]byte, domain.ArtifactMediaMetadata, error) {
	t.calls++
	if t.err != nil {
		return nil, t.metadata, t.err
	}
	if t.metadata.ProbeStatus == "" {
		t.metadata.ProbeStatus = domain.MediaProbePending
	}
	return append([]byte(nil), t.out...), t.metadata, nil
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

func (h *harness) createInputImageArtifact(t *testing.T, ownerID uuid.UUID, data []byte, mime string) *domain.Artifact {
	t.Helper()
	if mime == "" {
		mime = "image/png"
	}
	artifact := &domain.Artifact{
		OwnerUserID:   ownerID,
		Kind:          domain.ArtifactKindInput,
		MediaType:     domain.MediaTypeImage,
		MimeType:      mime,
		StorageBucket: "artifacts",
		StorageKey:    "inputs/" + uuid.NewString() + ".png",
		SHA256:        uuid.NewString(),
		SizeBytes:     int64(len(data)),
		Status:        domain.ArtifactStatusReady,
	}
	if err := h.store.Put(context.Background(), artifact.StorageBucket, artifact.StorageKey, data, mime); err != nil {
		t.Fatalf("put input artifact bytes: %v", err)
	}
	if err := h.artRepo.Create(context.Background(), artifact); err != nil {
		t.Fatalf("create input artifact: %v", err)
	}
	return artifact
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

func TestVideoProbeSuccessBeforeDelivery(t *testing.T) {
	prober := &fakeVideoProber{metadata: domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "H264",
		Container:   "MP4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	}}
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoProber = prober
		d.RequireVideoProbe = true
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "a safe clip")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if prober.calls != 1 {
		t.Fatalf("probe calls = %d, want 1", prober.calls)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue, got %v", h.streams.byStream)
	}
	if len(got.OutputArtifactIDs) != 1 {
		t.Fatalf("expected one output artifact, got %d", len(got.OutputArtifactIDs))
	}
	art, err := h.artRepo.GetByID(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if art.ProbeStatus != domain.MediaProbePassed || art.Codec != "h264" || art.Container != "mp4" {
		t.Fatalf("probe metadata not stored safely: %+v", art)
	}
	if art.Width != 1280 || art.Height != 720 || art.DurationMS != 5000 || art.BitrateBPS != 2400000 {
		t.Fatalf("probe numeric metadata not stored: %+v", art)
	}
}

func TestVideoTranscodeCreatesVKReadyVariantBeforeDelivery(t *testing.T) {
	prober := &fakeVideoProber{metadata: domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "H264",
		Container:   "MP4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	}}
	transcoder := &fakeVideoTranscoder{
		out: []byte("vk-ready-video"),
		metadata: domain.ArtifactMediaMetadata{
			Codec:       "h264",
			Container:   "mp4",
			ProbeStatus: domain.MediaProbePending,
		},
	}
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoProber = prober
		d.VideoTranscoder = transcoder
		d.RequireVideoProbe = true
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "a safe clip")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if prober.calls != 2 {
		t.Fatalf("probe calls = %d, want original plus variant probes", prober.calls)
	}
	if transcoder.calls != 1 {
		t.Fatalf("transcode calls = %d, want 1", transcoder.calls)
	}
	variants, err := h.artRepo.ListVariants(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 1 || variants[0].VariantType != domain.VariantVKVideo {
		t.Fatalf("expected one vk_video variant, got %+v", variants)
	}
	if variants[0].Codec != "h264" || variants[0].Container != "mp4" || variants[0].ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("variant metadata not stored safely: %+v", variants[0])
	}
	data, err := h.store.GetObject(ctx, variants[0].StorageBucket, variants[0].StorageKey)
	if err != nil {
		t.Fatalf("get variant object: %v", err)
	}
	if string(data) != "vk-ready-video" {
		t.Fatalf("variant bytes = %q", string(data))
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue after variant creation, got %v", h.streams.byStream)
	}
}

func TestVideoProbeFailureStopsDeliveryAndReleasesReservation(t *testing.T) {
	prober := &fakeVideoProber{
		metadata: domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbeFailed},
		err:      errors.New("LEAK_PATH_MARKER LEAK_AUTH_MARKER LEAK_STDERR_MARKER"),
	}
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoProber = prober
		d.RequireVideoProbe = true
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "unsafe clip")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != string(domain.ProviderErrMediaProbeFailed) {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.ProviderErrMediaProbeFailed)
	}
	for _, forbidden := range []string{"LEAK_PATH_MARKER", "LEAK_AUTH_MARKER", "LEAK_STDERR_MARKER"} {
		if strings.Contains(got.ErrorMessage, forbidden) {
			t.Fatalf("job error leaked %q in %q", forbidden, got.ErrorMessage)
		}
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("probe failure must not enqueue delivery: %v", h.streams.byStream)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for failed probe, got %v", h.releaser.released)
	}
	art, err := h.artRepo.GetByID(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if art.ProbeStatus != domain.MediaProbeFailed || art.Status != domain.ArtifactStatusFailed {
		t.Fatalf("artifact should be marked failed after probe failure: %+v", art)
	}
}

func TestVideoTranscodeFailureStopsDeliveryAndReleasesReservation(t *testing.T) {
	prober := &fakeVideoProber{metadata: domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "H264",
		Container:   "MP4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	}}
	transcoder := &fakeVideoTranscoder{
		err: errors.New("LEAK_PATH_MARKER LEAK_AUTH_MARKER LEAK_STDERR_MARKER"),
	}
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoProber = prober
		d.VideoTranscoder = transcoder
		d.RequireVideoProbe = true
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "unsafe transcode")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != string(domain.ProviderErrMediaTranscodeFailed) {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.ProviderErrMediaTranscodeFailed)
	}
	for _, forbidden := range []string{"LEAK_PATH_MARKER", "LEAK_AUTH_MARKER", "LEAK_STDERR_MARKER"} {
		if strings.Contains(got.ErrorMessage, forbidden) {
			t.Fatalf("job error leaked %q in %q", forbidden, got.ErrorMessage)
		}
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("transcode failure must not enqueue delivery: %v", h.streams.byStream)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for failed transcode, got %v", h.releaser.released)
	}
	variants, err := h.artRepo.ListVariants(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("transcode failure must not store variants: %+v", variants)
	}
	art, err := h.artRepo.GetByID(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if art.ProbeStatus != domain.MediaProbeFailed || art.Status != domain.ArtifactStatusFailed {
		t.Fatalf("artifact should be marked failed after transcode failure: %+v", art)
	}
}

func TestVideoProbeRequiredWithoutProberFailsClosed(t *testing.T) {
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.RequireVideoProbe = true
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "needs probe")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("missing required prober must not enqueue delivery: %v", h.streams.byStream)
	}
	if got.ErrorCode != string(domain.ProviderErrMediaProbeFailed) {
		t.Fatalf("error code = %q", got.ErrorCode)
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
	rawParams, _ := json.Marshal(map[string]string{
		"prompt":              "hi",
		"conversation_source": "miniapp",
		"external_thread_id":  "thread-a",
	})
	job.Params = rawParams
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
		ConversationID     string `json:"conversation_id"`
		ConversationSource string `json:"conversation_source"`
		ExternalThreadID   string `json:"external_thread_id"`
	}
	if err := json.Unmarshal(got.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.ConversationID != textCtx.conversationID.String() {
		t.Fatalf("conversation id = %q, want %s", params.ConversationID, textCtx.conversationID)
	}
	if params.ConversationSource != "miniapp" || params.ExternalThreadID != "thread-a" {
		t.Fatalf("conversation ref was not preserved: %+v", params)
	}
}

// Terminal error classification: a non-retryable failure ends the job.
func TestTerminalProviderError(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// Unsupported operation -> unsupported_capability (terminal) on Submit.
	job := h.queueJob(t, domain.OperationImageEdit, domain.ModalityImage, "edit this")
	job.VKPeerID = 555
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job peer: %v", err)
	}

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
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected one failure delivery enqueue, got %v", h.streams.byStream)
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

func TestProviderRegistryPrefersImageProvider(t *testing.T) {
	defaultProvider := &routingProvider{
		name:      domain.ProviderName("default"),
		cost:      1,
		operation: domain.OperationImageGenerate,
		modality:  domain.ModalityImage,
		model:     "default-image",
	}
	imageProvider := &routingProvider{
		name:      domain.ProviderName("image"),
		cost:      100,
		operation: domain.OperationImageGenerate,
		modality:  domain.ModalityImage,
		model:     "preferred-image",
	}
	reg := worker.NewRegistry(defaultProvider, imageProvider)
	reg.PreferProvider(domain.ModalityImage, imageProvider.name)
	req := domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "cat",
	}

	provider, err := reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("for request: %v", err)
	}
	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit through router: %v", err)
	}
	if task.Provider != imageProvider.name {
		t.Fatalf("task provider = %q, want %q", task.Provider, imageProvider.name)
	}
}

func TestProviderRegistryPrefersDeepInfraImageProvider(t *testing.T) {
	deepInfraProvider := deepinfra.New(deepinfra.Config{
		APIKey:     "test-key",
		ImageModel: "ByteDance/Seedream-4.5",
		ImagePrice: 99,
	})
	reg := worker.NewRegistry(mock.New(), deepInfraProvider)
	reg.PreferProvider(domain.ModalityImage, domain.ProviderDeepInfra)

	provider, err := reg.ForRequest(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		ModelCode: "ByteDance/Seedream-4.5",
		Prompt:    "a cat",
	})
	if err != nil {
		t.Fatalf("for request: %v", err)
	}
	if provider.Name() != domain.ProviderDeepInfra {
		t.Fatalf("provider = %q, want deepinfra", provider.Name())
	}
}

func TestGenerationImageRequestCarriesImageDefaultsAndReferences(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ImageModel = "foundation-image"
		d.ImageSize = "1024x1024"
	})
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, "image/png")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "a cat",
		"aspect_ratio":           "1:1",
		"reference_artifact_ids": []string{reference.ID.String()},
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := provider.last
	if got.UserID != job.UserID {
		t.Fatalf("provider request user id = %s, want %s", got.UserID, job.UserID)
	}
	if got.ModelCode != "foundation-image" || got.Size != "1024x1024" || got.AspectRatio != "1:1" {
		t.Fatalf("unexpected image request defaults: model=%q size=%q aspect=%q", got.ModelCode, got.Size, got.AspectRatio)
	}
	if len(got.ReferenceArtifactIDs) != 1 || got.ReferenceArtifactIDs[0] != reference.ID {
		t.Fatalf("reference ids = %v, want %s", got.ReferenceArtifactIDs, reference.ID)
	}
	if len(got.InputURLs) != 1 || !strings.HasPrefix(got.InputURLs[0], "data:image/png;base64,") {
		t.Fatalf("input urls were not resolved from reference artifact: %v", got.InputURLs)
	}
}

func TestBuildRequest_ResolvesReferenceInputURLs(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, "image/png")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "use this reference",
		"reference_artifact_ids": []string{reference.ID.String()},
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	if len(provider.last.InputURLs) != 1 || !strings.HasPrefix(provider.last.InputURLs[0], "data:image/") {
		t.Fatalf("input urls = %v, want one image data URL", provider.last.InputURLs)
	}
	stored, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("get stored job: %v", err)
	}
	if strings.Contains(string(stored.Params), "base64") || strings.Contains(string(stored.Params), "data:") {
		t.Fatalf("job params must not persist reference bytes: %s", string(stored.Params))
	}
	tasks, err := h.tasks.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list provider tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected provider task")
	}
	if strings.Contains(string(tasks[0].Request), "base64") || strings.Contains(string(tasks[0].Request), "data:") {
		t.Fatalf("provider task request must not persist reference bytes: %s", string(tasks[0].Request))
	}
}

func TestBuildRequest_ResolveReferenceRejectsForeignOwner(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	reference := h.createInputImageArtifact(t, uuid.New(), []byte("foreign"), "image/png")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "use foreign reference",
		"reference_artifact_ids": []string{reference.ID.String()},
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	err := h.gen.Process(ctx, taskFor(job))
	if err == nil || !strings.Contains(err.Error(), "invalid reference artifact owner") {
		t.Fatalf("expected foreign owner error, got %v", err)
	}
	if provider.last.JobID != uuid.Nil {
		t.Fatalf("provider must not be called for invalid reference, got request %+v", provider.last)
	}
}

func TestBuildRequest_ResolveReferenceRejectsTooMany(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		ids = append(ids, uuid.NewString())
	}
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "too many references",
		"reference_artifact_ids": ids,
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	err := h.gen.Process(ctx, taskFor(job))
	if err == nil || !strings.Contains(err.Error(), "too many reference artifacts") {
		t.Fatalf("expected too many references error, got %v", err)
	}
	if provider.last.JobID != uuid.Nil {
		t.Fatalf("provider must not be called for too many references, got request %+v", provider.last)
	}
}

type routingProvider struct {
	name      domain.ProviderName
	cost      int64
	fail      error
	submits   int
	operation domain.OperationType
	modality  domain.Modality
	model     string
}

func (p *routingProvider) Name() domain.ProviderName { return p.name }

func (p *routingProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	op := p.operation
	if op == "" {
		op = domain.OperationTextGenerate
	}
	mod := p.modality
	if mod == "" {
		mod = domain.ModalityText
	}
	model := p.model
	if model == "" {
		model = string(p.name) + "-model"
	}
	return []domain.Capability{{Operation: op, Modality: mod, ModelCode: model}}, nil
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
		ModelCode:  p.model,
		ExternalID: string(p.name) + "-task",
		Status:     domain.ProviderTaskSucceeded,
	}, nil
}

func (p *routingProvider) Poll(context.Context, domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	return domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, OutputURLs: []string{"data:text/plain;base64,b2s="}}, nil
}

func (p *routingProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }

type captureImageProvider struct {
	last domain.ProviderRequest
}

func (p *captureImageProvider) Name() domain.ProviderName {
	return domain.ProviderName("capture-image")
}

func (p *captureImageProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, SupportsPolling: true}}, nil
}

func (p *captureImageProvider) Estimate(context.Context, domain.ProviderRequest) (domain.CostEstimate, error) {
	return domain.CostEstimate{AmountCredits: 10, Currency: "credits"}, nil
}

func (p *captureImageProvider) Submit(_ context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	p.last = req
	return domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       p.Name(),
		ModelCode:      req.ModelCode,
		ExternalID:     "capture-image-task",
		Status:         domain.ProviderTaskSucceeded,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func (p *captureImageProvider) Poll(context.Context, domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	return domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, OutputURLs: []string{"data:image/png;base64,b2s="}}, nil
}

func (p *captureImageProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }

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
