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

func omniRequest(ext bool) domain.ProviderRequest {
	model, duration := "gemini-omni-1.1-flash", 10
	if ext {
		model, duration = "gemini-omni-1.1-flash-ext", 6
	}
	return domain.ProviderRequest{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo,
		ModelCode: model, Prompt: "Synthetic cinematic scene", DurationSec: duration, Resolution: "1080p", AspectRatio: "9:16", IdempotencyKey: "omni-contract"}
}

// Wire expectations follow the two APIMart generation pages, including their
// different duration and image-count contracts; no paid API calls are made.
func TestOmniVideoDocumentedRequestsAndPolling(t *testing.T) {
	for _, ext := range []bool{false, true} {
		for _, count := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("ext=%t/images=%d", ext, count), func(t *testing.T) {
				req := omniRequest(ext)
				want := map[string]any{"model": req.ModelCode, "prompt": req.Prompt, "resolution": "1080p", "aspect_ratio": "9:16"}
				if ext {
					want["duration"], want["nsfw_check"] = float64(6), true
				}
				if count > 0 {
					images := make([]any, count)
					for i := range images {
						images[i] = fmt.Sprintf("https://example.com/image-%d.png", i)
						req.InputURLs = append(req.InputURLs, images[i].(string))
					}
					want["image_urls"] = images
					if ext {
						want["generation_type"] = "frame"
						if count == 3 {
							want["generation_type"] = "reference"
						}
					}
				}
				posts := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer synthetic" {
						t.Error("missing authorization")
					}
					if r.Method == "GET" && r.URL.Path == "/tasks/omni-task" {
						fmt.Fprint(w, `{"code":200,"data":{"status":"completed","credits_cost":4,"result":{"videos":[{"url":["https://example.com/output.mp4"],"expires_at":1788518400}]}}}`)
						return
					}
					posts++
					if r.Method != "POST" || r.URL.Path != "/videos/generations" {
						t.Error("wrong endpoint")
					}
					var got map[string]any
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("request fields differ from documented contract: got %v; want %v", got, want)
					}
					fmt.Fprint(w, `{"code":200,"data":[{"status":"submitted","task_id":"omni-task"}]}`)
				}))
				defer srv.Close()
				p := New(Config{APIKey: "synthetic", BaseURL: srv.URL, HTTPClient: srv.Client()})
				for i := 0; i < 2; i++ {
					if _, err := p.Submit(context.Background(), req); err != nil {
						t.Fatal(err)
					}
				}
				if posts != 1 {
					t.Fatalf("paid submit repeated: %d", posts)
				}
				result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "omni-task"})
				if err != nil || result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
					t.Fatalf("documented polling response not handled: %v", err)
				}
			})
		}
	}
}

func TestOmniVideoRejectsUnsupportedInputsBeforeSubmission(t *testing.T) {
	for _, tc := range []struct {
		name string
		ext  bool
		edit func(*domain.ProviderRequest)
	}{
		{"standard fixed duration", false, func(r *domain.ProviderRequest) { r.DurationSec = 6 }},
		{"ext five seconds", true, func(r *domain.ProviderRequest) { r.DurationSec = 5 }},
		{"ext two images", true, func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"https://example.com/a.png", "https://example.com/b.png"}
		}},
		{"unsupported resolution", false, func(r *domain.ProviderRequest) { r.Resolution = "480p" }},
		{"unsupported ratio", true, func(r *domain.ProviderRequest) { r.AspectRatio = "1:1" }},
		{"private image", false, func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://127.0.0.1/a.png"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid request reached provider") }))
			defer srv.Close()
			req := omniRequest(tc.ext)
			tc.edit(&req)
			p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
			_, err := p.Submit(context.Background(), req)
			var pe *Error
			if !errors.As(err, &pe) || pe.Class != domain.ProviderErrInvalidRequest {
				t.Fatalf("expected invalid_request, got %v", err)
			}
		})
	}
}

func TestOmniVideoUncertainSubmitNeverReplays(t *testing.T) {
	for _, ext := range []bool{false, true} {
		t.Run(fmt.Sprint(ext), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(502) }))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
			for i := 0; i < 2; i++ {
				_, err := p.Submit(context.Background(), omniRequest(ext))
				var pe *Error
				if !errors.As(err, &pe) || pe.Class != domain.ProviderErrSubmitIndeterminate {
					t.Fatalf("expected uncertain submit, got %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("paid submits=%d", calls)
			}
		})
	}
}
