package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func klingV3ProviderRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelKlingV3,
		Prompt:         "A calm product video",
		NegativePrompt: "blur",
		DurationSec:    7,
		Resolution:     "1080p",
		AspectRatio:    "16:9",
		InputURLs:      []string{"https://cdn.test/start.png", "https://cdn.test/end.png"},
		VideoAudio:     true,
		IdempotencyKey: "kling-v3-contract",
	}
}

func klingMotionVideoRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:            domain.OperationVideoGenerate,
		Modality:             domain.ModalityVideo,
		ModelCode:            ModelKling26Motion,
		Prompt:               "Subtle camera move",
		DurationSec:          9,
		Resolution:           "pro",
		InputURLs:            []string{"https://cdn.test/character.png"},
		ReferenceVideoURL:    "https://cdn.test/source.mp4",
		CharacterOrientation: "image",
		KeepOriginalSound:    true,
		IdempotencyKey:       "kling-motion-contract",
	}
}

func veo31ProviderRequest(model string) domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      model,
		Prompt:         "Editorial camera move",
		DurationSec:    8,
		Resolution:     "1080p",
		AspectRatio:    "9:16",
		IdempotencyKey: "veo31-contract:" + model,
	}
}

func TestKlingVeoVideoDocumentedRequestBodies(t *testing.T) {
	tests := []struct {
		name string
		req  domain.ProviderRequest
		want map[string]any
	}{
		{
			name: "kling v3",
			req:  klingV3ProviderRequest(),
			want: map[string]any{
				"model":           ModelKlingV3,
				"prompt":          "A calm product video",
				"mode":            "pro",
				"duration":        float64(7),
				"aspect_ratio":    "16:9",
				"image_urls":      []any{"https://cdn.test/start.png", "https://cdn.test/end.png"},
				"negative_prompt": "blur",
				"audio":           true,
				"watermark":       false,
				"nsfw_check":      true,
			},
		},
		{
			name: "motion control",
			req:  klingMotionVideoRequest(),
			want: map[string]any{
				"model":                 ModelKling26Motion,
				"prompt":                "Subtle camera move",
				"image_url":             "https://cdn.test/character.png",
				"video_url":             "https://cdn.test/source.mp4",
				"mode":                  "pro",
				"character_orientation": "image",
				"keep_original_sound":   "yes",
				"nsfw_check":            true,
				"watermark_info":        map[string]any{"enabled": false},
			},
		},
		{
			name: "veo fast two frames",
			req: func() domain.ProviderRequest {
				req := veo31ProviderRequest(ModelVeo31Fast)
				req.InputURLs = []string{"https://cdn.test/start.png", "https://cdn.test/end.png"}
				return req
			}(),
			want: map[string]any{
				"model":             ModelVeo31Fast,
				"prompt":            "Editorial camera move",
				"duration":          float64(8),
				"aspect_ratio":      "9:16",
				"resolution":        "1080p",
				"image_urls":        []any{"https://cdn.test/start.png", "https://cdn.test/end.png"},
				"generation_type":   "frame",
				"official_fallback": false,
				"nsfw_check":        true,
				"enable_gif":        false,
			},
		},
		{
			name: "veo fast three references",
			req: func() domain.ProviderRequest {
				req := veo31ProviderRequest(ModelVeo31Fast)
				req.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png", "https://cdn.test/c.png"}
				return req
			}(),
			want: map[string]any{
				"model":             ModelVeo31Fast,
				"prompt":            "Editorial camera move",
				"duration":          float64(8),
				"aspect_ratio":      "9:16",
				"resolution":        "1080p",
				"image_urls":        []any{"https://cdn.test/a.png", "https://cdn.test/b.png", "https://cdn.test/c.png"},
				"generation_type":   "reference",
				"official_fallback": false,
				"nsfw_check":        true,
				"enable_gif":        false,
			},
		},
		{
			name: "veo quality frame",
			req: func() domain.ProviderRequest {
				req := veo31ProviderRequest(ModelVeo31Quality)
				req.InputURLs = []string{"https://cdn.test/start.png"}
				return req
			}(),
			want: map[string]any{
				"model":             ModelVeo31Quality,
				"prompt":            "Editorial camera move",
				"duration":          float64(8),
				"aspect_ratio":      "9:16",
				"resolution":        "1080p",
				"image_urls":        []any{"https://cdn.test/start.png"},
				"generation_type":   "frame",
				"official_fallback": false,
				"nsfw_check":        true,
				"enable_gif":        false,
			},
		},
		{
			name: "veo lite text only",
			req:  veo31ProviderRequest(ModelVeo31Lite),
			want: map[string]any{
				"model":        ModelVeo31Lite,
				"prompt":       "Editorial camera move",
				"duration":     float64(8),
				"aspect_ratio": "9:16",
				"resolution":   "1080p",
				"nsfw_check":   true,
				"enable_gif":   false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				if r.Method != http.MethodPost || r.URL.Path != "/videos/generations" || r.Header.Get("Idempotency-Key") != tc.req.IdempotencyKey {
					t.Fatalf("unexpected submit call %s %s", r.Method, r.URL.Path)
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("request JSON differs\ngot  %#v\nwant %#v", got, tc.want)
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"kling-veo-task"}]}`))
			}))
			defer srv.Close()

			task, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).submitKlingVeoVideo(context.Background(), tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if task.ExternalID != "kling-veo-task" || posts != 1 {
				t.Fatalf("task=%+v posts=%d", task, posts)
			}
		})
	}
}

func TestKlingVeoVideoRejectsUnsupportedInputsBeforeHTTP(t *testing.T) {
	cases := []struct {
		name string
		req  domain.ProviderRequest
		edit func(*domain.ProviderRequest)
	}{
		{"kling duration low", klingV3ProviderRequest(), func(r *domain.ProviderRequest) { r.DurationSec = 2 }},
		{"kling duration high", klingV3ProviderRequest(), func(r *domain.ProviderRequest) { r.DurationSec = 16 }},
		{"kling bad aspect", klingV3ProviderRequest(), func(r *domain.ProviderRequest) { r.AspectRatio = "4:3" }},
		{"kling too many images", klingV3ProviderRequest(), func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png", "https://cdn.test/c.png"}
		}},
		{"motion missing image", klingMotionVideoRequest(), func(r *domain.ProviderRequest) { r.InputURLs = nil }},
		{"motion private video", klingMotionVideoRequest(), func(r *domain.ProviderRequest) { r.ReferenceVideoURL = "https://127.0.0.1/source.mp4" }},
		{"motion bad orientation", klingMotionVideoRequest(), func(r *domain.ProviderRequest) { r.CharacterOrientation = "front" }},
		{"motion image duration high", klingMotionVideoRequest(), func(r *domain.ProviderRequest) { r.DurationSec = 11 }},
		{"veo bad duration", veo31ProviderRequest(ModelVeo31Fast), func(r *domain.ProviderRequest) { r.DurationSec = 6 }},
		{"veo bad aspect", veo31ProviderRequest(ModelVeo31Quality), func(r *domain.ProviderRequest) { r.AspectRatio = "1:1" }},
		{"veo quality three images", veo31ProviderRequest(ModelVeo31Quality), func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png", "https://cdn.test/c.png"}
		}},
		{"veo lite image", veo31ProviderRequest(ModelVeo31Lite), func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://cdn.test/a.png"} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("invalid request reached APIMart")
			}))
			defer srv.Close()
			tc.edit(&tc.req)

			_, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).submitKlingVeoVideo(context.Background(), tc.req)
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest {
				t.Fatalf("error=%v, want invalid_request", err)
			}
		})
	}
}

func TestKlingVeoVideoCostsAreProviderCreditCeilings(t *testing.T) {
	tests := []struct {
		name string
		req  domain.ProviderRequest
		want int64
	}{
		{"kling 720 no audio", func() domain.ProviderRequest {
			req := klingV3ProviderRequest()
			req.Resolution, req.DurationSec, req.VideoAudio = "720p", 3, false
			return req
		}(), 3},
		{"kling 1080 audio", func() domain.ProviderRequest {
			req := klingV3ProviderRequest()
			req.Resolution, req.DurationSec, req.VideoAudio = "1080p", 3, true
			return req
		}(), 5},
		{"kling 4k", func() domain.ProviderRequest {
			req := klingV3ProviderRequest()
			req.Resolution, req.DurationSec = "4k", 3
			return req
		}(), 13},
		{"motion std", func() domain.ProviderRequest {
			req := klingMotionVideoRequest()
			req.Resolution, req.DurationSec = "std", 3
			return req
		}(), 2},
		{"motion pro", func() domain.ProviderRequest {
			req := klingMotionVideoRequest()
			req.Resolution, req.DurationSec = "pro", 3
			return req
		}(), 3},
		{"veo fast 4k", func() domain.ProviderRequest {
			req := veo31ProviderRequest(ModelVeo31Fast)
			req.Resolution = "4k"
			return req
		}(), 7},
		{"veo quality 1080", veo31ProviderRequest(ModelVeo31Quality), 10},
		{"veo lite 720", func() domain.ProviderRequest {
			req := veo31ProviderRequest(ModelVeo31Lite)
			req.Resolution = "720p"
			return req
		}(), 1},
		{"veo lite 4k", func() domain.ProviderRequest {
			req := veo31ProviderRequest(ModelVeo31Lite)
			req.Resolution = "4k"
			return req
		}(), 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := klingVeoVideoCostCredits(tc.req); got != tc.want {
				t.Fatalf("cost=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestKlingVeoVideoUncertainSubmitUsesOnePaidCall(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := veo31ProviderRequest(ModelVeo31Fast)
	for i := 0; i < 2; i++ {
		_, err := p.submitUnversionedOnce(context.Background(), req, p.submitKlingVeoVideo)
		var providerErr *Error
		if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrSubmitIndeterminate {
			t.Fatalf("attempt %d error=%v, want submit_indeterminate", i, err)
		}
	}
	if calls != 1 {
		t.Fatalf("paid submits=%d, want 1", calls)
	}
}

func TestKlingVeoVideoPollCompletedTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks/kling-veo-task" {
			t.Fatalf("unexpected poll call %s %s", r.Method, r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"id":"kling-veo-task","status":"completed","result":{"videos":[{"url":["https://cdn.test/output.mp4"]}]}}}`)
	}))
	defer srv.Close()

	result, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "kling-veo-task"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 || result.OutputURLs[0] != "https://cdn.test/output.mp4" {
		t.Fatalf("unexpected poll result: %+v", result)
	}
}
