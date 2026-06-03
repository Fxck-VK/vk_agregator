package worker_test

import (
	"context"
	"encoding/json"
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
