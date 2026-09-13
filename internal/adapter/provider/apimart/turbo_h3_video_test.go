package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func kling30TurboRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelKling30Turbo,
		Prompt:         "A corgi runs along the sea shore",
		DurationSec:    5,
		Resolution:     "1080p",
		AspectRatio:    "16:9",
		IdempotencyKey: "provider_submit:kling30-turbo:test",
	}
}

func minimaxH3Request() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelMiniMaxH3,
		Prompt:         "A boy playing basketball by the sea at dusk",
		DurationSec:    5,
		Resolution:     "2K",
		AspectRatio:    "16:9",
		IdempotencyKey: "provider_submit:minimax-h3:test",
	}
}

func TestTurboH3VideoDocumentedBodiesAndPolling(t *testing.T) {
	tests := []struct {
		name string
		req  domain.ProviderRequest
		want map[string]any
	}{
		{
			name: "kling 3 turbo text to video",
			req: func() domain.ProviderRequest {
				req := kling30TurboRequest()
				req.KeepOriginalSound = true
				return req
			}(),
			want: map[string]any{
				"model":        "kling-3.0-turbo",
				"prompt":       "A corgi runs along the sea shore",
				"aspect_ratio": "16:9",
				"resolution":   "1080p",
				"duration":     float64(5),
			},
		},
		{
			name: "kling 3 turbo first frame only",
			req: func() domain.ProviderRequest {
				req := kling30TurboRequest()
				req.Prompt = ""
				req.Resolution = "720p"
				req.AspectRatio = ""
				req.InputURLs = []string{"https://cdn.test/first.png"}
				return req
			}(),
			want: map[string]any{
				"model":             "kling-3.0-turbo",
				"first_frame_image": "https://cdn.test/first.png",
				"resolution":        "720p",
				"duration":          float64(5),
			},
		},
		{
			name: "minimax h3 text to video",
			req:  minimaxH3Request(),
			want: map[string]any{
				"model":        "MiniMax-H3",
				"prompt":       "A boy playing basketball by the sea at dusk",
				"duration":     float64(5),
				"resolution":   "2K",
				"aspect_ratio": "16:9",
			},
		},
		{
			name: "minimax h3 first frame",
			req: func() domain.ProviderRequest {
				req := minimaxH3Request()
				req.Resolution = "768P"
				req.AspectRatio = ""
				req.InputURLs = []string{"https://cdn.test/ramen.png"}
				return req
			}(),
			want: map[string]any{
				"model":             "MiniMax-H3",
				"prompt":            "A boy playing basketball by the sea at dusk",
				"first_frame_image": "https://cdn.test/ramen.png",
				"duration":          float64(5),
				"resolution":        "768P",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/tasks/turbo-h3-task" {
					_, _ = w.Write([]byte(`{"code":200,"data":{"id":"turbo-h3-task","status":"completed","result":{"videos":[{"url":["https://cdn.test/output.mp4"]}]}}}`))
					return
				}
				posts.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/videos/generations" || r.Header.Get("Authorization") != "Bearer fixture-key" || r.Header.Get("Idempotency-Key") != tc.req.IdempotencyKey {
					t.Fatalf("unexpected submit call %s %s", r.Method, r.URL.Path)
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("documented request mismatch\ngot  %#v\nwant %#v", got, tc.want)
				}
				if _, ok := got["image_urls"]; ok {
					t.Fatal("H3/Turbo first frame routes must not send image_urls")
				}
				if _, ok := got["nsfw_check"]; ok {
					t.Fatal("H3/Turbo must omit nsfw_check until upstream surcharge is priced")
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"turbo-h3-task"}]}`))
			}))
			defer srv.Close()

			p := New(Config{APIKey: "fixture-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			for range 2 {
				task, err := p.Submit(context.Background(), tc.req)
				if err != nil {
					t.Fatal(err)
				}
				if task.ExternalID != "turbo-h3-task" || task.ModelCode != tc.req.ModelCode {
					t.Fatalf("task=%+v", task)
				}
			}
			if posts.Load() != 1 {
				t.Fatalf("paid submits=%d, want 1", posts.Load())
			}
			result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "turbo-h3-task"})
			if err != nil || result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
				t.Fatalf("poll result=%+v err=%v", result, err)
			}
		})
	}
}

func TestTurboH3VideoUploadsDataURIAsFirstFrame(t *testing.T) {
	const uploaded = "https://upload.apimart.ai/f/image/first-frame.png"
	req := minimaxH3Request()
	req.InputURLs = []string{turboH3PNGDataURL(t, 300, 300)}
	req.AspectRatio = ""
	var uploadSeen, submitSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/images":
			uploadSeen = true
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if header.Filename != "first-frame.png" {
				t.Fatalf("upload filename=%q", header.Filename)
			}
			_, _ = w.Write([]byte(`{"url":"` + uploaded + `"}`))
		case "/videos/generations":
			submitSeen = true
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got["first_frame_image"] != uploaded || strings.Contains(asJSON(t, got), "data:image") {
				t.Fatalf("first frame was not uploaded before submit: %#v", got)
			}
			_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"h3-upload"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	task, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).Submit(context.Background(), req)
	if err != nil || task.ExternalID != "h3-upload" {
		t.Fatalf("submit task=%+v err=%v", task, err)
	}
	if !uploadSeen || !submitSeen {
		t.Fatalf("uploadSeen=%v submitSeen=%v", uploadSeen, submitSeen)
	}
}

func TestTurboH3VideoRejectsUnsupportedInputsBeforeHTTP(t *testing.T) {
	cases := map[string]struct {
		base func() domain.ProviderRequest
		edit func(*domain.ProviderRequest)
	}{
		"kling duration low":  {kling30TurboRequest, func(r *domain.ProviderRequest) { r.DurationSec = 2 }},
		"kling duration high": {kling30TurboRequest, func(r *domain.ProviderRequest) { r.DurationSec = 16 }},
		"kling resolution":    {kling30TurboRequest, func(r *domain.ProviderRequest) { r.Resolution = "4k" }},
		"kling aspect":        {kling30TurboRequest, func(r *domain.ProviderRequest) { r.AspectRatio = "4:3" }},
		"kling blank text":    {kling30TurboRequest, func(r *domain.ProviderRequest) { r.Prompt = " " }},
		"kling prompt length": {kling30TurboRequest, func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("x", 3073) }},
		"too many first frames": {kling30TurboRequest, func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}
		}},
		"private first frame": {kling30TurboRequest, func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://127.0.0.1/a.png"} }},
		"negative prompt":     {kling30TurboRequest, func(r *domain.ProviderRequest) { r.NegativePrompt = "blur" }},
		"output count":        {kling30TurboRequest, func(r *domain.ProviderRequest) { r.OutputCount = 2 }},
		"video audio":         {kling30TurboRequest, func(r *domain.ProviderRequest) { r.VideoAudio = true }},
		"reference video":     {kling30TurboRequest, func(r *domain.ProviderRequest) { r.ReferenceVideoURL = "https://cdn.test/ref.mp4" }},
		"native image urls": {minimaxH3Request, func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"image_urls":["https://cdn.test/ref.png"]}`)
		}},
		"native video urls": {minimaxH3Request, func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"video_urls":["https://cdn.test/ref.mp4"]}`)
		}},
		"native audio urls": {minimaxH3Request, func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"audio_urls":["https://cdn.test/ref.mp3"]}`)
		}},
		"native last frame": {minimaxH3Request, func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"last_frame_image":"https://cdn.test/end.png"}`)
		}},
		"native image roles": {minimaxH3Request, func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"image_with_roles":[{"url":"https://cdn.test/a.png","role":"reference_image"}]}`)
		}},
		"native nsfw":         {minimaxH3Request, func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"nsfw_check":true}`) }},
		"native watermark":    {minimaxH3Request, func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"watermark":true}`) }},
		"native webhook":      {minimaxH3Request, func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"webhook":"https://example.test/hook"}`) }},
		"h3 blank prompt":     {minimaxH3Request, func(r *domain.ProviderRequest) { r.Prompt = " " }},
		"h3 prompt length":    {minimaxH3Request, func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("x", 7001) }},
		"h3 duration low":     {minimaxH3Request, func(r *domain.ProviderRequest) { r.DurationSec = 3 }},
		"h3 resolution":       {minimaxH3Request, func(r *domain.ProviderRequest) { r.Resolution = "1080p" }},
		"h3 aspect":           {minimaxH3Request, func(r *domain.ProviderRequest) { r.AspectRatio = "3:2" }},
		"h3 malformed params": {minimaxH3Request, func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) }},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			req := tc.base()
			tc.edit(&req)
			_, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).Submit(context.Background(), req)
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest || calls.Load() != 0 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestTurboH3VideoEstimateDimensions(t *testing.T) {
	tests := []struct {
		name string
		req  domain.ProviderRequest
		want int64
	}{
		{"kling 720 three seconds", func() domain.ProviderRequest {
			req := kling30TurboRequest()
			req.Resolution = "720p"
			req.DurationSec = 3
			return req
		}(), 4},
		{"kling 1080 fifteen seconds", func() domain.ProviderRequest {
			req := kling30TurboRequest()
			req.DurationSec = 15
			return req
		}(), 22},
		{"h3 768 four seconds", func() domain.ProviderRequest {
			req := minimaxH3Request()
			req.Resolution = "768P"
			req.DurationSec = 4
			return req
		}(), 3},
		{"h3 2k five seconds", minimaxH3Request(), 5},
	}
	p := New(Config{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			estimate, err := p.Estimate(context.Background(), tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if estimate.AmountCredits != tc.want || estimate.Currency != "credits" || !estimate.Estimated {
				t.Fatalf("estimate=%+v want credits=%d estimated=true", estimate, tc.want)
			}
		})
	}

	req := minimaxH3Request()
	req.DurationSec = 16
	if _, err := p.Estimate(context.Background(), req); err == nil {
		t.Fatal("estimate accepted unsupported duration")
	}
}

func TestTurboH3VideoCapabilities(t *testing.T) {
	caps, err := New(Config{}).Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{ModelKling30Turbo, ModelMiniMaxH3} {
		found := false
		for _, cap := range caps {
			if cap.Operation == domain.OperationVideoGenerate && cap.Modality == domain.ModalityVideo && cap.ModelCode == model && cap.SupportsPolling && cap.MaxDurationSec == 15 {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing video capability for %s", model)
		}
	}
}

func turboH3PNGDataURL(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
