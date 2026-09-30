package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestBuildMusicRequestRejectsForeignSource(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	p := &processor{jobs: jobs, tasks: tasks}

	owner := uuid.New()
	foreign := uuid.New()
	source := createMusicJobForTest(t, ctx, jobs, foreign, domain.MusicActionUpload, nil, nil)
	addSucceededMusicResult(t, ctx, jobs, source, domain.MusicResult{
		Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 7, DurationSec: 120}},
	})
	createProviderTaskForTest(t, ctx, tasks, source.ID, "music:foreign")

	target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionExtend, []musicgeneration.Source{
		{JobID: source.ID, AudioIndex: 7},
	}, nil)

	_, err := p.buildMusicRequest(ctx, target, 1)
	if err == nil || !strings.Contains(err.Error(), "owned") {
		t.Fatalf("buildMusicRequest error = %v, want ownership rejection", err)
	}
}

func TestBuildMusicRequestHydratesActualSourceIndexes(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	p := &processor{jobs: jobs, tasks: tasks}
	owner := uuid.New()

	first := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, nil)
	addSucceededMusicResult(t, ctx, jobs, first, domain.MusicResult{
		Tracks: []domain.MusicTrackResult{
			{OriginalAudioIndex: 1, DurationSec: 80},
			{OriginalAudioIndex: 5, DurationSec: 150},
		},
	})
	createProviderTaskForTest(t, ctx, tasks, first.ID, "music:first-native")

	second := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, nil)
	addSucceededMusicResult(t, ctx, jobs, second, domain.MusicResult{
		Tracks: []domain.MusicTrackResult{
			{OriginalAudioIndex: 2, DurationSec: 90},
			{OriginalAudioIndex: 9, DurationSec: 100},
		},
	})
	createProviderTaskForTest(t, ctx, tasks, second.ID, "music:second-native")

	target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionMashup, []musicgeneration.Source{
		{JobID: first.ID, AudioIndex: 5},
		{JobID: second.ID, AudioIndex: 2},
	}, nil)

	req, err := p.buildMusicRequest(ctx, target, 3)
	if err != nil {
		t.Fatalf("buildMusicRequest: %v", err)
	}
	if req.Music == nil {
		t.Fatal("ProviderRequest.Music is nil")
	}
	if got, want := req.Music.SourceTaskIDs, []string{"music:first-native", "music:second-native"}; !equalStrings(got, want) {
		t.Fatalf("SourceTaskIDs = %#v, want %#v", got, want)
	}
	if got, want := req.Music.SourceAudioIndexes, []int{5, 2}; !equalInts(got, want) {
		t.Fatalf("SourceAudioIndexes = %#v, want %#v", got, want)
	}
	if req.Music.SourceTaskID != "" || req.Music.SourceAudioIndex != 0 {
		t.Fatalf("single source fields populated for multi-source request: %#v", *req.Music)
	}
	if req.IdempotencyKey != "provider_submit:"+target.ID.String()+":3" {
		t.Fatalf("idempotency key = %q", req.IdempotencyKey)
	}
}

