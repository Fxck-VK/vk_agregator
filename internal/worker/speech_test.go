package worker

import (
	"context"
	"github.com/google/uuid"
	"testing"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

type speechTestSaver struct{ fakeMusicArtifactSaver }

func (s *speechTestSaver) SaveBytesArtifactWithMetadataForAccount(_ context.Context, user, account uuid.UUID, job *uuid.UUID, kind domain.ArtifactKind, media domain.MediaType, mime string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	return s.save(user, account, job, kind, media, "", string(data))
}

type speechTestProbe struct{}

func (speechTestProbe) ProbeVideo(context.Context, []byte, int64) (domain.ArtifactMediaMetadata, error) {
	panic("unexpected video probe")
}
func (speechTestProbe) ProbeAudio(context.Context, []byte, int64) (domain.ArtifactMediaMetadata, error) {
	return domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, DurationMS: 1000}, nil
}

func TestSynchronousSpeechCheckpointPersistsOwnedOutputOnce(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	saver := &speechTestSaver{}
	p := processor{jobs: jobs, artifacts: saver, videoProber: speechTestProbe{}}
	owner := uuid.New()
	job := &domain.Job{ID: uuid.New(), AccountID: owner, OperationType: domain.OperationAudioTTS, Modality: domain.ModalityAudio, Status: domain.JobStatusDispatchingProvider}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	result := domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, InlineAudio: &domain.InlineAudio{Bytes: []byte("RIFFxxxxWAVEsynthetic"), MIME: "audio/wav", Extension: "wav"}}
	if err := p.saveSynchronousResult(ctx, job, result); err != nil {
		t.Fatal(err)
	}
	stored, err := jobs.GetByID(ctx, job.ID)
	if err != nil || len(stored.OutputArtifactIDs) != 1 || len(saver.saved) != 1 || saver.saved[0].accountID != owner || saver.saved[0].userID != uuid.Nil {
		t.Fatal("output not checkpointed to account")
	}
	// Recover with a new processor and no copy of transient provider bytes.
	restarted := processor{jobs: jobs, artifacts: saver}
	if err := restarted.saveOutputs(ctx, stored, nil, ""); err != nil {
		t.Fatal(err)
	}
	pt := &domain.ProviderTask{Status: domain.ProviderTaskSucceeded, Result: domain.DurableProviderTaskResultJSON(result)}
	if _, ok := durableProviderTaskResultForJob(pt, stored); !ok {
		t.Fatal("cannot resume from artifacts")
	}
	if len(saver.saved) != 1 {
		t.Fatal("duplicate audio")
	}
	bad := *job
	bad.ID = uuid.New()
	bad.OutputArtifactIDs = nil
	result.InlineAudio.Bytes = []byte("<html>not audio</html>")
	if p.saveSynchronousResult(ctx, &bad, result) == nil || len(saver.saved) != 1 {
		t.Fatal("uninspected bytes stored")
	}
}
