package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/audiodownload"
)

type audioDownloaderFunc func(context.Context, string) (audiodownload.Result, error)

func (f audioDownloaderFunc) DownloadWithMetadata(ctx context.Context, url string) (audiodownload.Result, error) {
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
		Params:         json.RawMessage(`{"source_url":"https://www.youtube.com/watch?v=abcdefghijk","download_filename":"Мой ролик epifera.com.mp3"}`),
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
		audioDownloaderFunc(func(context.Context, string) (audiodownload.Result, error) {
			t.Error("recovery contacted the audio source again")
			return audiodownload.Result{}, errors.New("unexpected download")
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
	var savedParams struct {
		Filename string `json:"download_filename"`
	}
	if json.Unmarshal(stored.Params, &savedParams) != nil || savedParams.Filename != "Мой ролик epifera.com.mp3" {
		t.Fatalf("recovery lost the download filename: %s", stored.Params)
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

func TestAudioDownloadPersistsFilenameBeforeSavingArtifact(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	job := &domain.Job{
		ID: uuid.New(), AccountID: uuid.New(), Status: domain.JobStatusPostprocessing,
		Source: "web", OperationType: domain.OperationAudioDownload,
		Params:         json.RawMessage(`{"source_url":"https://soundcloud.com/artist/track","retained":"value"}`),
		IdempotencyKey: uuid.NewString(),
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	expected := "Название - Исполнитель epifera.com.mp3"
	w := NewAudioDownloadWorker(
		Deps{Jobs: jobs},
		audioDownloaderFunc(func(_ context.Context, source string) (audiodownload.Result, error) {
			if source != "https://soundcloud.com/artist/track" {
				t.Fatal("source URL changed before download")
			}
			return audiodownload.Result{Data: []byte("ID3sample"), Filename: expected}, nil
		}),
		audioArtifactSaverFunc(func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, domain.ArtifactKind, domain.MediaType, string, []byte) (*domain.Artifact, error) {
			stored, err := jobs.GetByID(ctx, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]string
			if json.Unmarshal(stored.Params, &params) != nil || params["download_filename"] != expected ||
				params["source_url"] != "https://soundcloud.com/artist/track" || params["retained"] != "value" {
				t.Fatalf("filename checkpoint incomplete: %s", stored.Params)
			}
			return nil, errors.New("simulate interrupted artifact storage")
		}),
		nil, nil,
	)
	if _, err := w.saveOutput(ctx, job); err == nil {
		t.Fatal("artifact storage error was lost")
	}
}

type audioArtifactSaverFunc func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, domain.ArtifactKind, domain.MediaType, string, []byte) (*domain.Artifact, error)

func (f audioArtifactSaverFunc) SaveBytesArtifactForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mime string, data []byte) (*domain.Artifact, error) {
	return f(ctx, userID, accountID, jobID, kind, mediaType, mime, data)
}
