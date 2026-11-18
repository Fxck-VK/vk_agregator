package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
)

type audioDownloaderFunc func(context.Context, string) ([]byte, error)

func (f audioDownloaderFunc) Download(ctx context.Context, url string) ([]byte, error) {
	return f(ctx, url)
}

func TestAudioDownloadResumesFromSavedArtifact(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	artifacts := memory.NewArtifactRepo()
	moderation := memory.NewModerationRepo()
	job := &domain.Job{
		ID: uuid.New(), AccountID: uuid.New(), Source: "web",
		ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:     domain.ResultModeAccountHistory, Modality: domain.ModalityAudio,
		OperationType: domain.OperationAudioDownload, Status: domain.JobStatusPostprocessing,
		IdempotencyKey: uuid.NewString(),
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	artifact := &domain.Artifact{
		ID: uuid.New(), OwnerAccountID: job.AccountID, JobID: &job.ID,
		Kind: domain.ArtifactKindOutput, Status: domain.ArtifactStatusReady,
		MediaType: domain.MediaTypeAudio, MimeType: "audio/mpeg",
	}
	if err := artifacts.Create(ctx, artifact); err != nil {
		t.Fatal(err)
	}
	w := NewAudioDownloadWorker(
		Deps{
			Jobs: jobs, ArtifactRepo: artifacts, ModResults: moderation,
			ResultReadyUOW: memory.NewUnitOfWork(jobs, outbox, nil),
		},
		audioDownloaderFunc(func(context.Context, string) ([]byte, error) {
			t.Error("recovery contacted the audio source again")
			return nil, errors.New("unexpected download")
		}),
		artifactservice.New(artifacts, nil, "test"),
		func(_ context.Context, accountID, jobID uuid.UUID) (uuid.UUID, error) {
			if accountID != job.AccountID || jobID != job.ID {
				t.Fatal("lookup did not include the job's canonical owner")
			}
			return artifact.ID, nil
		},
		nil,
	)
	for range 2 {
		if err := w.process(ctx, job.ID); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := jobs.GetByID(ctx, job.ID)
	if err != nil || stored.Status != domain.JobStatusResultReady ||
		len(stored.OutputArtifactIDs) != 1 || stored.OutputArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected checkpoint: %+v, %v", stored, err)
	}
	verdicts, err := moderation.ListByJob(ctx, job.ID)
	if err != nil || len(verdicts) != 1 || !verdicts[0].Decision.Allowed() {
		t.Fatalf("unexpected output moderation: %+v, %v", verdicts, err)
	}
	events := outbox.Events()
	if len(events) != 1 || events[0].EventType != eventJobResultReady {
		t.Fatalf("unexpected result delivery events: %+v", events)
	}
}