func TestBuildMusicRequestHydratesAudioArtifactReferences(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	owner := uuid.New()

	t.Run("fails closed without signer", func(t *testing.T) {
		target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, []uuid.UUID{uuid.New()})
		p := &processor{jobs: jobs, tasks: memory.NewProviderTaskRepo()}
		_, err := p.buildMusicRequest(ctx, target, 1)
		if err == nil || !strings.Contains(err.Error(), "audio artifact reference gateway is unavailable") {
			t.Fatalf("buildMusicRequest error = %v, want closed audio artifact gateway error", err)
		}
	})

	t.Run("single upload", func(t *testing.T) {
		id := createMusicInputArtifactForTest(t, ctx, artifacts, owner, 120_000)
		target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, []uuid.UUID{id})
		target.InputArtifactIDs = []uuid.UUID{id}
		if err := jobs.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		p := &processor{jobs: jobs, tasks: memory.NewProviderTaskRepo(), artifactRepo: artifacts, providerReferences: fakeMusicReferenceSigner{}}
		req, err := p.buildMusicRequest(ctx, target, 1)
		if err != nil {
			t.Fatalf("buildMusicRequest: %v", err)
		}
		if req.Music.AudioURL == "" || len(req.Music.AudioURLs) != 0 {
			t.Fatalf("music audio urls = %q %#v, want single signed URL", req.Music.AudioURL, req.Music.AudioURLs)
		}
		if strings.Contains(string(target.Params), "provider-reference.test") {
			t.Fatalf("job params stored signed URL: %s", string(target.Params))
		}
	})

	t.Run("inspiration with one input keeps array shape", func(t *testing.T) {
		id := createMusicInputArtifactForTest(t, ctx, artifacts, owner, 120_000)
		target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionInspo, nil, []uuid.UUID{id})
		target.InputArtifactIDs = []uuid.UUID{id}
		p := &processor{jobs: jobs, tasks: memory.NewProviderTaskRepo(), artifactRepo: artifacts, providerReferences: fakeMusicReferenceSigner{}}
		req, err := p.buildMusicRequest(ctx, target, 1)
		if err != nil {
			t.Fatal(err)
		}
		if req.Music.AudioURL != "" || len(req.Music.AudioURLs) != 1 {
			t.Fatal("inspiration must send audio_urls even for one input")
		}
	})

	t.Run("multi upload create model", func(t *testing.T) {
		var ids []uuid.UUID
		for range 6 {
			ids = append(ids, createMusicInputArtifactForTest(t, ctx, artifacts, owner, 120_000))
		}
		target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionCreateModel, nil, ids)
		target.InputArtifactIDs = append([]uuid.UUID(nil), ids...)
		if err := jobs.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		p := &processor{jobs: jobs, tasks: memory.NewProviderTaskRepo(), artifactRepo: artifacts, providerReferences: fakeMusicReferenceSigner{}}
		req, err := p.buildMusicRequest(ctx, target, 1)
		if err != nil {
			t.Fatalf("buildMusicRequest: %v", err)
		}
		if req.Music.AudioURL != "" || len(req.Music.AudioURLs) != 6 {
			t.Fatalf("music audio urls = %q %#v, want six signed URLs", req.Music.AudioURL, req.Music.AudioURLs)
		}
	})

	t.Run("upload cover rejects exact eight minutes", func(t *testing.T) {
		id := createMusicInputArtifactForTest(t, ctx, artifacts, owner, 480_000)
		target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUploadCover, nil, []uuid.UUID{id})
		target.InputArtifactIDs = []uuid.UUID{id}
		if err := jobs.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		p := &processor{jobs: jobs, tasks: memory.NewProviderTaskRepo(), artifactRepo: artifacts, providerReferences: fakeMusicReferenceSigner{}}
		_, err := p.buildMusicRequest(ctx, target, 1)
		if err == nil || !strings.Contains(err.Error(), "duration") {
			t.Fatalf("buildMusicRequest error = %v, want strict duration rejection", err)
		}
	})
}

func TestBuildMusicRequestRequiresUploadedSourceForAddVocals(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	p := &processor{jobs: jobs, tasks: tasks}
	owner := uuid.New()

	source := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)
	addSucceededMusicResult(t, ctx, jobs, source, domain.MusicResult{
		Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 1, DurationSec: 60}},
	})
	createProviderTaskForTest(t, ctx, tasks, source.ID, "music:not-upload")
	target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionAddVocals, []musicgeneration.Source{
		{JobID: source.ID, AudioIndex: 1},
	}, nil)

	_, err := p.buildMusicRequest(ctx, target, 1)
	if err == nil || !strings.Contains(err.Error(), "uploaded") {
		t.Fatalf("buildMusicRequest error = %v, want uploaded-source rejection", err)
	}
}

