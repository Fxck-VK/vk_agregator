package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		lyrics  bool
	}{
		{"suno_v6", 30, false},
		{"suno_v6_wild", 30, false},
		{"suno_v6_mini", 30, false},
		{"lyria_3_5", 40, false},
		{"suno_v6", 30, true},
		{"suno_v6_wild", 30, true},
		{"suno_v6_mini", 30, true},
	} {
		name := model.id
		if model.lyrics {
			name += "_own_lyrics"
		}
		t.Run(name, func(t *testing.T) {
			accountID, key := uuid.New(), uuid.NewString()
			request := musicgeneration.Request{
				ModelID:          model.id,
				Music:            domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Synthetic test music"},
				Sources:          []musicgeneration.Source{},
				AudioArtifactIDs: []uuid.UUID{},
			}
			if model.lyrics {
				custom, instrumental := true, false
				// Match the browser's duplicated fields at the Unicode limits,
				// including four-byte characters: the JSON exceeds 64 KiB.
				text := strings.Repeat("\U0001F3B5", 5000)
				style := strings.Repeat("\U0001F3B5", 1000)
				request.Music = domain.MusicRequest{
					Action: domain.MusicActionGenerate, Custom: &custom,
					Instrumental: &instrumental, AutoLyrics: &instrumental,
					Prompt: text, Lyrics: text, Description: text,
					Style: style, Styles: style, Tags: style,
				}
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
					value := true
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

func TestMusicPreparationKeepsJSONAndFieldBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		music map[string]any
	}{
		{"body_too_large", map[string]any{"action": "generate", "prompt": "test", "description": strings.Repeat("x", 128<<10)}},
		{"lyrics_too_long", map[string]any{"action": "generate", "custom": true, "lyrics": strings.Repeat("я", 5001)}},
		{"unknown_field", map[string]any{"action": "generate", "prompt": "test", "untrusted_option": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, jobs, sessions := newImageJobTestHandler(t)
			r := safeImageMutationRequest(t, sessions, uuid.New(), http.MethodPost, "/web/v1/music-jobs/prepare", map[string]any{"model_id": "suno_v6", "music": tc.music})
			r.Header.Set("X-Idempotency-Key", uuid.NewString())
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || jobs.prepareCalls != 0 || jobs.activateCalls != 0 {
				t.Fatal("invalid request reached job preparation or activation")
			}
		})
	}
}
