package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func nextVisualWanRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelWan30Video,
		Prompt:         "A quiet train crosses a rain-soaked city bridge",
		IdempotencyKey: "next-visual:wan",
	}
}

func nextVisualViduRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelViduQ3Pro,
		Prompt:         "A ceramic fox waves from a mossy tree stump",
		IdempotencyKey: "next-visual:vidu",
	}
}

func nextVisualImagenRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      ModelImagen40,
		Prompt:         "A corgi astronaut on the lunar surface",
		IdempotencyKey: "next-visual:imagen",
	}
}

func TestNextVisualModelConstantsAndMembership(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{ModelWan30Video, true},
		{ModelViduQ3Pro, true},
		{ModelImagen40, true},
		{"wan3.0-video-prime", false},
		{"viduq3-turbo", false},
		{"imagen-4.0", false},
	} {
		if got := isNextVisualModel(tc.model); got != tc.want {
			t.Fatalf("isNextVisualModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestBuildNextVisualBodyUsesDocumentedDefaults(t *testing.T) {
	seed := 0
	tests := []struct {
		name string
		req  domain.ProviderRequest
		want map[string]any
	}{
		{
			name: "wan text defaults explicit resolution duration aspect audio and seed",
			req: func() domain.ProviderRequest {
				req := nextVisualWanRequest()
				req.VideoMedia = &domain.VideoMediaRequest{Seed: &seed}
				return req
			}(),
			want: map[string]any{
				"model":      ModelWan30Video,
				"prompt":     "A quiet train crosses a rain-soaked city bridge",
				"resolution": "720P",
				"size":       "16:9",
				"duration":   float64(5),
				"audio":      false,
				"seed":       float64(0),
			},
		},
		{
			name: "vidu text defaults explicit resolution duration aspect audio",
			req:  nextVisualViduRequest(),
			want: map[string]any{
				"model":        ModelViduQ3Pro,
				"prompt":       "A ceramic fox waves from a mossy tree stump",
				"resolution":   "720p",
				"aspect_ratio": "16:9",
				"duration":     float64(5),
				"audio":        false,
			},
		},
		{
			name: "imagen text defaults one output and documented ratio",
			req:  nextVisualImagenRequest(),
			want: map[string]any{
				"model":  ModelImagen40,
				"prompt": "A corgi astronaut on the lunar surface",
				"size":   "16:9",
				"n":      float64(1),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := buildNextVisualBody(tc.req)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeNextVisualBody(t, raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("body = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestBuildNextVisualBodyUsesVideoMediaFrames(t *testing.T) {
	wan := nextVisualWanRequest()
	wan.DurationSec = 12
	wan.Resolution = "1080p"
	wan.AspectRatio = "9:16"
	wan.VideoAudio = true
	wan.VideoMedia = &domain.VideoMediaRequest{
		StartFrame: &domain.VideoFrame{URL: "https://cdn.example/start.png"},
		EndFrame:   &domain.VideoFrame{URL: "https://cdn.example/end.png"},
	}
	raw, err := buildNextVisualBody(wan)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeNextVisualBody(t, raw)
	want := map[string]any{
		"model":      ModelWan30Video,
		"prompt":     "A quiet train crosses a rain-soaked city bridge",
		"resolution": "1080P",
		"size":       "9:16",
		"duration":   float64(12),
		"audio":      true,
		"image_urls": []any{"https://cdn.example/start.png", "https://cdn.example/end.png"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wan frame body = %#v, want %#v", got, want)
	}

	vidu := nextVisualViduRequest()
	vidu.Prompt = ""
	vidu.VideoMedia = &domain.VideoMediaRequest{
		StartFrame: &domain.VideoFrame{URL: "https://cdn.example/first.jpg"},
		EndFrame:   &domain.VideoFrame{URL: "https://cdn.example/last.jpg"},
	}
	raw, err = buildNextVisualBody(vidu)
	if err != nil {
		t.Fatal(err)
	}
	got = decodeNextVisualBody(t, raw)
	want = map[string]any{
		"model":      ModelViduQ3Pro,
		"resolution": "720p",
		"duration":   float64(5),
		"audio":      false,
		"image_urls": []any{"https://cdn.example/first.jpg", "https://cdn.example/last.jpg"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vidu frame body = %#v, want %#v", got, want)
	}
	if _, ok := got["aspect_ratio"]; ok {
		t.Fatal("vidu frame body must omit aspect_ratio")
	}
}

func TestNextVisualRejectsInvalidInputsBeforeHTTP(t *testing.T) {
	longWanPrompt := strings.Repeat("a", 20001)
	longViduPrompt := strings.Repeat("b", 2001)
	seedTooLarge := 2147483648
	for name, req := range map[string]domain.ProviderRequest{
		"wan wrong operation": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.Operation = domain.OperationImageGenerate
			return req
		}(),
		"wan auto duration": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.DurationSec = -1
			return req
		}(),
		"wan long prompt": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.Prompt = longWanPrompt
			return req
		}(),
		"wan seed too large": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.VideoMedia = &domain.VideoMediaRequest{Seed: &seedTooLarge}
			return req
		}(),
		"wan unsupported media": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.VideoMedia = &domain.VideoMediaRequest{KeyFrames: []domain.VideoKeyFrame{{URL: "https://cdn.example/key.png"}}}
			return req
		}(),
		"text mode with frame": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.VideoMedia = &domain.VideoMediaRequest{Mode: domain.VideoMediaModeText, StartFrame: &domain.VideoFrame{URL: "https://cdn.example/start.png"}}
			return req
		}(),
		"wan params native override": func() domain.ProviderRequest {
			req := nextVisualWanRequest()
			req.Params = json.RawMessage(`{"duration":30}`)
			return req
		}(),
		"vidu duration": func() domain.ProviderRequest {
			req := nextVisualViduRequest()
			req.DurationSec = 17
			return req
		}(),
		"vidu resolution": func() domain.ProviderRequest {
			req := nextVisualViduRequest()
			req.Resolution = "4k"
			return req
		}(),
		"vidu explicit frame aspect": func() domain.ProviderRequest {
			req := nextVisualViduRequest()
			req.AspectRatio = "16:9"
			req.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://cdn.example/start.png"}}
			return req
		}(),
		"vidu long prompt": func() domain.ProviderRequest {
			req := nextVisualViduRequest()
			req.Prompt = longViduPrompt
			return req
		}(),
		"vidu private frame": func() domain.ProviderRequest {
			req := nextVisualViduRequest()
			req.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "http://127.0.0.1/start.png"}}
			return req
		}(),
		"imagen output count": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.OutputCount = 2
			return req
		}(),
		"imagen input urls": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.InputURLs = []string{"https://cdn.example/ref.png"}
			return req
		}(),
		"imagen resolution": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.Resolution = "1K"
			return req
		}(),
		"imagen unsupported ratio": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.AspectRatio = "21:9"
			return req
		}(),
		"imagen negative prompt": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.NegativePrompt = "no text"
			return req
		}(),
		"imagen mask param": func() domain.ProviderRequest {
			req := nextVisualImagenRequest()
			req.Params = json.RawMessage(`{"mask":"https://cdn.example/mask.png"}`)
			return req
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("invalid request reached provider")
			}))
			defer srv.Close()
			_, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).submitNextVisual(context.Background(), req)
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest {
				t.Fatalf("error = %v, want invalid request", err)
			}
		})
	}
}

func TestSubmitNextVisualPostsDocumentedEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  domain.ProviderRequest
		path string
	}{
		{"wan", nextVisualWanRequest(), "/videos/generations"},
		{"vidu", nextVisualViduRequest(), "/videos/generations"},
		{"imagen", nextVisualImagenRequest(), "/images/generations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sawBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tc.path {
					t.Fatalf("request = %s %s, want POST %s", r.Method, r.URL.Path, tc.path)
				}
				if got := r.Header.Get("Idempotency-Key"); got != tc.req.IdempotencyKey {
					t.Fatalf("idempotency header = %q, want %q", got, tc.req.IdempotencyKey)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				sawBody = decodeNextVisualBody(t, raw)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_next_visual"}]}`))
			}))
			defer srv.Close()
			task, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).submitNextVisual(context.Background(), tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if task.ExternalID != "task_next_visual" || task.ModelCode != tc.req.ModelCode || task.Status != domain.ProviderTaskPending {
				t.Fatalf("task = %#v", task)
			}
			if sawBody["model"] != tc.req.ModelCode {
				t.Fatalf("posted model = %v, want %s", sawBody["model"], tc.req.ModelCode)
			}
		})
	}
}

func decodeNextVisualBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