func TestBuildMusicRequestChecksSourceDurationBounds(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	p := &processor{jobs: jobs, tasks: tasks}
	owner := uuid.New()

	source := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, nil)
	addSucceededMusicResult(t, ctx, jobs, source, domain.MusicResult{
		Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 1, DurationSec: 30}},
	})
	createProviderTaskForTest(t, ctx, tasks, source.ID, "music:short")

	continueAt := 90.0
	target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionExtend, []musicgeneration.Source{
		{JobID: source.ID, AudioIndex: 1},
	}, nil)
	params, err := musicgeneration.DecodeJob(target)
	if err != nil {
		t.Fatal(err)
	}
	params.Music.ContinueAtSec = &continueAt
	updateMusicJobParamsForTest(t, ctx, jobs, target, params)

	_, err = p.buildMusicRequest(ctx, target, 1)
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("buildMusicRequest error = %v, want duration-bound rejection", err)
	}
}

func TestBuildMusicRequestUsesUploadedReceiptInputDuration(t *testing.T) {
	ctx := context.Background()
	allowMusicAdmissionForTest(t)

	jobs := memory.NewJobRepo()
	tasks := memory.NewProviderTaskRepo()
	artifacts := memory.NewArtifactRepo()
	saver := &fakeMusicArtifactSaver{}
	owner := uuid.New()
	inputID := createMusicInputArtifactForTest(t, ctx, artifacts, owner, 125_000)
	source := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionUpload, nil, []uuid.UUID{inputID})
	source.InputArtifactIDs = []uuid.UUID{inputID}
	if err := jobs.Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	p := &processor{jobs: jobs, tasks: tasks, artifacts: saver, artifactRepo: artifacts}

	err := p.saveMusicOutputs(ctx, source, domain.ProviderTaskResult{
		Status: domain.ProviderTaskSucceeded,
		Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{{
			OriginalAudioIndex: 1,
			AudioID:            "native-upload-receipt",
		}}},
	})
	if err != nil {
		t.Fatalf("saveMusicOutputs: %v", err)
	}
	if err := jobs.UpdateStatus(ctx, source.ID, source.Status, domain.JobStatusSucceeded, "", ""); err != nil {
		t.Fatal(err)
	}
	source = mustGetJobForTest(t, ctx, jobs, source.ID)
	createProviderTaskForTest(t, ctx, tasks, source.ID, "music:uploaded")

	continueAt := 90.0
	target := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionExtend, []musicgeneration.Source{
		{JobID: source.ID, AudioIndex: 1},
	}, nil)
	params, err := musicgeneration.DecodeJob(target)
	if err != nil {
		t.Fatal(err)
	}
	params.Music.ContinueAtSec = &continueAt
	updateMusicJobParamsForTest(t, ctx, jobs, target, params)

	req, err := p.buildMusicRequest(ctx, target, 1)
	if err != nil {
		t.Fatalf("buildMusicRequest: %v", err)
	}
	if req.Music.SourceTaskID != "music:uploaded" || req.Music.SourceAudioIndex != 1 {
		t.Fatalf("source fields = %q/%d", req.Music.SourceTaskID, req.Music.SourceAudioIndex)
	}
	stored, err := musicgeneration.DecodeJob(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.Result.Music.Tracks[0].DurationSec; got != 125 {
		t.Fatalf("stored upload duration = %v, want 125", got)
	}
}

func TestSaveMusicOutputsMixedAndTextOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("mixed media and helper text", func(t *testing.T) {
		jobs := memory.NewJobRepo()
		saver := &fakeMusicArtifactSaver{}
		p := &processor{jobs: jobs, artifacts: saver}
		owner := uuid.New()
		job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)

		err := p.saveMusicOutputs(ctx, job, domain.ProviderTaskResult{
			Status: domain.ProviderTaskSucceeded,
			Text:   "helper lyrics",
			Music: &domain.MusicResult{
				Tracks: []domain.MusicTrackResult{{
					OriginalAudioIndex: 3,
					Title:              "track",
					DurationSec:        42,
					AudioURL:           "https://private.example/audio.mp3?token=secret",
					ImageURL:           "https://private.example/image.jpg?token=secret",
					VideoURL:           "https://private.example/video.mp4?token=secret",
				}},
				Lyrics: []domain.MusicLyricsResult{{Title: "track", Text: "la la"}},
			},
		})
		if err != nil {
			t.Fatalf("saveMusicOutputs: %v", err)
		}
		stored := mustGetJobForTest(t, ctx, jobs, job.ID)
		params, err := musicgeneration.DecodeJob(stored)
		if err != nil {
			t.Fatal(err)
		}
		if params.Result == nil || !params.Result.Complete {
			t.Fatalf("stored result = %#v, want complete checkpoint", params.Result)
		}
		if got := len(params.Result.Artifacts); got != 4 {
			t.Fatalf("stored artifacts = %d, want 4", got)
		}
		if got := len(stored.OutputArtifactIDs); got != 4 {
			t.Fatalf("job output artifacts = %d, want 4", got)
		}
		if kinds := artifactKindsForTest(params.Result.Artifacts); !equalStrings(kinds, []string{"audio", "image", "video", "text"}) {
			t.Fatalf("artifact kinds = %#v", kinds)
		}
		if strings.Contains(string(stored.Params), "private.example") || strings.Contains(string(stored.Params), "secret") {
			t.Fatalf("job params leaked private URLs: %s", string(stored.Params))
		}
	})

	t.Run("text only structured output", func(t *testing.T) {
		jobs := memory.NewJobRepo()
		saver := &fakeMusicArtifactSaver{}
		p := &processor{jobs: jobs, artifacts: saver}
		owner := uuid.New()
		job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionLyrics, nil, nil)

		err := p.saveMusicOutputs(ctx, job, domain.ProviderTaskResult{
			Status: domain.ProviderTaskSucceeded,
			Music: &domain.MusicResult{
				Lyrics: []domain.MusicLyricsResult{{Title: "demo", Text: "only words", Tags: "pop"}},
			},
		})
		if err != nil {
			t.Fatalf("saveMusicOutputs: %v", err)
		}
		stored := mustGetJobForTest(t, ctx, jobs, job.ID)
		params, err := musicgeneration.DecodeJob(stored)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(params.Result.Artifacts); got != 1 {
			t.Fatalf("stored artifacts = %d, want 1", got)
		}
		if params.Result.Artifacts[0].Kind != "text" {
			t.Fatalf("artifact kind = %q, want text", params.Result.Artifacts[0].Kind)
		}
		if len(saver.saved) != 1 || saver.saved[0].mediaType != domain.MediaTypeText || !strings.Contains(saver.saved[0].text, "only words") {
			t.Fatalf("saved text artifact = %#v", saver.saved)
		}
	})

	t.Run("url-less upload receipt and aligned metadata", func(t *testing.T) {
		jobs := memory.NewJobRepo()
		saver := &fakeMusicArtifactSaver{}
		p := &processor{jobs: jobs, artifacts: saver}
		owner := uuid.New()
		job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)

		err := p.saveMusicOutputs(ctx, job, domain.ProviderTaskResult{
			Status: domain.ProviderTaskSucceeded,
			Music: &domain.MusicResult{
				Tracks: []domain.MusicTrackResult{{
					OriginalAudioIndex: 1,
					AudioID:            "native-audio-id",
					DurationSec:        123,
				}},
				Alignment: json.RawMessage(`[{"start":0,"end":1,"url":"https://private.example/alignment","audio_id":"native-secret"}]`),
				Waveform:  json.RawMessage(`{"peaks":[0.1,0.2],"source_url":"https://private.example/waveform"}`),
			},
		})
		if err != nil {
			t.Fatalf("saveMusicOutputs: %v", err)
		}
		stored := mustGetJobForTest(t, ctx, jobs, job.ID)
		params, err := musicgeneration.DecodeJob(stored)
		if err != nil {
			t.Fatal(err)
		}
		if len(params.Result.Artifacts) != 1 || params.Result.Artifacts[0].Kind != "metadata" || params.Result.Artifacts[0].AudioIndex != 0 {
			t.Fatalf("metadata artifacts = %#v, want one job-level metadata artifact", params.Result.Artifacts)
		}
		if len(saver.saved) != 1 || saver.saved[0].mediaType != domain.MediaTypeText {
			t.Fatalf("saved metadata artifact = %#v", saver.saved)
		}
		for _, forbidden := range []string{"native-audio-id", "native-secret", "private.example", "audio_id", "url"} {
			if strings.Contains(saver.saved[0].text, forbidden) {
				t.Fatalf("metadata artifact leaked %q: %s", forbidden, saver.saved[0].text)
			}
			if strings.Contains(string(stored.Params), "private.example") {
				t.Fatalf("job params leaked private URL: %s", string(stored.Params))
			}
		}
	})
}

