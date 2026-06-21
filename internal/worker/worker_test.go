package worker_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/provider/deepinfra"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/provider/poyo"
	"vk-ai-aggregator/internal/adapter/provider/runway"
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
	mu       sync.Mutex
	metadata domain.ArtifactMediaMetadata
	sequence []domain.ArtifactMediaMetadata
	err      error
	calls    int
}

func (p *fakeVideoProber) ProbeVideo(_ context.Context, _ []byte, _ int64) (domain.ArtifactMediaMetadata, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if len(p.sequence) > 0 {
		metadata := p.sequence[0]
		p.sequence = p.sequence[1:]
		if p.err != nil {
			return metadata, p.err
		}
		if metadata.ProbeStatus == "" {
			metadata.ProbeStatus = domain.MediaProbePassed
		}
		return metadata, nil
	}
	if p.err != nil {
		return p.metadata, p.err
	}
	if p.metadata.ProbeStatus == "" {
		p.metadata.ProbeStatus = domain.MediaProbePassed
	}
	return p.metadata, nil
}

type fakeVideoTranscoder struct {
	mu       sync.Mutex
	out      []byte
	metadata domain.ArtifactMediaMetadata
	err      error
	calls    int
}

func (t *fakeVideoTranscoder) TranscodeVKVideo(_ context.Context, _ []byte, _ domain.ArtifactMediaMetadata) ([]byte, domain.ArtifactMediaMetadata, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	if t.err != nil {
		return nil, t.metadata, t.err
	}
	if t.metadata.ProbeStatus == "" {
		t.metadata.ProbeStatus = domain.MediaProbePending
	}
	return append([]byte(nil), t.out...), t.metadata, nil
}

type blockingVideoTranscoder struct {
	mu       sync.Mutex
	entered  chan struct{}
	release  chan struct{}
	out      []byte
	metadata domain.ArtifactMediaMetadata
	calls    int
}

func newBlockingVideoTranscoder(out []byte, metadata domain.ArtifactMediaMetadata) *blockingVideoTranscoder {
	return &blockingVideoTranscoder{
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
		out:      out,
		metadata: metadata,
	}
}

func (t *blockingVideoTranscoder) TranscodeVKVideo(ctx context.Context, _ []byte, _ domain.ArtifactMediaMetadata) ([]byte, domain.ArtifactMediaMetadata, error) {
	t.mu.Lock()
	t.calls++
	call := t.calls
	t.mu.Unlock()
	if call == 1 {
		close(t.entered)
		select {
		case <-t.release:
		case <-ctx.Done():
			return nil, t.metadata, ctx.Err()
		}
	}
	metadata := t.metadata
	if metadata.ProbeStatus == "" {
		metadata.ProbeStatus = domain.MediaProbePending
	}
	return append([]byte(nil), t.out...), metadata, nil
}

func (t *blockingVideoTranscoder) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
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

func (h *harness) queueVideoJob(t *testing.T, params map[string]any) *domain.Job {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   10,
		Params:         raw,
	}
	if err := h.jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("create video job: %v", err)
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

func validPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 120, G: 80, B: 40, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func jpegWithAPP1(t *testing.T, marker string) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	raw := buf.Bytes()
	if len(raw) < 2 || raw[0] != 0xff || raw[1] != 0xd8 {
		t.Fatal("test jpeg missing SOI")
	}
	payload := append([]byte("Exif\x00\x00"), []byte(marker)...)
	segment := []byte{0xff, 0xe1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	segment = append(segment, payload...)
	out := make([]byte, 0, len(raw)+len(segment))
	out = append(out, raw[:2]...)
	out = append(out, segment...)
	out = append(out, raw[2:]...)
	return out
}

func pngWithTextChunk(t *testing.T, keyword, text string) []byte {
	t.Helper()
	raw := validPNGBytes(t)
	const afterIHDR = 8 + 4 + 4 + 13 + 4
	if len(raw) <= afterIHDR {
		t.Fatal("test png too short")
	}
	payload := append([]byte(keyword), 0)
	payload = append(payload, []byte(text)...)
	var chunk bytes.Buffer
	if err := binary.Write(&chunk, binary.BigEndian, uint32(len(payload))); err != nil {
		t.Fatalf("write png chunk length: %v", err)
	}
	chunk.WriteString("tEXt")
	chunk.Write(payload)
	crc := crc32.ChecksumIEEE(append([]byte("tEXt"), payload...))
	if err := binary.Write(&chunk, binary.BigEndian, crc); err != nil {
		t.Fatalf("write png chunk crc: %v", err)
	}
	out := make([]byte, 0, len(raw)+chunk.Len())
	out = append(out, raw[:afterIHDR]...)
	out = append(out, chunk.Bytes()...)
	out = append(out, raw[afterIHDR:]...)
	return out
}

func minimalWebPBytes() []byte {
	return []byte("RIFF\x10\x00\x00\x00WEBPVP8 \x04\x00\x00\x00\x00\x00\x00\x00")
}

