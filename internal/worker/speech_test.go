package worker

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/speechgeneration"
)

func whisperWorkerFixture(t *testing.T) (*processor, *domain.Job, *domain.Artifact) {
	t.Helper()
	ctx, owner := context.Background(), uuid.New()
	data := []byte("RIFFxxxxWAVEfmt synthetic")
	a := &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeAudio, Status: domain.ArtifactStatusReady, StorageBucket: "private", StorageKey: "synthetic.wav", MimeType: "audio/wav", SizeBytes: int64(len(data)), DurationMS: 60001, Codec: "pcm_s16le", Container: "wav", ProbeStatus: domain.MediaProbePassed}
	arts, objects := memory.NewArtifactRepo(), memory.NewObjectStore()
	if err := arts.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := objects.Put(ctx, a.StorageBucket, a.StorageKey, data, a.MimeType); err != nil {
		t.Fatal(err)
	}
	prices, _ := pricingcatalog.NewStaticCatalog()
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	params, q, err := speechgeneration.Resolve(speechgeneration.Request{ModelID: "whisper_1", AudioArtifactID: a.ID, Speech: domain.SpeechRequest{Format: "json"}}, providermodels.RuntimeRegistry(), prices, a.DurationMS)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(params)
	price, _ := json.Marshal(q)
	job := &domain.Job{ID: uuid.New(), AccountID: owner, OperationType: domain.OperationAudioSTT, Modality: domain.ModalityText, Params: raw, PricingSnapshot: price, CostEstimate: 50, InputArtifactIDs: []uuid.UUID{a.ID}}
	return &processor{artifactRepo: arts, objects: objects}, job, a
}

func TestWhisperWorkerRejectsChangedInputBeforeSubmit(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	p, job, a := whisperWorkerFixture(t)
	ctx := context.Background()
	r, err := p.buildSpeechRequest(ctx, job, 1)
	if err != nil || r.Speech == nil || r.Speech.FileExtension != "wav" || len(r.Speech.FileBytes) != int(a.SizeBytes) || len(r.InputURLs) != 0 {
		t.Fatalf("speech hydration: %v", err)
	}
	for _, change := range []func(*domain.Artifact){func(a *domain.Artifact) { a.OwnerAccountID = uuid.New() }, func(a *domain.Artifact) { a.DurationMS++ }, func(a *domain.Artifact) { a.SizeBytes++ }, func(a *domain.Artifact) { a.ProbeStatus = "pending" }} {
		bad := *a
		change(&bad)
		if err := p.artifactRepo.(*memory.ArtifactRepo).Update(ctx, &bad); err != nil {
			t.Fatal(err)
		}
		if _, err := p.buildSpeechRequest(ctx, job, 1); err == nil {
			t.Fatal("changed input accepted")
		}
	}
}

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