func TestSaveMusicOutputsModeratesAllPublicMusicTextWithMedia(t *testing.T) {
	ctx := context.Background()

	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{}
	p := &processor{jobs: jobs, artifacts: saver}
	owner := uuid.New()
	job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)

	err := p.saveMusicOutputs(ctx, job, domain.ProviderTaskResult{
		Status: domain.ProviderTaskSucceeded,
		Text:   "provider helper\nsource_task_id native-secret\nhttps://private.example/helper",
		Music: &domain.MusicResult{
			Tracks: []domain.MusicTrackResult{
				{
					OriginalAudioIndex: 2,
					Title:              "First Public Title",
					Lyrics:             "track words",
					Tags:               "synth-pop",
					AudioURL:           "https://private.example/audio.mp3?token=secret",
				},
				{
					OriginalAudioIndex: 5,
					Title:              "Second Public Title",
					Tags:               "ambient",
					ImageURL:           "https://private.example/cover.png?token=secret",
				},
			},
			Lyrics:        []domain.MusicLyricsResult{{Title: "Lyric Title", Text: "line one", Tags: "hook"}},
			UpsampledTags: "upbeat electro",
			BPM:           &domain.MusicBPMResult{Average: 128, Minimum: 120, Maximum: 132},
		},
	})
	if err != nil {
		t.Fatalf("saveMusicOutputs: %v", err)
	}
	stored := mustGetJobForTest(t, ctx, jobs, job.ID)
	params, err := musicgeneration.DecodeJob(stored)
	if err != nil {
		t.Fatal(err)
	}
	if params.Result == nil || !params.Result.Complete {
		t.Fatalf("stored result = %#v, want complete checkpoint", params.Result)
	}
	if got := len(params.Result.Artifacts); got != 3 {
		t.Fatalf("stored artifact count = %d, want variable media plus one text", got)
	}
	if kinds := artifactKindsForTest(params.Result.Artifacts); !equalStrings(kinds, []string{"audio", "image", "text"}) {
		t.Fatalf("artifact kinds = %#v", kinds)
	}
	if len(saver.saved) != 3 || saver.saved[2].mediaType != domain.MediaTypeText {
		t.Fatalf("saved artifacts = %#v", saver.saved)
	}
	text := saver.saved[2].text
	for _, want := range []string{
		"provider helper",
		"First Public Title",
		"Second Public Title",
		"track words",
		"synth-pop",
		"Lyric Title",
		"line one",
		"hook",
		"upbeat electro",
		"128",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("moderation text missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"private.example", "source_task_id", "native-secret", "token=secret", "audio_id"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("moderation text leaked %q: %s", forbidden, text)
		}
		if strings.Contains(string(stored.Params), "private.example") {
			t.Fatalf("job params leaked private URL: %s", string(stored.Params))
		}
	}
}