func dataURLBytes(t *testing.T, rawURL, expectedMIME string) []byte {
	t.Helper()
	prefix := "data:" + expectedMIME + ";base64,"
	if !strings.HasPrefix(rawURL, prefix) {
		t.Fatalf("data url prefix = %q, want %q", rawURL, prefix)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(rawURL, prefix))
	if err != nil {
		t.Fatalf("decode data url: %v", err)
	}
	return data
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

func TestVideoFastPathSkipsTranscodeForSafeProviderOutput(t *testing.T) {
	prober := &fakeVideoProber{metadata: safeVideoMetadata("h264")}
	transcoder := &fakeVideoTranscoder{out: []byte("should-not-run")}
	contract := validVideoContract(domain.ProviderMock, "mock-video")
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoModel = "mock-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.VideoProber = prober
		d.VideoTranscoder = transcoder
		d.RequireVideoProbe = true
		d.VideoTranscodeEnabled = true
		d.VideoTranscodePolicy = "fallback"
		d.RawVideoDeliveryPolicy = "if_probe_passed"
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "safe provider mp4")

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
	if transcoder.calls != 0 {
		t.Fatalf("transcoder calls = %d, want 0", transcoder.calls)
	}
	variants, err := h.artRepo.ListVariants(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("safe fast path must not create variants, got %+v", variants)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue after safe fast path, got %v", h.streams.byStream)
	}
}

func TestVideoFastPathUnsafeCodecWithoutFallbackFailsClosed(t *testing.T) {
	prober := &fakeVideoProber{metadata: safeVideoMetadata("hevc")}
	contract := validVideoContract(domain.ProviderMock, "mock-video")
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoModel = "mock-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.VideoProber = prober
		d.RequireVideoProbe = true
		d.VideoTranscodePolicy = "never"
		d.RawVideoDeliveryPolicy = "if_probe_passed"
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "unsafe codec")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != domain.JobErrMediaProviderOutputInvalid {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.JobErrMediaProviderOutputInvalid)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("unsafe codec must not enqueue delivery: %v", h.streams.byStream)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for unsafe codec, got %v", h.releaser.released)
	}
}

func TestVideoFastPathUnsafeCodecWithFallbackTranscodes(t *testing.T) {
	prober := &fakeVideoProber{sequence: []domain.ArtifactMediaMetadata{
		safeVideoMetadata("hevc"),
		safeVideoMetadata("h264"),
	}}
	transcoder := &fakeVideoTranscoder{
		out:      []byte("vk-ready-video"),
		metadata: safeVideoMetadata("h264"),
	}
	contract := validVideoContract(domain.ProviderMock, "mock-video")
	contract.TranscodeAllowed = true
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoModel = "mock-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.VideoProber = prober
		d.VideoTranscoder = transcoder
		d.RequireVideoProbe = true
		d.VideoTranscodeEnabled = true
		d.VideoTranscodePolicy = "fallback"
		d.RawVideoDeliveryPolicy = "if_probe_passed"
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "unsafe codec fallback")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if prober.calls != 2 {
		t.Fatalf("probe calls = %d, want original plus variant", prober.calls)
	}
	if transcoder.calls != 1 {
		t.Fatalf("transcoder calls = %d, want 1", transcoder.calls)
	}
	variants, err := h.artRepo.ListVariants(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 1 || variants[0].VariantType != domain.VariantVKVideo {
		t.Fatalf("expected one VK-ready variant, got %+v", variants)
	}
}

func TestVideoTranscodeConcurrencyLimiterFailsClosed(t *testing.T) {
	prober := &fakeVideoProber{sequence: []domain.ArtifactMediaMetadata{
		safeVideoMetadata("hevc"),
		safeVideoMetadata("hevc"),
		safeVideoMetadata("h264"),
	}}
	transcoder := newBlockingVideoTranscoder([]byte("vk-ready-video"), safeVideoMetadata("h264"))
	contract := validVideoContract(domain.ProviderMock, "mock-video")
	contract.TranscodeAllowed = true
	h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
		d.VideoModel = "mock-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.VideoProber = prober
		d.VideoTranscoder = transcoder
		d.RequireVideoProbe = true
		d.VideoTranscodeEnabled = true
		d.VideoTranscodePolicy = "fallback"
		d.RawVideoDeliveryPolicy = "if_probe_passed"
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
		d.MediaMaxConcurrentProbes = 4
		d.MediaMaxConcurrentTranscodes = 1
		d.MediaMaxPendingVariants = 4
	})
	ctx := context.Background()
	first := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "first unsafe codec")
	second := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "second unsafe codec")
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- h.gen.Process(ctx, taskFor(first))
	}()

	select {
	case <-transcoder.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first transcode did not start")
	}
	if err := h.gen.Process(ctx, taskFor(second)); err != nil {
		t.Fatalf("second process: %v", err)
	}
	if calls := transcoder.Calls(); calls != 1 {
		t.Fatalf("transcode limiter must prevent second ffmpeg call, got %d calls", calls)
	}
	secondReloaded := h.reload(t, second.ID)
	if secondReloaded.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("second status = %q, want failed_terminal", secondReloaded.Status)
	}
	if secondReloaded.ErrorCode != domain.JobErrMediaOverloadedRetryLater {
		t.Fatalf("second error = %q, want %q", secondReloaded.ErrorCode, domain.JobErrMediaOverloadedRetryLater)
	}
	close(transcoder.release)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first process: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first transcode did not finish")
	}
	firstReloaded := h.reload(t, first.ID)
	if firstReloaded.Status != domain.JobStatusResultReady {
		t.Fatalf("first status = %q, want result_ready", firstReloaded.Status)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != second.ID {
		t.Fatalf("expected reservation release only for overloaded second job, got %v", h.releaser.released)
	}
}

