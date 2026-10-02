package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestVideoExpansionSubmitAndPollAreRetrySafe(t *testing.T) {
	for _, model := range []string{"flux-3-video", "pixverse-v6", "viduq3", "viduq3-mix", "viduq3-turbo", "kling-video-o1", "MiniMax-H3-Max", "wan3.0-video-prime", "wan2.7", "gemini-omni-flash-preview"} {
		t.Run(model, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/videos/generations":
					calls++
					var body map[string]any
					if json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != model || r.Header.Get("Idempotency-Key") == "" {
						t.Error("invalid submit")
					}
					_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"test-video"}]}`))
				case "GET /v1/tasks/test-video":
					_, _ = w.Write([]byte(`{"code":200,"data":{"id":"test-video","status":"completed","result":{"videos":[{"url":["https://cdn.example/result.mp4"]}]}}}`))
				default:
					t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client(), APIKey: "test-key"})
			req := expansionVideoRequest(model)
			if model == "viduq3" || model == "viduq3-mix" {
				req.InputURLs = []string{"https://cdn.example/input.png"}
			}
			for range 2 {
				task, err := p.Submit(context.Background(), req)
				if err != nil || task.ExternalID != "test-video" {
					t.Fatalf("submit: %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("paid submit called %d times", calls)
			}
			result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "test-video"})
			if err != nil || result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
				t.Fatalf("poll: %+v %v", result, err)
			}
			if estimate, err := p.Estimate(context.Background(), req); err != nil || estimate.AmountCredits <= 0 {
				t.Fatalf("priced model estimate: %+v %v", estimate, err)
			}
		})
	}
}

func TestVideoExpansionEstimateRejectsUnpricedNativeInputs(t *testing.T) {
	p := New(Config{})
	for _, model := range []string{ModelFlux3Video, ModelPixVerseV6, ModelMiniMaxH3Max} {
		req := expansionVideoRequest(model)
		req.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://cdn.example/input.png"}}
		if _, err := p.Estimate(context.Background(), req); err == nil {
			t.Fatalf("%s priced an unwired native input", model)
		}
	}
}

func TestViduOwnedImagesValidatedBeforeUpload(t *testing.T) {
	imageURL := func(width, height int) string {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
			t.Fatal(err)
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	req := expansionVideoRequest("viduq3")
	req.InputURLs = []string{imageURL(128, 128)}
	if err := validateNextVisualRequest(req); err != nil {
		t.Fatal(err)
	}
	if _, err := buildNextVisualBody(req); err == nil {
		t.Fatal("data URI must not be forwarded to Vidu")
	}
	for _, input := range []string{imageURL(127, 128), imageURL(128, 513), "data:image/png;base64,YmFk"} {
		req.InputURLs = []string{input}
		if err := validateNextVisualRequest(req); err == nil {
			t.Fatal("invalid Vidu image accepted")
		}
	}
}

func TestViduUploadsValidatedImagesBeforeOneGeneration(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 128, 128))); err != nil {
		t.Fatal(err)
	}
	input := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	for _, model := range []string{ModelViduQ3, ModelViduQ3Mix} {
		t.Run(model, func(t *testing.T) {
			uploads, generations := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/uploads/images":
					uploads++
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Errorf("invalid multipart: %v", err)
					}
					if r.MultipartForm != nil {
						defer r.MultipartForm.RemoveAll()
					}
					_, _ = w.Write([]byte(`{"url":"https://cdn.example/upload.png"}`))
				case "POST /v1/videos/generations":
					generations++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !reflect.DeepEqual(body["image_urls"], []any{"https://cdn.example/upload.png"}) {
						t.Error("generation did not use uploaded reference")
					}
					_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"vidu-image-test"}]}`))
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client(), APIKey: "test-key"})
			req := expansionVideoRequest(model)
			req.InputURLs = []string{input, "data:image/png;base64,YmFk"}
			if _, err := p.Submit(context.Background(), req); err == nil || uploads != 0 || generations != 0 {
				t.Fatal("invalid second image must prevent every HTTP request")
			}
			req.InputURLs = []string{input}
			for range 2 {
				if _, err := p.Submit(context.Background(), req); err != nil {
					t.Fatal(err)
				}
			}
			if uploads != 1 || generations != 1 {
				t.Fatalf("uploads=%d generations=%d", uploads, generations)
			}
		})
	}
}

