package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestMusicPreparationReplay(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })

	for _, model := range []struct {
		id      string
		credits int64
	}{
		{"suno_v6", 30},
		{"suno_v6_wild", 30},
		{"suno_v6_mini", 30},
		{"lyria_3_5", 40},
	} {
		t.Run(model.id, func(t *testing.T) {
			accountID, key := uuid.New(), uuid.NewString()
			request := musicgeneration.Request{
				ModelID:          model.id,
				Music:            domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Synthetic test music"},
				Sources:          []musicgeneration.Source{},
				AudioArtifactIDs: []uuid.UUID{},
			}
			params, quote, err := musicgeneration.Resolve(request, providermodels.RuntimeRegistry())
			if err != nil {
				t.Fatal(err)
			}
			rawParams, err := json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
			rawPrice, err := json.Marshal(quote)
			if err != nil {
				t.Fatal(err)
			}
			job := &domain.Job{
				ID: uuid.New(), AccountID: accountID, Source: "web",
				OperationType: domain.OperationAudioMusic, Modality: domain.ModalityAudio,
				Status: domain.JobStatusPrepared, IdempotencyKey: key,
				ResultMode:     domain.ResultModeAccountHistory,
				ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
				Params:         rawParams, PricingSnapshot: rawPrice, CostEstimate: quote.InternalCredits,
			}

			for _, scenario := range []string{"new", "replay", "concurrent_replay"} {
				t.Run(scenario, func(t *testing.T) {
					h, jobs, sessions := newImageJobTestHandler(t)
					jobs.prepared = job
					lookup := &imageJobIdempotencyReaderStub{}
					h.deps.ImageJobIdempotency = lookup
					if scenario == "replay" {
						lookup.job = job
					} else if scenario == "concurrent_replay" {
						jobs.prepareErr = domain.ErrConflict
						lookup.results = []imageJobIdempotencyLookup{{err: domain.ErrNotFound}, {job: job}}
					}
					// Use explicit arrays like the browser; Request's omitempty tags
					// would otherwise hide the input that exposed this regression.
					req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/music-jobs/prepare", map[string]any{
						"model_id": model.id, "music": request.Music,
						"sources": []musicgeneration.Source{}, "audio_artifact_ids": []uuid.UUID{},
					})
					req.Header.Set("X-Idempotency-Key", key)
					rec := httptest.NewRecorder()
					h.Routes().ServeHTTP(rec, req)
					if rec.Code != http.StatusCreated {
						t.Fatalf("preparation status = %d, want 201", rec.Code)
					}
					var response struct {
						Job       safeMusicJob `json:"job"`
						Balance   int64        `json:"balance"`
						CanAfford bool         `json:"can_afford"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if response.Job.ID != job.ID || response.Job.Status != domain.JobStatusPrepared || response.Job.ModelID != model.id || response.Job.Action != domain.MusicActionGenerate || response.Job.CostEstimate != model.credits || response.Balance != 104 || !response.CanAfford {
						t.Fatal("preparation changed job identity, status or server-owned price/balance")
					}
					wantPrepareCalls := 1
					if scenario == "replay" {
						wantPrepareCalls = 0
					}
					if jobs.prepareCalls != wantPrepareCalls || jobs.activateCalls != 0 {
						t.Fatal("preparation repeated job creation or activated a paid job")
					}
				})
			}

			for _, change := range []struct {
				name   string
				mutate func(*musicgeneration.Request)
			}{
				{"prompt", func(r *musicgeneration.Request) { r.Music.Prompt = "Different synthetic music" }},
				{"model", func(r *musicgeneration.Request) { r.ModelID = "suno_v6_other" }},
				{"source", func(r *musicgeneration.Request) {
					r.Sources = []musicgeneration.Source{{JobID: uuid.New(), AudioIndex: 1}}
				}},
				{"upload", func(r *musicgeneration.Request) { r.AudioArtifactIDs = []uuid.UUID{uuid.New()} }},
				{"explicit_option", func(r *musicgeneration.Request) {
					value := false
					r.Music.Instrumental = &value
				}},
			} {
				t.Run("changed_"+change.name, func(t *testing.T) {
					changed := request
					change.mutate(&changed)
					if musicReplayMatches(job, accountID, key, changed) {
						t.Fatal("changed request accepted as the same preparation")
					}
				})
			}
			if musicReplayMatches(job, uuid.New(), key, request) || musicReplayMatches(job, accountID, uuid.NewString(), request) {
				t.Fatal("preparation accepted a different owner or idempotency key")
			}
		})
	}
}
