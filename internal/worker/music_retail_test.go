package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestPaidTrackJobPreservesAdditionalAlternativesAndReplays(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{}
	p := &processor{jobs: jobs, artifacts: saver}
	job := createMusicJobForTest(t, ctx, jobs, uuid.New(), domain.MusicActionGenerate, nil, nil)
	var quote pricingcatalog.PricingSnapshot
	if err := json.Unmarshal(job.PricingSnapshot, &quote); err != nil {
		t.Fatal(err)
	}
	if quote.MusicPaidTrackCount != 1 || quote.InternalCredits != 60 {
		t.Fatalf("new quote is not per-track: %+v", quote)
	}
	result := domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{
		{OriginalAudioIndex: 1, AudioURL: "https://example.com/one.mp3"},
		{OriginalAudioIndex: 2, AudioURL: "https://example.com/two.mp3"},
	}}}
	if err := p.saveMusicOutputs(ctx, job, result); err != nil {
		t.Fatal(err)
	}
	stored := mustGetJobForTest(t, ctx, jobs, job.ID)
	params, err := musicgeneration.DecodeJob(stored)
	if err != nil {
		t.Fatal(err)
	}
	if params.Result == nil || len(params.Result.Music.Tracks) != 2 || len(params.Result.Artifacts) != 2 || len(saver.saved) != 2 {
		t.Fatalf("all returned alternatives must remain available: %+v saves%d", params.Result, len(saver.saved))
	}
	if params.Result.Music.Tracks[0].OriginalAudioIndex != 1 {
		t.Fatal("provider source index lost")
	}
	if err := p.saveMusicOutputs(ctx, stored, result); err != nil {
		t.Fatal(err)
	}
	if len(saver.saved) != 2 {
		t.Fatal("replay saved duplicate outputs")
	}
}

func TestHistoricalMusicJobRetainsItsOriginalVariableOutputs(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{}
	p := &processor{jobs: jobs, artifacts: saver}
	job := createMusicJobForTest(t, ctx, jobs, uuid.New(), domain.MusicActionGenerate, nil, nil)
	var quote pricingcatalog.PricingSnapshot
	if err := json.Unmarshal(job.PricingSnapshot, &quote); err != nil {
		t.Fatal(err)
	}
	quote.MusicPaidTrackCount, quote.RetailPolicyVersion = 0, 0
	job.PricingSnapshot, _ = json.Marshal(quote)
	if err := jobs.Update(ctx, job); err != nil {
		t.Fatal(err)
	}
	result := domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{
		{OriginalAudioIndex: 1, AudioURL: "https://example.com/one.mp3"},
		{OriginalAudioIndex: 2, AudioURL: "https://example.com/two.mp3"},
	}}}
	if err := p.saveMusicOutputs(ctx, job, result); err != nil {
		t.Fatal(err)
	}
	if len(saver.saved) != 2 {
		t.Fatalf("historical job repriced/reinterpreted: %d", len(saver.saved))
	}
}

func TestPaidTrackJobCannotCaptureAResultWithoutAudio(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{}
	p := &processor{jobs: jobs, artifacts: saver}
	job := createMusicJobForTest(t, ctx, jobs, uuid.New(), domain.MusicActionGenerate, nil, nil)
	result := domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 1, ImageURL: "https://example.com/cover.png"}}}}
	if err := p.saveMusicOutputs(ctx, job, result); err == nil {
		t.Fatal("cover-only response accepted as a paid music track")
	}
	if len(saver.saved) != 0 {
		t.Fatal("invalid audio result partially published")
	}
}