func TestVideoProbeDisabledDevMockStaysSimple(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "dev video")

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue in dev/mock disabled policy, got %v", h.streams.byStream)
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
	if got.ErrorCode != domain.JobErrMediaProviderOutputInvalid {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.JobErrMediaProviderOutputInvalid)
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
	if got.ErrorCode != domain.JobErrMediaProcessingUnavailable {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.JobErrMediaProcessingUnavailable)
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
	if got.ErrorCode != domain.JobErrMediaProcessingUnavailable {
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
	if got.CostCaptured != 0 {
		t.Fatalf("timed out provider submit must not capture credits, got %d", got.CostCaptured)
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

func TestProviderRegistryMediaFallbackBudgetStopsExtraPaidSubmit(t *testing.T) {
	primary := &routingProvider{
		name:      domain.ProviderName("primary"),
		cost:      1,
		fail:      routingError{class: domain.ProviderErrRateLimited},
		operation: domain.OperationVideoGenerate,
		modality:  domain.ModalityVideo,
		model:     "safe-video",
	}
	fallback := &routingProvider{
		name:      domain.ProviderName("fallback"),
		cost:      10,
		operation: domain.OperationVideoGenerate,
		modality:  domain.ModalityVideo,
		model:     "safe-video",
	}
	reg := worker.NewRegistry(primary, fallback)
	primaryContract := validVideoContract(primary.name, "safe-video")
	fallbackContract := validVideoContract(fallback.name, "safe-video")
	reg.ConfigureProviderMediaContracts([]domain.ProviderMediaContract{primaryContract, fallbackContract}, true, false)
	reg.ConfigureMediaProviderBudget(1, 0)
	req := domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   "safe-video",
		Prompt:      "safe video",
		DurationSec: 5,
		Resolution:  "720p",
		AspectRatio: "16:9",
		AttemptNo:   1,
	}

	provider, err := reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("for request: %v", err)
	}
	if _, err := provider.Submit(context.Background(), req); err == nil {
		t.Fatal("expected primary failure after fallback budget exhaustion")
	}
	if primary.submits != 1 || fallback.submits != 0 {
		t.Fatalf("fallback budget must stop extra paid submits, primary=%d fallback=%d", primary.submits, fallback.submits)
	}
}

func TestProviderQualityGuardSkipsDisabledProviderWhenFallbackExists(t *testing.T) {
	primary := &routingProvider{
		name: domain.ProviderName("primary"),
		cost: 1,
		fail: routingError{class: domain.ProviderErrRateLimited},
	}
	fallback := &routingProvider{name: domain.ProviderName("fallback"), cost: 10}
	reg := worker.NewRegistry(primary, fallback)
	reg.ConfigureProviderQualityGuard(true, 1, 1, 1)
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
	if _, err := provider.Submit(context.Background(), req); err != nil {
		t.Fatalf("first submit should use fallback after primary failure: %v", err)
	}
	if primary.submits != 1 || fallback.submits != 1 {
		t.Fatalf("first route submits primary=%d fallback=%d", primary.submits, fallback.submits)
	}

	provider, err = reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("second for request: %v", err)
	}
	if _, err := provider.Submit(context.Background(), req); err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if primary.submits != 1 {
		t.Fatalf("disabled primary should be skipped while fallback is available, submits=%d", primary.submits)
	}
	if fallback.submits != 2 {
		t.Fatalf("fallback submits = %d, want 2", fallback.submits)
	}
}

func TestProviderQualityGuardDoesNotDropOnlyProvider(t *testing.T) {
	primary := &routingProvider{
		name: domain.ProviderName("primary"),
		cost: 1,
		fail: routingError{class: domain.ProviderErrRateLimited},
	}
	reg := worker.NewRegistry(primary)
	reg.ConfigureProviderQualityGuard(true, 1, 1, 1)
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
	if _, err := provider.Submit(context.Background(), req); err == nil {
		t.Fatal("expected first provider failure")
	}
	primary.fail = nil
	provider, err = reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("second for request should keep only disabled provider as fallback: %v", err)
	}
	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("second submit should still try only provider: %v", err)
	}
	if task.Provider != primary.name || primary.submits != 2 {
		t.Fatalf("unexpected only-provider retry: task=%+v submits=%d", task, primary.submits)
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

func TestProviderRegistryPinsExplicitRequestProvider(t *testing.T) {
	cheapDefault := &routingProvider{
		name:      domain.ProviderName("cheap-default"),
		cost:      1,
		operation: domain.OperationImageGenerate,
		modality:  domain.ModalityImage,
		model:     "ByteDance/Seedream-4.5",
	}
	deepInfraProvider := &routingProvider{
		name:      domain.ProviderDeepInfra,
		cost:      99,
		operation: domain.OperationImageGenerate,
		modality:  domain.ModalityImage,
		model:     "ByteDance/Seedream-4.5",
	}
	reg := worker.NewRegistry(cheapDefault, deepInfraProvider)
	req := domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		ModelCode: "ByteDance/Seedream-4.5",
		Provider:  domain.ProviderDeepInfra,
		Prompt:    "a cat",
	}

	provider, err := reg.ForRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("for request: %v", err)
	}
	if provider.Name() != domain.ProviderDeepInfra {
		t.Fatalf("provider = %q, want deepinfra", provider.Name())
	}
	if _, err := provider.Submit(context.Background(), req); err != nil {
		t.Fatalf("submit through router: %v", err)
	}
	if cheapDefault.submits != 0 || deepInfraProvider.submits != 1 {
		t.Fatalf("unexpected submits cheap=%d deepinfra=%d", cheapDefault.submits, deepInfraProvider.submits)
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
	reference := h.createInputImageArtifact(t, userID, validPNGBytes(t), "image/png")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "a cat",
		"aspect_ratio":           "1:1",
		"resolution":             "4K",
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
	if got.ModelCode != "foundation-image" || got.Size != "1024x1024" || got.AspectRatio != "1:1" || got.Resolution != "4K" {
		t.Fatalf("unexpected image request defaults: model=%q size=%q aspect=%q resolution=%q", got.ModelCode, got.Size, got.AspectRatio, got.Resolution)
	}
	if len(got.ReferenceArtifactIDs) != 1 || got.ReferenceArtifactIDs[0] != reference.ID {
		t.Fatalf("reference ids = %v, want %s", got.ReferenceArtifactIDs, reference.ID)
	}
	if len(got.InputURLs) != 1 || !strings.HasPrefix(got.InputURLs[0], "data:image/png;base64,") {
		t.Fatalf("input urls were not resolved from reference artifact: %v", got.InputURLs)
	}
}