func TestSaveMusicOutputsPartialReplayPreservesAssociations(t *testing.T) {
	ctx := context.Background()

	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{failAfter: 1}
	p := &processor{jobs: jobs, artifacts: saver}
	owner := uuid.New()
	job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)
	res := domain.ProviderTaskResult{
		Status: domain.ProviderTaskSucceeded,
		Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{
			{OriginalAudioIndex: 4, AudioURL: "https://private.example/a.mp3"},
			{OriginalAudioIndex: 8, AudioURL: "https://private.example/b.mp3"},
		}},
	}

	err := p.saveMusicOutputs(ctx, job, res)
	if err == nil {
		t.Fatal("saveMusicOutputs succeeded, want partial failure")
	}
	stored := mustGetJobForTest(t, ctx, jobs, job.ID)
	params, err := musicgeneration.DecodeJob(stored)
	if err != nil {
		t.Fatal(err)
	}
	if params.Result == nil || params.Result.Complete || len(params.Result.Artifacts) != 1 {
		t.Fatalf("partial checkpoint = %#v, want one incomplete artifact", params.Result)
	}
	firstID := params.Result.Artifacts[0].ID

	saver.failAfter = 0
	err = p.saveMusicOutputs(ctx, stored, res)
	if err != nil {
		t.Fatalf("saveMusicOutputs replay: %v", err)
	}
	replayed := mustGetJobForTest(t, ctx, jobs, job.ID)
	params, err = musicgeneration.DecodeJob(replayed)
	if err != nil {
		t.Fatal(err)
	}
	if params.Result == nil || !params.Result.Complete || len(params.Result.Artifacts) != 2 {
		t.Fatalf("replayed checkpoint = %#v, want two complete artifacts", params.Result)
	}
	if params.Result.Artifacts[0].ID != firstID {
		t.Fatalf("first artifact changed from %s to %s", firstID, params.Result.Artifacts[0].ID)
	}
	if len(saver.saved) != 2 {
		t.Fatalf("saved artifact calls = %d, want 2", len(saver.saved))
	}
	if got := len(replayed.OutputArtifactIDs); got != 2 {
		t.Fatalf("job output artifacts = %d, want 2", got)
	}
	calls := len(saver.saved)
	if err := p.saveMusicOutputs(ctx, replayed, domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded}); err != nil {
		t.Fatalf("complete checkpoint replay without transient URLs: %v", err)
	}
	if len(saver.saved) != calls {
		t.Fatalf("complete checkpoint replay saved %d new artifacts, want 0", len(saver.saved)-calls)
	}
	durable, ok := durableMusicTaskResult(replayed)
	if !ok || durable.Status != domain.ProviderTaskSucceeded || durable.Music == nil || len(durable.Music.Tracks) != 2 {
		t.Fatalf("durableMusicTaskResult = %#v, %v; want succeeded checkpoint music", durable, ok)
	}
	if durable.Music.Tracks[0].AudioURL != "" {
		t.Fatalf("durable checkpoint leaked URL: %#v", durable.Music.Tracks[0])
	}
	if err := p.saveMusicOutputs(ctx, replayed, durable); err != nil {
		t.Fatalf("recovery using actual durable result: %v", err)
	}
	if len(saver.saved) != calls {
		t.Fatal("durable replay wrote duplicate outputs")
	}
}

func TestSaveMusicOutputsRejectsChangedCheckpointAssociations(t *testing.T) {
	ctx := context.Background()

	jobs := memory.NewJobRepo()
	saver := &fakeMusicArtifactSaver{}
	p := &processor{jobs: jobs, artifacts: saver}
	owner := uuid.New()
	job := createMusicJobForTest(t, ctx, jobs, owner, domain.MusicActionGenerate, nil, nil)
	params, err := musicgeneration.DecodeJob(job)
	if err != nil {
		t.Fatal(err)
	}
	existingID := uuid.New()
	params.Result = &musicgeneration.StoredResult{
		Artifacts: []musicgeneration.StoredArtifact{{ID: existingID, AudioIndex: 1, Kind: "video", Format: "mp3"}},
	}
	job.OutputArtifactIDs = []uuid.UUID{existingID}
	updateMusicJobParamsForTest(t, ctx, jobs, job, params)

	err = p.saveMusicOutputs(ctx, job, domain.ProviderTaskResult{
		Status: domain.ProviderTaskSucceeded,
		Music: &domain.MusicResult{Tracks: []domain.MusicTrackResult{{
			OriginalAudioIndex: 1,
			AudioURL:           "https://private.example/audio.mp3",
		}}},
	})
	if err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("saveMusicOutputs error = %v, want checkpoint mismatch", err)
	}
	if len(saver.saved) != 0 {
		t.Fatalf("saved artifacts on checkpoint mismatch = %d, want 0", len(saver.saved))
	}
}

