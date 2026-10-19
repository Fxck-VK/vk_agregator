package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestDEVStudioToolsReachAdapterWithOwnedSources(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	ctx, owner := context.Background(), uuid.New()
	jobs, tasks, artifacts := memory.NewJobRepo(), memory.NewProviderTaskRepo(), memory.NewArtifactRepo()
	generated := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)
	imported := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, nil)
	for _, source := range []*domain.Job{generated, imported} {
		addSucceededMusicResult(t, ctx, jobs, source, domain.MusicResult{Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 3, DurationSec: 120}, {OriginalAudioIndex: 5, DurationSec: 90}}})
		createProviderTaskForTest(t, ctx, tasks, source.ID, "music:owned-source")
	}
	inputID := createMusicInputArtifactForTest(t, ctx, artifacts, owner, 120000)
	p := processor{jobs: jobs, tasks: tasks, artifactRepo: artifacts, providerReferences: fakeMusicReferenceSigner{}}
	adapter := apimart.New(apimart.Config{})
	at, custom := 30.0, false
	for _, model := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
		for _, op := range musicgeneration.OperationsForModel(model) {
			if !providermodels.RuntimeRegistry().MediaCandidateRunnable(model, string(op.ID)) {
				continue
			}
			t.Run(model+"/"+string(op.ID), func(t *testing.T) {
				r := musicgeneration.Request{ModelID: model, Music: domain.MusicRequest{Action: op.ID, Prompt: "Synthetic forest ballad", GPTDescription: "Gentle acoustic melody", Tags: "acoustic", Custom: &custom}}
				if op.ID == domain.MusicActionUploadExtend || op.ID == domain.MusicActionExtend {
					r.Music.ContinueAtSec = &at
				}
				if op.SupportsAudioFormat {
					r.Music.AudioFormat = "wav"
				}
				if op.ID == domain.MusicActionSounds {
					r.Music.SoundType, r.Music.Key, r.Music.BPM = "loop", "C#m", 300
				}
				source := generated
				if op.ID == domain.MusicActionAddVocals || op.ID == domain.MusicActionAddInstrumental {
					source = imported
				}
				for i := 0; i < op.MinSources; i++ {
					r.Sources = append(r.Sources, musicgeneration.Source{JobID: source.ID, AudioIndex: 3 + 2*i})
				}
				for range op.MinUploads {
					r.AudioArtifactIDs = append(r.AudioArtifactIDs, inputID)
				}
				params, price, err := musicgeneration.Resolve(r, providermodels.RuntimeRegistry())
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(params)
				quote, _ := json.Marshal(price)
				job := &domain.Job{ID: uuid.New(), AccountID: owner, OperationType: domain.OperationAudioMusic, Modality: domain.ModalityAudio, Params: raw, PricingSnapshot: quote, CostEstimate: price.InternalCredits, InputArtifactIDs: r.AudioArtifactIDs}
				request, err := p.buildMusicRequest(ctx, job, 1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := adapter.Estimate(ctx, request); err != nil {
					t.Fatal(err)
				}
				if op.MinSources == 1 && request.Music.SourceAudioIndex != 3 {
					t.Fatal("original source index lost")
				}
				if op.MinUploads > 0 && request.Music.AudioURL == "" {
					t.Fatal("owned upload was not resolved")
				}
			})
		}
	}
}