func TestGenerationImageRequestCarriesProviderFromParams(t *testing.T) {
	provider := &captureImageProvider{name: domain.ProviderDeepInfra}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	params, _ := json.Marshal(map[string]any{
		"prompt":   "a cat",
		"provider": string(domain.ProviderDeepInfra),
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

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	if provider.last.Provider != domain.ProviderDeepInfra {
		t.Fatalf("provider = %q, want %q", provider.last.Provider, domain.ProviderDeepInfra)
	}
}

func TestBuildRequest_ResolvesReferenceInputURLs(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, validPNGBytes(t), "image/png")
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

func TestBuildRequest_SanitizesJPEGReferenceMetadata(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	userID := uuid.New()
	secret := "GPS_METADATA_SECRET"
	raw := jpegWithAPP1(t, secret)
	reference := h.createInputImageArtifact(t, userID, raw, "image/jpeg")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "use private jpeg metadata",
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

	storedRaw, err := h.store.GetObject(ctx, reference.StorageBucket, reference.StorageKey)
	if err != nil {
		t.Fatalf("get raw reference: %v", err)
	}
	if !bytes.Contains(storedRaw, []byte(secret)) {
		t.Fatal("raw private artifact should remain unchanged for owner access/debugging policy")
	}
	sanitized := dataURLBytes(t, provider.last.InputURLs[0], "image/jpeg")
	if bytes.Contains(sanitized, []byte(secret)) {
		t.Fatalf("provider input leaked JPEG metadata marker %q", secret)
	}
	for _, forbidden := range []string{reference.StorageBucket, reference.StorageKey, reference.SHA256} {
		if forbidden != "" && strings.Contains(provider.last.InputURLs[0], forbidden) {
			t.Fatalf("provider input leaked private artifact detail %q", forbidden)
		}
	}
}

func TestBuildRequest_SanitizesPNGTextMetadata(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	userID := uuid.New()
	secret := "PNG_COMMENT_SECRET"
	raw := pngWithTextChunk(t, "Comment", secret)
	reference := h.createInputImageArtifact(t, userID, raw, "image/png")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "use private png metadata",
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

	sanitized := dataURLBytes(t, provider.last.InputURLs[0], "image/png")
	if bytes.Contains(sanitized, []byte(secret)) {
		t.Fatalf("provider input leaked PNG metadata marker %q", secret)
	}
}

func TestBuildRequest_RejectsUnsupportedWebPReferenceBeforeProvider(t *testing.T) {
	provider := &captureImageProvider{}
	h := newHarnessWithProvider(t, provider, nil)
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, minimalWebPBytes(), "image/webp")
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "use webp reference",
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

	err := h.gen.Process(ctx, taskFor(job))
	if err == nil || !strings.Contains(err.Error(), "unsupported reference image format") {
		t.Fatalf("expected unsupported reference image error, got %v", err)
	}
	if provider.last.JobID != uuid.Nil {
		t.Fatalf("provider must not be called for unsupported reference image, got request %+v", provider.last)
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

func TestProviderMediaContractAllowsValidVideoSpec(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{validVideoContract(provider.name, provider.model)}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "safe clip",
		"duration_sec": 5,
		"aspect_ratio": "16:9",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	if provider.submits != 1 {
		t.Fatalf("provider submits = %d, want 1", provider.submits)
	}
	if provider.last.DurationSec != 5 || provider.last.AspectRatio != "16:9" || provider.last.Resolution != "720p" {
		t.Fatalf("unexpected provider request: %+v", provider.last)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusProviderProcessing {
		t.Fatalf("status = %q, want provider_processing", got.Status)
	}
}

func TestGenerationVideoRequestUsesResolvedRouteSnapshot(t *testing.T) {
	provider := &captureVideoProvider{
		name:  domain.ProviderPoYo,
		model: poyo.ModelKlingO3Standard,
		cost:  100,
	}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		contract := validVideoContract(provider.name, provider.model)
		contract.AllowedDurationsSec = []int{10}
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  domain.VideoRouteKlingO3Standard,
		Provider:               domain.ProviderPoYo,
		ProviderModelID:        poyo.ModelKlingO3Standard,
		ModelClass:             "kling_o3_standard",
		DurationSec:            10,
		Resolution:             "720p",
		AspectRatio:            "16:9",
		ProviderCostCredits:    100,
		InternalCostCredits:    200,
		PriceMultiplier:        2,
		MaxProviderCostCredits: 100,
		MaxInternalCostCredits: 200,
	}
	job := h.queueVideoJob(t, map[string]any{
		"prompt":               "clean prompt",
		"video_route_alias":    string(domain.VideoRouteKlingO3Standard),
		"duration_sec":         5,
		"resolved_video_route": snapshot,
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if provider.submits != 1 {
		t.Fatalf("provider submits = %d, want 1", provider.submits)
	}
	if provider.last.Provider != domain.ProviderPoYo || provider.last.ModelCode != poyo.ModelKlingO3Standard {
		t.Fatalf("provider request did not use route snapshot: %+v", provider.last)
	}
	if provider.last.DurationSec != 10 || provider.last.Resolution != "720p" || provider.last.AspectRatio != "16:9" {
		t.Fatalf("provider request timing/shape did not use snapshot: %+v", provider.last)
	}
	var providerParams map[string]json.RawMessage
	if err := json.Unmarshal(provider.last.Params, &providerParams); err != nil {
		t.Fatalf("unmarshal provider params: %v", err)
	}
	if _, ok := providerParams["resolved_video_route"]; !ok {
		t.Fatalf("provider params missing route snapshot: %s", string(provider.last.Params))
	}
	if strings.Contains(string(provider.last.Params), "clean prompt") {
		t.Fatalf("provider params must not persist prompt: %s", string(provider.last.Params))
	}
	tasks, err := h.tasks.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list provider tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ModelCode != poyo.ModelKlingO3Standard {
		t.Fatalf("unexpected provider task: %+v", tasks)
	}
	if strings.Contains(string(tasks[0].Request), "clean prompt") {
		t.Fatalf("provider task request must not persist prompt: %s", string(tasks[0].Request))
	}
}

func TestGenerationVideoResolvesReferenceInputURLs(t *testing.T) {
	provider := &captureVideoProvider{
		name:  domain.ProviderAPIMart,
		model: apimart.ModelHailuo23Fast,
		cost:  2,
	}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ProviderMediaContracts = []domain.ProviderMediaContract{{
			Provider:               domain.ProviderAPIMart,
			Model:                  apimart.ModelHailuo23Fast,
			ModelClass:             "hailuo_2_3_fast",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{6},
			AllowedResolutions:     []string{"768p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       128 << 20,
			DeliveryReadyOutput:    true,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 2,
		}}
	})
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, validPNGBytes(t), "image/png")
	snapshot := domain.VideoRouteSnapshot{
		Alias:               domain.VideoRouteHailuo23Fast,
		Provider:            domain.ProviderAPIMart,
		ProviderModelID:     apimart.ModelHailuo23Fast,
		ModelClass:          "hailuo_2_3_fast",
		DurationSec:         6,
		Resolution:          "768p",
		ProviderCostCredits: 1,
		InternalCostCredits: 2,
		PriceMultiplier:     2,
	}
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "safe clip",
		"video_route_alias":      string(domain.VideoRouteHailuo23Fast),
		"reference_artifact_ids": []string{reference.ID.String()},
		"resolved_video_route":   snapshot,
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   2,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(provider.last.InputURLs) != 1 || !strings.HasPrefix(provider.last.InputURLs[0], "data:image/png;base64,") {
		t.Fatalf("input urls = %v, want one sanitized data URL", provider.last.InputURLs)
	}
	tasks, err := h.tasks.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	if strings.Contains(string(tasks[0].Request), "base64") || strings.Contains(string(tasks[0].Request), "data:image") {
		t.Fatalf("provider task request must not persist reference bytes: %s", string(tasks[0].Request))
	}
}

func TestGenerationVideoAPIMartStoresOutputBeforeDelivery(t *testing.T) {
	var submitSeen bool
	var pollSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/videos/generations":
			submitSeen = true
			_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_apimart"}]}`))
		case "/tasks/task_apimart":
			pollSeen = true
			_, _ = w.Write([]byte(`{"code":200,"data":{"id":"task_apimart","status":"completed","progress":100,"result":{"videos":[{"url":["https://upload.apimart.ai/f/video/output.mp4?token=secret"],"expires_at":1763174708}]}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := apimart.New(apimart.Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ProviderMediaContracts = []domain.ProviderMediaContract{{
			Provider:               domain.ProviderAPIMart,
			Model:                  apimart.ModelHailuo23Standard,
			ModelClass:             "hailuo_2_3_standard",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{6},
			AllowedResolutions:     []string{"768p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       128 << 20,
			DeliveryReadyOutput:    true,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 2,
		}}
	})
	ctx := context.Background()
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  domain.VideoRouteHailuo23Standard,
		Provider:               domain.ProviderAPIMart,
		ProviderModelID:        apimart.ModelHailuo23Standard,
		ModelClass:             "hailuo_2_3_standard",
		DurationSec:            6,
		Resolution:             "768p",
		ProviderCostCredits:    1,
		InternalCostCredits:    2,
		PriceMultiplier:        2,
		MaxProviderCostCredits: 1,
		MaxInternalCostCredits: 2,
	}
	job := h.queueVideoJob(t, map[string]any{
		"prompt":               "safe video",
		"video_route_alias":    string(domain.VideoRouteHailuo23Standard),
		"resolved_video_route": snapshot,
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !submitSeen || pollSeen {
		t.Fatalf("after submit submitSeen=%v pollSeen=%v", submitSeen, pollSeen)
	}
	pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
	if len(pollTasks) != 1 {
		t.Fatalf("expected one provider poll task, got %d", len(pollTasks))
	}
	if err := h.poll.Process(ctx, pollTasks[0]); err != nil {
		t.Fatalf("poll process: %v", err)
	}
	if !pollSeen {
		t.Fatalf("poll was not called")
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("status = %q, want result_ready", got.Status)
	}
	if len(got.OutputArtifactIDs) != 1 {
		t.Fatalf("output artifacts = %v, want one", got.OutputArtifactIDs)
	}
	artifact, err := h.artRepo.GetByID(ctx, got.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	data, err := h.store.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey)
	if err != nil {
		t.Fatalf("get stored output: %v", err)
	}
	if string(data) != "output" {
		t.Fatalf("stored output = %q", data)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("delivery stream = %v, want one task after artifact storage", h.streams.byStream[redisqueue.StreamDelivery])
	}
}

func TestGenerationVideoRunwayTaskFailureReleasesReservation(t *testing.T) {
	var submitSeen bool
	var pollSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/image_to_video":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s", r.Method)
			}
			submitSeen = true
			_, _ = w.Write([]byte(`{"id":"task_runway","status":"PENDING","createdAt":"2026-06-15T10:00:00.000Z"}`))
		case "/tasks/task_runway":
			if r.Method != http.MethodGet {
				t.Fatalf("poll method = %s", r.Method)
			}
			pollSeen = true
			_, _ = w.Write([]byte(`{"id":"task_runway","status":"FAILED","createdAt":"2026-06-15T10:00:00.000Z","failureCode":"SAFETY.INPUT.IMAGE","failure":"blocked by safety policy"}`))
		default:
			t.Fatalf("unexpected runway path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := runway.New(runway.Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ProviderMediaContracts = []domain.ProviderMediaContract{{
			Provider:               domain.ProviderRunway,
			Model:                  runway.ModelGen4Turbo,
			ModelClass:             "runway_gen4_turbo",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{5},
			AllowedAspectRatios:    []string{"16:9"},
			AllowedResolutions:     []string{"720p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       128 << 20,
			DeliveryReadyOutput:    true,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 50,
		}}
	})
	ctx := context.Background()
	userID := uuid.New()
	reference := h.createInputImageArtifact(t, userID, validPNGBytes(t), "image/png")
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  domain.VideoRouteRunwayGen4Turbo,
		Provider:               domain.ProviderRunway,
		ProviderModelID:        runway.ModelGen4Turbo,
		ModelClass:             "runway_gen4_turbo",
		DurationSec:            5,
		Resolution:             "720p",
		AspectRatio:            "16:9",
		ProviderCostCredits:    25,
		InternalCostCredits:    50,
		PriceMultiplier:        2,
		MaxProviderCostCredits: 25,
		MaxInternalCostCredits: 50,
	}
	params, _ := json.Marshal(map[string]any{
		"prompt":                 "safe video",
		"video_route_alias":      string(domain.VideoRouteRunwayGen4Turbo),
		"reference_artifact_ids": []string{reference.ID.String()},
		"resolved_video_route":   snapshot,
	})
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "job:" + uuid.NewString(),
		CorrelationID:  "corr",
		CostReserved:   50,
		Params:         params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !submitSeen || pollSeen {
		t.Fatalf("after submit submitSeen=%v pollSeen=%v", submitSeen, pollSeen)
	}
	pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
	if len(pollTasks) != 1 {
		t.Fatalf("expected one provider poll task, got %d", len(pollTasks))
	}
	if err := h.poll.Process(ctx, pollTasks[0]); err != nil {
		t.Fatalf("poll process: %v", err)
	}
	if !pollSeen {
		t.Fatalf("poll was not called")
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != string(domain.ProviderErrContentRejected) {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, domain.ProviderErrContentRejected)
	}
	if got.CostCaptured != 0 {
		t.Fatalf("cost captured = %d, want 0", got.CostCaptured)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for failed runway task, got %v", h.releaser.released)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("failed runway task must not enqueue delivery: %v", h.streams.byStream)
	}
}

func TestExternalAsyncVideoPollTransportErrorKeepsTaskPolling(t *testing.T) {
	provider := &captureVideoProvider{
		name:  domain.ProviderPoYo,
		model: poyo.ModelKlingO3Standard,
		cost:  100,
		pollErrors: []error{
			routingError{class: domain.ProviderErrRateLimited},
			routingError{class: domain.ProviderErrInternal},
		},
		pollResults: []domain.ProviderTaskResult{{
			Status:     domain.ProviderTaskSucceeded,
			OutputURLs: []string{"https://provider.test/video.mp4"},
		}},
	}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ProviderMediaContracts = []domain.ProviderMediaContract{{
			Provider:               domain.ProviderPoYo,
			Model:                  poyo.ModelKlingO3Standard,
			ModelClass:             "kling_o3_standard",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{5},
			AllowedAspectRatios:    []string{"16:9"},
			AllowedResolutions:     []string{"720p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       128 << 20,
			DeliveryReadyOutput:    true,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 100,
		}}
	})
	ctx := context.Background()
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  domain.VideoRouteKlingO3Standard,
		Provider:               domain.ProviderPoYo,
		ProviderModelID:        poyo.ModelKlingO3Standard,
		ModelClass:             "kling_o3_standard",
		DurationSec:            5,
		Resolution:             "720p",
		AspectRatio:            "16:9",
		ProviderCostCredits:    50,
		InternalCostCredits:    100,
		PriceMultiplier:        2,
		MaxProviderCostCredits: 100,
		MaxInternalCostCredits: 200,
	}
	job := h.queueVideoJob(t, map[string]any{
		"prompt":               "safe video",
		"video_route_alias":    string(domain.VideoRouteKlingO3Standard),
		"resolved_video_route": snapshot,
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("gen process: %v", err)
	}
	pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
	if len(pollTasks) != 1 {
		t.Fatalf("expected initial poll task, got %d", len(pollTasks))
	}

	if err := h.poll.Process(ctx, pollTasks[len(pollTasks)-1]); err != nil {
		t.Fatalf("first poll process: %v", err)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusProviderPending {
		t.Fatalf("after transient poll error status = %q, want provider_pending", got.Status)
	}
	if len(h.releaser.released) != 0 {
		t.Fatalf("transient poll error must not release credits: %v", h.releaser.released)
	}

	pollTasks = h.streams.byStream[redisqueue.StreamProviderPoll]
	if err := h.poll.Process(ctx, pollTasks[len(pollTasks)-1]); err != nil {
		t.Fatalf("second poll process: %v", err)
	}
	got = h.reload(t, job.ID)
	if got.Status != domain.JobStatusProviderPending {
		t.Fatalf("after second transient poll error status = %q, want provider_pending", got.Status)
	}

	pollTasks = h.streams.byStream[redisqueue.StreamProviderPoll]
	if err := h.poll.Process(ctx, pollTasks[len(pollTasks)-1]); err != nil {
		t.Fatalf("third poll process: %v", err)
	}
	got = h.reload(t, job.ID)
	if got.Status != domain.JobStatusResultReady {
		t.Fatalf("after successful poll status = %q, want result_ready", got.Status)
	}
	if provider.polls != 3 {
		t.Fatalf("polls = %d, want 3", provider.polls)
	}
	if len(h.streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected one delivery task, got %v", h.streams.byStream[redisqueue.StreamDelivery])
	}
}

func TestExternalAsyncVideoPollTransportErrorKeepsPollingAfterOldBudget(t *testing.T) {
	provider := &captureVideoProvider{
		name:  domain.ProviderPoYo,
		model: poyo.ModelKlingO3Standard,
		cost:  100,
		pollErrors: []error{
			routingError{class: domain.ProviderErrInternal},
		},
	}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.ProviderMediaContracts = []domain.ProviderMediaContract{{
			Provider:               domain.ProviderPoYo,
			Model:                  poyo.ModelKlingO3Standard,
			ModelClass:             "kling_o3_standard",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{5},
			AllowedAspectRatios:    []string{"16:9"},
			AllowedResolutions:     []string{"720p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       128 << 20,
			DeliveryReadyOutput:    true,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 100,
		}}
	})
	ctx := context.Background()
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  domain.VideoRouteKlingO3Standard,
		Provider:               domain.ProviderPoYo,
		ProviderModelID:        poyo.ModelKlingO3Standard,
		ModelClass:             "kling_o3_standard",
		DurationSec:            5,
		Resolution:             "720p",
		AspectRatio:            "16:9",
		ProviderCostCredits:    50,
		InternalCostCredits:    100,
		PriceMultiplier:        2,
		MaxProviderCostCredits: 100,
		MaxInternalCostCredits: 200,
	}
	job := h.queueVideoJob(t, map[string]any{
		"prompt":               "safe video",
		"video_route_alias":    string(domain.VideoRouteKlingO3Standard),
		"resolved_video_route": snapshot,
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("gen process: %v", err)
	}
	pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
	if len(pollTasks) != 1 {
		t.Fatalf("expected initial poll task, got %d", len(pollTasks))
	}
	task := pollTasks[len(pollTasks)-1]
	task.Attempt = 20
	if err := h.poll.Process(ctx, task); err != nil {
		t.Fatalf("poll process: %v", err)
	}

	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusProviderPending {
		t.Fatalf("after old-budget poll error status = %q, want provider_pending", got.Status)
	}
	if len(h.releaser.released) != 0 {
		t.Fatalf("old-budget transient poll error must not release credits: %v", h.releaser.released)
	}
	if len(h.streams.byStream[redisqueue.StreamProviderPoll]) < 2 {
		t.Fatalf("expected poll task to be requeued, got %v", h.streams.byStream[redisqueue.StreamProviderPoll])
	}
}

func TestProviderMediaContractRejectsUnsupportedDurationBeforeProvider(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{validVideoContract(provider.name, provider.model)}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "unsafe duration",
		"duration_sec": 7,
		"aspect_ratio": "16:9",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	assertRejectedBeforeProvider(t, h, provider, job, "unsupported_capability")
}

func TestProviderMediaContractRejectsUnsupportedAspectBeforeProvider(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{validVideoContract(provider.name, provider.model)}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "unsafe aspect",
		"duration_sec": 5,
		"aspect_ratio": "4:3",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	assertRejectedBeforeProvider(t, h, provider, job, "unsupported_capability")
}

func TestProviderMediaContractRejectsUnsupportedModelBeforeProvider(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{validVideoContract(provider.name, provider.model)}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "unsafe model",
		"model_code":   "unapproved-video-model",
		"duration_sec": 5,
		"aspect_ratio": "16:9",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	assertRejectedBeforeProvider(t, h, provider, job, "unsupported_capability")
}

func TestProviderMediaContractRejectsRequiredTranscodeWhenDisabled(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	contract := validVideoContract(provider.name, provider.model)
	contract.DeliveryReadyOutput = false
	contract.RequiresTranscode = true
	contract.TranscodeAllowed = true
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.VideoTranscodeEnabled = false
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "needs transcode",
		"duration_sec": 5,
		"aspect_ratio": "16:9",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	assertRejectedBeforeProvider(t, h, provider, job, "unsupported_capability")
}

func TestProviderMediaContractDeliveryReadyProbePathAllowsSubmit(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	contract := validVideoContract(provider.name, provider.model)
	contract.DeliveryReadyOutput = true
	contract.RequiresProbe = true
	contract.RequiresTranscode = false
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.VideoTranscodeEnabled = false
		d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":       "cheap probe path",
		"duration_sec": 5,
		"aspect_ratio": "16:9",
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	if provider.submits != 1 {
		t.Fatalf("provider submits = %d, want 1", provider.submits)
	}
	if len(h.streams.byStream[redisqueue.StreamProviderPoll]) != 1 {
		t.Fatalf("expected provider poll enqueue, got %v", h.streams.byStream)
	}
}

func TestProviderMediaContractStripsUnsafeNativeParams(t *testing.T) {
	provider := &captureVideoProvider{name: "capture-video", model: "safe-video", cost: 10}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.VideoModel = "safe-video"
		d.VideoDurationSec = 5
		d.VideoResolution = "720p"
		d.VideoAspectRatio = "16:9"
		d.RequireVideoProbe = true
		d.ProviderMediaContracts = []domain.ProviderMediaContract{validVideoContract(provider.name, provider.model)}
	})
	ctx := context.Background()
	job := h.queueVideoJob(t, map[string]any{
		"prompt":                  "safe clip",
		"duration_sec":            5,
		"aspect_ratio":            "16:9",
		"provider_native_payload": "must-not-reach-adapter",
		"input_urls":              []string{"INPUT_REFERENCE_SENTINEL"},
	})

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	params := string(provider.last.Params)
	for _, forbidden := range []string{"provider_native_payload", "input_urls", "must-not-reach-adapter", "INPUT_REFERENCE_SENTINEL"} {
		if strings.Contains(params, forbidden) {
			t.Fatalf("provider params leaked unsafe value %q in %s", forbidden, params)
		}
	}
	for _, want := range []string{"duration_sec", "resolution", "aspect_ratio"} {
		if !strings.Contains(params, want) {
			t.Fatalf("provider params missing safe product field %q in %s", want, params)
		}
	}
}

func validVideoContract(provider domain.ProviderName, model string) domain.ProviderMediaContract {
	return domain.ProviderMediaContract{
		Provider:               provider,
		Model:                  model,
		ModelClass:             "safe_video",
		Modality:               domain.ModalityVideo,
		AllowedDurationsSec:    []int{5},
		AllowedAspectRatios:    []string{"16:9"},
		AllowedResolutions:     []string{"720p"},
		ExpectedContainer:      "mp4",
		ExpectedCodec:          "h264",
		ExpectedMaxBytes:       128 << 20,
		DeliveryReadyOutput:    true,
		RequiresProbe:          true,
		TranscodeAllowed:       false,
		MaxProviderAttempts:    1,
		MaxFallbackAttempts:    0,
		MaxProviderCostCredits: 100,
	}
}

func safeVideoMetadata(codec string) domain.ArtifactMediaMetadata {
	return domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       codec,
		Container:   "mp4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	}
}

func assertRejectedBeforeProvider(t *testing.T, h *harness, provider *captureVideoProvider, job *domain.Job, code string) {
	t.Helper()
	if provider.submits != 0 {
		t.Fatalf("provider submits = %d, want 0; last=%+v", provider.submits, provider.last)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status = %q, want failed_terminal", got.Status)
	}
	if got.ErrorCode != code {
		t.Fatalf("error code = %q, want %q", got.ErrorCode, code)
	}
	if len(h.streams.byStream[redisqueue.StreamProviderPoll]) != 0 || len(h.streams.byStream[redisqueue.StreamDelivery]) != 0 {
		t.Fatalf("rejected request must not enqueue follow-up streams: %v", h.streams.byStream)
	}
	if len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("expected reservation release for rejected job, got %v", h.releaser.released)
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
	name domain.ProviderName
	last domain.ProviderRequest
}

func (p *captureImageProvider) Name() domain.ProviderName {
	if p.name != "" {
		return p.name
	}
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

type captureVideoProvider struct {
	name        domain.ProviderName
	model       string
	cost        int64
	last        domain.ProviderRequest
	submits     int
	polls       int
	pollErrors  []error
	pollResults []domain.ProviderTaskResult
}

func (p *captureVideoProvider) Name() domain.ProviderName {
	if p.name == "" {
		return domain.ProviderName("capture-video")
	}
	return p.name
}

func (p *captureVideoProvider) Capabilities(context.Context) ([]domain.Capability, error) {
	model := p.model
	if model == "" {
		model = "safe-video"
	}
	return []domain.Capability{{
		Operation:       domain.OperationVideoGenerate,
		Modality:        domain.ModalityVideo,
		ModelCode:       model,
		SupportsPolling: true,
		MaxDurationSec:  10,
	}}, nil
}

func (p *captureVideoProvider) Estimate(context.Context, domain.ProviderRequest) (domain.CostEstimate, error) {
	cost := p.cost
	if cost <= 0 {
		cost = 10
	}
	return domain.CostEstimate{AmountCredits: cost, Currency: "credits"}, nil
}

func (p *captureVideoProvider) Submit(_ context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	p.submits++
	p.last = req
	return domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       p.Name(),
		ModelCode:      req.ModelCode,
		ExternalID:     "capture-video-task",
		Status:         domain.ProviderTaskPending,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func (p *captureVideoProvider) Poll(context.Context, domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	p.polls++
	if len(p.pollErrors) > 0 {
		err := p.pollErrors[0]
		p.pollErrors = p.pollErrors[1:]
		return domain.ProviderTaskResult{}, err
	}
	if len(p.pollResults) > 0 {
		res := p.pollResults[0]
		p.pollResults = p.pollResults[1:]
		return res, nil
	}
	return domain.ProviderTaskResult{Status: domain.ProviderTaskProcessing}, nil
}

func (p *captureVideoProvider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }

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