func allowMusicAdmissionForTest(t *testing.T) {
	t.Helper()
	old := musicCandidateAdmitted
	musicCandidateAdmitted = func(string, domain.MusicAction) bool { return true }
	t.Cleanup(func() { musicCandidateAdmitted = old })
}

func createMusicJobForTest(t *testing.T, ctx context.Context, repo domain.JobRepository, owner uuid.UUID, action domain.MusicAction, sources []musicgeneration.Source, uploads []uuid.UUID) *domain.Job {
	t.Helper()
	if uploads == nil {
		if op, ok := musicgeneration.OperationByID(action); ok {
			for len(uploads) < op.MinUploads {
				uploads = append(uploads, uuid.New())
			}
		}
	}
	req := musicgeneration.Request{
		ModelID:          "suno_v6",
		Music:            domain.MusicRequest{Action: action, Prompt: "prompt"},
		Sources:          sources,
		AudioArtifactIDs: uploads,
	}
	candidate, ok := providermodels.MediaCandidateByID(req.ModelID)
	if !ok {
		t.Fatalf("missing candidate %q", req.ModelID)
	}
	price, err := pricingcatalog.MusicCandidateQuote(req.ModelID, string(action), false)
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	params := musicgeneration.JobParams{
		Request:   req,
		Provider:  candidate.Provider,
		ModelCode: candidate.ModelCode,
		ModelName: candidate.Name,
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	rawPrice, err := json.Marshal(price)
	if err != nil {
		t.Fatal(err)
	}
	job := &domain.Job{
		ID:              uuid.New(),
		UserID:          owner,
		AccountID:       owner,
		Source:          "test",
		ResultMode:      domain.ResultModeAccountHistory,
		OperationType:   domain.OperationAudioMusic,
		Modality:        domain.ModalityAudio,
		Status:          domain.JobStatusQueued,
		IdempotencyKey:  uuid.NewString(),
		CorrelationID:   uuid.NewString(),
		Params:          rawParams,
		PricingSnapshot: rawPrice,
		CostEstimate:    price.InternalCredits,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job
}

func addSucceededMusicResult(t *testing.T, ctx context.Context, repo domain.JobRepository, job *domain.Job, result domain.MusicResult) {
	t.Helper()
	params, err := musicgeneration.DecodeJob(job)
	if err != nil {
		t.Fatal(err)
	}
	params.Result = &musicgeneration.StoredResult{Music: result, Complete: true}
	updateMusicJobParamsForTest(t, ctx, repo, job, params)
	if err := repo.UpdateStatus(ctx, job.ID, job.Status, domain.JobStatusSucceeded, "", ""); err != nil {
		t.Fatalf("mark source succeeded: %v", err)
	}
	job.Status = domain.JobStatusSucceeded
}

func updateMusicJobParamsForTest(t *testing.T, ctx context.Context, repo domain.JobRepository, job *domain.Job, params musicgeneration.JobParams) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	job.Params = raw
	if err := repo.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
}

func createProviderTaskForTest(t *testing.T, ctx context.Context, repo domain.ProviderTaskRepository, jobID uuid.UUID, externalID string) {
	t.Helper()
	now := testNowForMusic()
	task := &domain.ProviderTask{
		ID:             uuid.New(),
		JobID:          jobID,
		Provider:       domain.ProviderAPIMart,
		ModelCode:      "suno-v6",
		ExternalID:     externalID,
		AttemptNo:      1,
		Status:         domain.ProviderTaskSucceeded,
		Request:        domain.DurableProviderTaskRequestJSON(),
		IdempotencyKey: "task:" + uuid.NewString(),
		SubmittedAt:    &now,
		CompletedAt:    &now,
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("create provider task: %v", err)
	}
}

func createMusicInputArtifactForTest(t *testing.T, ctx context.Context, repo domain.ArtifactRepository, owner uuid.UUID, durationMS int64) uuid.UUID {
	t.Helper()
	artifact := &domain.Artifact{
		ID:             uuid.New(),
		OwnerUserID:    owner,
		OwnerAccountID: owner,
		Kind:           domain.ArtifactKindInput,
		MediaType:      domain.MediaTypeAudio,
		MimeType:       "audio/mpeg",
		StorageBucket:  "artifacts",
		StorageKey:     "inputs/" + uuid.NewString() + ".mp3",
		SizeBytes:      1024,
		DurationMS:     durationMS,
		Codec:          "mp3",
		Container:      "mp3",
		ProbeStatus:    domain.MediaProbePassed,
		Status:         domain.ArtifactStatusReady,
	}
	if err := repo.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	return artifact.ID
}

type fakeMusicReferenceSigner struct{}

func (fakeMusicReferenceSigner) URL(jobID, artifactID uuid.UUID) (string, error) {
	return "https://provider-reference.test/" + jobID.String() + "/" + artifactID.String(), nil
}

func mustGetJobForTest(t *testing.T, ctx context.Context, repo domain.JobRepository, id uuid.UUID) *domain.Job {
	t.Helper()
	job, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func artifactKindsForTest(in []musicgeneration.StoredArtifact) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		out = append(out, a.Kind)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testNowForMusic() (now time.Time) {
	return time.Unix(1_700_000_000, 0).UTC()
}

type fakeMusicArtifactSaver struct {
	saved     []savedMusicArtifact
	failAfter int
}

type savedMusicArtifact struct {
	id        uuid.UUID
	userID    uuid.UUID
	accountID uuid.UUID
	jobID     uuid.UUID
	kind      domain.ArtifactKind
	mediaType domain.MediaType
	url       string
	text      string
}

func (s *fakeMusicArtifactSaver) SaveRemoteArtifactForAccount(_ context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error) {
	return s.save(userID, accountID, jobID, kind, mediaType, url, "")
}

func (s *fakeMusicArtifactSaver) SaveTextArtifactForAccount(_ context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, text string) (*domain.Artifact, error) {
	return s.save(userID, accountID, jobID, kind, domain.MediaTypeText, "", text)
}

func (s *fakeMusicArtifactSaver) SaveVariantWithMetadata(context.Context, *domain.Artifact, domain.VariantType, string, []byte, domain.ArtifactMediaMetadata) (*domain.ArtifactVariant, error) {
	return nil, errors.New("unexpected SaveVariantWithMetadata")
}

func (s *fakeMusicArtifactSaver) EnsureArtifactScanned(context.Context, uuid.UUID) error {
	return nil
}

func (s *fakeMusicArtifactSaver) save(userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url, text string) (*domain.Artifact, error) {
	if s.failAfter > 0 && len(s.saved) >= s.failAfter {
		return nil, errors.New("injected artifact save failure")
	}
	id := uuid.New()
	var jobIDValue uuid.UUID
	if jobID != nil {
		jobIDValue = *jobID
	}
	s.saved = append(s.saved, savedMusicArtifact{
		id:        id,
		userID:    userID,
		accountID: accountID,
		jobID:     jobIDValue,
		kind:      kind,
		mediaType: mediaType,
		url:       url,
		text:      text,
	})
	return &domain.Artifact{
		ID:             id,
		OwnerUserID:    userID,
		OwnerAccountID: accountID,
		JobID:          jobID,
		Kind:           kind,
		MediaType:      mediaType,
		Status:         domain.ArtifactStatusReady,
	}, nil
}