func TestViduProAndTurboDocumentedSeedBounds(t *testing.T) {
	for _, model := range []string{ModelViduQ3Pro, ModelViduQ3Turbo} {
		for _, seed := range []int{-2, -1, 0, 4294967295, 4294967296} {
			req := expansionVideoRequest(model)
			req.VideoMedia = &domain.VideoMediaRequest{Seed: &seed}
			raw, err := buildNextVisualBody(req)
			if seed < -1 || int64(seed) > 4294967295 {
				if err == nil {
					t.Fatalf("%s accepted seed %d", model, seed)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s rejected seed %d: %v", model, seed, err)
			}
			if decodeNextVisualBody(t, raw)["seed"] != float64(seed) {
				t.Fatalf("%s changed seed", model)
			}
		}
	}
}

func expansionVideoRequest(model string) domain.ProviderRequest {
	return domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: model, Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Prompt: "A paper boat floats across a pond", IdempotencyKey: "video-expansion-test"}
}

func TestVideoExpansionDocumentedWireRequests(t *testing.T) {
	for _, tc := range []struct {
		model, resolution, ratioField string
		duration                      int
	}{
		{"flux-3-video", "hd", "aspect_ratio", 5},
		{"pixverse-v6", "540p", "size", 5},
		{"viduq3", "720p", "aspect_ratio", 5},
		{"viduq3-mix", "720p", "aspect_ratio", 5},
		{"viduq3-turbo", "720p", "aspect_ratio", 5},
		{"kling-video-o1", "", "aspect_ratio", 5},
		{"MiniMax-H3-Max", "768P", "aspect_ratio", 5},
		{"wan3.0-video-prime", "1080P", "size", 5},
		{"wan2.7", "1080P", "size", 5},
		{"gemini-omni-flash-preview", "720p", "aspect_ratio", 0},
	} {
		t.Run(tc.model, func(t *testing.T) {
			req := expansionVideoRequest(tc.model)
			if tc.model == "viduq3" || tc.model == "viduq3-mix" {
				req.InputURLs = []string{"https://cdn.example/character.png"}
			}
			if !isNextVisualModel(tc.model) {
				t.Fatalf("model is not routed")
			}
			raw, err := buildNextVisualBody(req)
			if err != nil {
				t.Fatal(err)
			}
			body := decodeNextVisualBody(t, raw)
			if body["model"] != tc.model || body["prompt"] != req.Prompt || body[tc.ratioField] != "16:9" {
				t.Fatalf("incorrect wire request: %#v", body)
			}
			if tc.resolution != "" && body["resolution"] != tc.resolution {
				t.Fatalf("wrong resolution: %#v", body)
			}
			if tc.duration > 0 && body["duration"] != float64(tc.duration) {
				t.Fatalf("wrong duration: %#v", body)
			}
			if tc.model == "kling-video-o1" && (body["mode"] != "std" || body["resolution"] != nil) {
				t.Fatalf("O1 uses mode, not resolution: %#v", body)
			}
			if tc.model == "gemini-omni-flash-preview" && (body["duration"] != nil || body["audio"] != nil) {
				t.Fatal("automatic duration/audio must not be sent")
			}
			if tc.model == "MiniMax-H3-Max" && (body["audio"] != nil || body["mode"] != nil || body["watermark"] != nil) {
				t.Fatal("undocumented or resolution-dependent parameters sent")
			}
		})
	}
}

func TestVideoExpansionFramesAndReferences(t *testing.T) {
	start, end := "https://cdn.example/first.png", "https://cdn.example/last.png"
	for _, model := range []string{"flux-3-video", "pixverse-v6", "viduq3-turbo", "kling-video-o1", "MiniMax-H3-Max", "wan3.0-video-prime", "wan2.7"} {
		t.Run(model, func(t *testing.T) {
			req := expansionVideoRequest(model)
			req.VideoMedia = &domain.VideoMediaRequest{Mode: domain.VideoMediaModeImage, StartFrame: &domain.VideoFrame{URL: start}, EndFrame: &domain.VideoFrame{URL: end}}
			raw, err := buildNextVisualBody(req)
			if err != nil {
				t.Fatal(err)
			}
			body := decodeNextVisualBody(t, raw)
			if model == "pixverse-v6" || model == "MiniMax-H3-Max" {
				if body["first_frame_image"] != start || body["last_frame_image"] != end || body["image_urls"] != nil {
					t.Fatalf("wrong frame fields: %#v", body)
				}
			} else if !reflect.DeepEqual(body["image_urls"], []any{start, end}) {
				t.Fatalf("frame order changed: %#v", body)
			}
			if model == "viduq3-turbo" && body["aspect_ratio"] != nil {
				t.Fatal("Vidu frames forbid aspect_ratio")
			}
		})
	}
	for _, model := range []string{"viduq3", "viduq3-mix", "pixverse-v6", "gemini-omni-flash-preview"} {
		req := expansionVideoRequest(model)
		req.VideoMedia = &domain.VideoMediaRequest{Mode: domain.VideoMediaModeReferenceImage, ReferenceImageGroups: []domain.VideoReferenceImageGroup{{URLs: []string{start, start, end}}}}
		raw, err := buildNextVisualBody(req)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		field := "image_urls"
		if model == "pixverse-v6" {
			field = "img_references"
		}
		if !reflect.DeepEqual(decodeNextVisualBody(t, raw)[field], []any{start, start, end}) {
			t.Fatalf("%s lost reference ordering", model)
		}
	}
}

