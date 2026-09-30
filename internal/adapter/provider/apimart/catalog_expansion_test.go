package apimart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func expansionRequest(model string) domain.ProviderRequest {
	r := domain.ProviderRequest{ModelCode: model, Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Prompt: "A tree in the wind", IdempotencyKey: "expansion:" + model}
	if model == "gemini-2.5-flash-image-preview" {
		r.Operation, r.Modality = domain.OperationImageGenerate, domain.ModalityImage
	}
	return r
}

func TestCatalogExpansionWireContracts(t *testing.T) {
	for _, tc := range []struct {
		model  string
		want   map[string]any
		absent []string
	}{
		{"gemini-2.5-flash-image-preview", map[string]any{"resolution": "1K", "size": "16:9", "n": float64(1), "official_fallback": false, "nsfw_check": true}, []string{"audio", "duration"}},
		{"grok-imagine-1.5-video-ext", map[string]any{"resolution": "720p", "size": "16:9", "duration": float64(6), "nsfw_check": true}, []string{"audio", "generate_audio", "mode", "aspect_ratio"}},
		{"kling-v2-6", map[string]any{"mode": "std", "aspect_ratio": "16:9", "duration": float64(5), "audio": false}, []string{"resolution", "size", "generate_audio"}},
		{"seedance-2.0", map[string]any{"resolution": "720p", "size": "16:9", "duration": float64(5), "generate_audio": false}, []string{"audio", "mode", "aspect_ratio"}},
		{"seedance-2.0-mini", map[string]any{"resolution": "720p", "size": "16:9", "duration": float64(5), "generate_audio": false}, []string{"audio", "mode", "aspect_ratio"}},
	} {
		t.Run(tc.model, func(t *testing.T) {
			raw, err := buildNextVisualBody(expansionRequest(tc.model))
			if err != nil {
				t.Fatal(err)
			}
			body := decodeNextVisualBody(t, raw)
			if body["model"] != tc.model {
				t.Fatal("wrong native model")
			}
			for k, v := range tc.want {
				if body[k] != v {
					t.Errorf("%s = %v, want %v", k, body[k], v)
				}
			}
			for _, k := range tc.absent {
				if _, ok := body[k]; ok {
					t.Errorf("unexpected wire field %s", k)
				}
			}
		})
	}
}

func TestCatalogExpansionRejectsBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, model string
		mutate      func(*domain.ProviderRequest)
	}{
		{"nano prompt", "gemini-2.5-flash-image-preview", func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("я", 1001) }},
		{"nano resolution", "gemini-2.5-flash-image-preview", func(r *domain.ProviderRequest) { r.Resolution = "2K" }},
		{"nano count", "gemini-2.5-flash-image-preview", func(r *domain.ProviderRequest) { r.OutputCount = 2 }},
		{"nano fallback injection", "gemini-2.5-flash-image-preview", func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"official_fallback":true}`) }},
		{"grok short", "grok-imagine-1.5-video-ext", func(r *domain.ProviderRequest) { r.DurationSec = 5 }},
		{"grok long", "grok-imagine-1.5-video-ext", func(r *domain.ProviderRequest) { r.DurationSec = 16 }},
		{"grok audio unknown", "grok-imagine-1.5-video-ext", func(r *domain.ProviderRequest) { r.VideoAudio = true }},
		{"grok aspect", "grok-imagine-1.5-video-ext", func(r *domain.ProviderRequest) { r.AspectRatio = "21:9" }},
		{"kling sparse duration", "kling-v2-6", func(r *domain.ProviderRequest) { r.DurationSec = 6 }},
		{"kling std sound", "kling-v2-6", func(r *domain.ProviderRequest) { r.VideoAudio = true }},
		{"kling prompt", "kling-v2-6", func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("я", 2501) }},
		{"seedance short", "seedance-2.0", func(r *domain.ProviderRequest) { r.DurationSec = 3 }},
		{"seedance prompt", "seedance-2.0", func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("я", 4001) }},
		{"mini 1080", "seedance-2.0-mini", func(r *domain.ProviderRequest) { r.Resolution = "1080p" }},
		{"unpriced native input", "seedance-2.0", func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"video_urls":["https://example.com/a.mp4"]}`)
		}},
		{"unpriced typed input", "seedance-2.0", func(r *domain.ProviderRequest) {
			r.VideoMedia = &domain.VideoMediaRequest{ReferenceVideos: []domain.VideoReferenceVideo{{URL: "https://example.com/a.mp4"}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := expansionRequest(tc.model)
			tc.mutate(&r)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(500) }))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
			if _, err := p.Submit(context.Background(), r); err == nil {
				t.Error("invalid request accepted")
			}
			if calls != 0 {
				t.Errorf("invalid request made %d HTTP calls", calls)
			}
			if _, err := p.Estimate(context.Background(), r); err == nil {
				t.Error("invalid request priced")
			}
		})
	}
}

func TestCatalogExpansionSubmitAndEstimate(t *testing.T) {
	for _, model := range []string{"gemini-2.5-flash-image-preview", "grok-imagine-1.5-video-ext", "kling-v2-6", "seedance-2.0", "seedance-2.0-mini"} {
		t.Run(model, func(t *testing.T) {
			r := expansionRequest(model)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				calls++
				path := "/v1/videos/generations"
				if r.Modality == domain.ModalityImage {
					path = "/v1/images/generations"
				}
				if q.Method != "POST" || q.URL.Path != path || q.Header.Get("Idempotency-Key") != r.IdempotencyKey {
					t.Errorf("wrong endpoint/header: %s %s", q.Method, q.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(q.Body).Decode(&body); err != nil || body["model"] != model {
					t.Error("wrong body")
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"expansion-task"}]}`))
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			if cost, err := p.Estimate(context.Background(), r); err != nil || cost.AmountCredits <= 0 {
				t.Fatalf("estimate=%+v error=%v", cost, err)
			}
			for i := 0; i < 2; i++ {
				if task, err := p.Submit(context.Background(), r); err != nil || task.ExternalID != "expansion-task" {
					t.Fatalf("submit task=%+v error=%v", task, err)
				}
			}
			if calls != 1 {
				t.Fatalf("duplicate paid submit: %d calls", calls)
			}
		})
	}
}