func TestVideoExpansionRejectsInvalidControlsBeforeSubmission(t *testing.T) {
	tests := []struct {
		name, model string
		edit        func(*domain.ProviderRequest)
	}{
		{"flux lower duration", "flux-3-video", func(r *domain.ProviderRequest) { r.DurationSec = 4 }},
		{"flux upper duration", "flux-3-video", func(r *domain.ProviderRequest) { r.DurationSec = 21 }},
		{"pix transition duration", "pixverse-v6", func(r *domain.ProviderRequest) {
			r.DurationSec = 6
			r.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://cdn.example/a.png"}, EndFrame: &domain.VideoFrame{URL: "https://cdn.example/b.png"}}
		}},
		{"vidu missing required references", "viduq3", func(r *domain.ProviderRequest) {}},
		{"vidu minimum duration", "viduq3", func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://cdn.example/a.png"}
			r.DurationSec = 2
		}},
		{"mix unsupported resolution", "viduq3-mix", func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://cdn.example/a.png"}
			r.Resolution = "540p"
		}},
		{"vidu too many references", "viduq3", func(r *domain.ProviderRequest) {
			for range 8 {
				r.InputURLs = append(r.InputURLs, "https://cdn.example/a.png")
			}
		}},
		{"vidu frame aspect conflict", "viduq3-turbo", func(r *domain.ProviderRequest) {
			r.AspectRatio = "16:9"
			r.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://cdn.example/a.png"}}
		}},
		{"O1 duration", "kling-video-o1", func(r *domain.ProviderRequest) { r.DurationSec = 6 }},
		{"O1 audio field", "kling-video-o1", func(r *domain.ProviderRequest) { r.VideoAudio = true }},
		{"H3 no 2K", "MiniMax-H3-Max", func(r *domain.ProviderRequest) { r.Resolution = "2K" }},
		{"H3 minimum duration", "MiniMax-H3-Max", func(r *domain.ProviderRequest) { r.DurationSec = 4 }},
		{"wan minimum duration", "wan2.7", func(r *domain.ProviderRequest) { r.DurationSec = 1 }},
		{"wan media audio conflict", "wan2.7", func(r *domain.ProviderRequest) {
			r.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://cdn.example/a.png"}, Audio: &domain.VideoMediaAudio{URL: "https://cdn.example/a.mp3"}}
		}},
		{"omni duration is automatic", "gemini-omni-flash-preview", func(r *domain.ProviderRequest) { r.DurationSec = 5 }},
		{"omni unsupported aspect", "gemini-omni-flash-preview", func(r *domain.ProviderRequest) { r.AspectRatio = "1:1" }},
		{"omni unsupported resolution", "gemini-omni-flash-preview", func(r *domain.ProviderRequest) { r.Resolution = "1080p" }},
		{"private frame URL", "flux-3-video", func(r *domain.ProviderRequest) {
			r.VideoMedia = &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://127.0.0.1/a.png"}}
		}},
		{"native params injection", "pixverse-v6", func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"extend_from_task_id":"foreign-task"}`) }},
		{"foreign route", "flux-3-video", func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"model_code":"pixverse-v6"}`) }},
		{"unsupported draft", "flux-3-video", func(r *domain.ProviderRequest) { r.Draft = true }},
		{"H3 prompt bound", "MiniMax-H3-Max", func(r *domain.ProviderRequest) { r.Prompt = strings.Repeat("я", 7001) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := expansionVideoRequest(tc.model)
			tc.edit(&req)
			if _, err := buildNextVisualBody(req); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestVideoExpansionBoundaryWireMappings(t *testing.T) {
	for _, tc := range []struct {
		model, res, wire string
		duration         int
	}{
		{"flux-3-video", "1080p", "fhd", 20}, {"pixverse-v6", "360p", "360p", 1},
		{"viduq3-mix", "1080p", "1080p", 1}, {"viduq3-turbo", "540p", "540p", 16},
		{"MiniMax-H3-Max", "1080p", "1080P", 15}, {"wan3.0-video-prime", "480p", "480P", 30},
		{"wan2.7", "720p", "720P", 2},
	} {
		t.Run(tc.model, func(t *testing.T) {
			req := expansionVideoRequest(tc.model)
			req.Resolution = tc.res
			req.DurationSec = tc.duration
			if tc.model == "viduq3-mix" {
				req.InputURLs = []string{"https://cdn.example/a.png"}
			}
			raw, err := buildNextVisualBody(req)
			if err != nil {
				t.Fatal(err)
			}
			if decodeNextVisualBody(t, raw)["resolution"] != tc.wire {
				t.Fatal("incorrect provider resolution")
			}
		})
	}
}
