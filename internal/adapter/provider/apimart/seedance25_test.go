package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func seedance25Request() domain.ProviderRequest {
	return domain.ProviderRequest{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo,
		ModelCode: "seedance-2.5", Prompt: "A blue paper boat on calm water", DurationSec: 5,
		Resolution: "480p", AspectRatio: "16:9", IdempotencyKey: "seedance-test:1"}
}

func TestSeedance25SubmitAndPoll(t *testing.T) {
	for _, references := range [][]string{nil, {"https://example.com/reference.png"}} {
		t.Run(strings.Join(references, ""), func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && r.URL.Path == "/tasks/seedance-task" {
					_, _ = w.Write([]byte(`{"code":200,"data":{"status":"completed","result":{"videos":[{"url":"https://example.com/video.mp4"}]}}}`))
					return
				}
				posts++
				if r.URL.Path != "/videos/generations" || r.Method != http.MethodPost || r.Header.Get("Idempotency-Key") != "seedance-test:1" {
					t.Error("incorrect submit endpoint, method or idempotency header")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				for key, want := range map[string]any{"model": "seedance-2.5", "duration": float64(5), "resolution": "480p", "size": "16:9", "generate_audio": true, "output_format": "mp4", "watermark": false, "NSFWCheck": true} {
					if body[key] != want {
						t.Errorf("%s = %v, want %v", key, body[key], want)
					}
				}
				if _, exists := body["firstFrameImage"]; exists {
					t.Error("Seedance must use image_urls for reference images")
				}
				if len(references) > 0 {
					images, ok := body["image_urls"].([]any)
					if !ok || len(images) != 1 || images[0] != references[0] {
						t.Error("reference images missing")
					}
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"seedance-task","status":"submitted"}]}`))
			}))
			defer srv.Close()
			p := New(Config{APIKey: "test", BaseURL: srv.URL, HTTPClient: srv.Client()})
			req := seedance25Request()
			req.InputURLs = references
			for i := 0; i < 2; i++ {
				task, err := p.Submit(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if task.ExternalID != "seedance-task" {
					t.Fatal("missing persisted polling identity")
				}
			}
			if posts != 1 {
				t.Fatalf("submit requests = %d, want 1", posts)
			}
			result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "seedance-task"})
			if err != nil || result.Status != domain.ProviderTaskSucceeded {
				t.Fatalf("poll failed: %v", err)
			}
		})
	}
}

func TestSeedance25UncertainSubmissionIsNotRepeated(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"gateway", 502, `{}`}, {"conflict", 409, `{}`}, {"timeout", 408, `{}`},
		{"malformed", 200, `{`}, {"missing task", 200, `{"code":200,"data":[]}`},
		{"envelope failure", 200, `{"code":500,"message":"unavailable"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
			for i := 0; i < 2; i++ {
				_, err := p.Submit(context.Background(), seedance25Request())
				var providerErr *Error
				if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrSubmitIndeterminate {
					t.Fatalf("error = %v, want submit_indeterminate", err)
				}
			}
			if posts != 1 {
				t.Fatalf("paid request repeated %d times", posts)
			}
		})
	}
}

func TestSeedance25RejectsUnpricedOptionsBeforeHTTP(t *testing.T) {
	for name, edit := range map[string]func(*domain.ProviderRequest){
		"auto duration":     func(r *domain.ProviderRequest) { r.DurationSec = -1 },
		"unpriced duration": func(r *domain.ProviderRequest) { r.DurationSec = 7 },
		"resolution":        func(r *domain.ProviderRequest) { r.Resolution = "4k" },
		"aspect":            func(r *domain.ProviderRequest) { r.AspectRatio = "adaptive" },
		"blank prompt":      func(r *domain.ProviderRequest) { r.Prompt = " " },
		"reference limit":   func(r *domain.ProviderRequest) { r.InputURLs = make([]string, 31) },
		"private URL":       func(r *domain.ProviderRequest) { r.InputURLs = []string{"http://127.0.0.1/image.png"} },
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid request reached provider") }))
			defer srv.Close()
			req := seedance25Request()
			edit(&req)
			_, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).Submit(context.Background(), req)
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSeedance25ConcurrentReplayCreatesOneTask(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"one-task","status":"submitted"}]}`))
	}))
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	var callers sync.WaitGroup
	for i := 0; i < 8; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			task, err := p.Submit(context.Background(), seedance25Request())
			if err != nil || task.ExternalID != "one-task" {
				t.Error("concurrent caller lost accepted task")
			}
		}()
	}
	callers.Wait()
	if posts.Load() != 1 {
		t.Fatalf("upstream submissions = %d, want 1", posts.Load())
	}
}

type seedanceDisconnectedTransport struct{ calls int }

func (r *seedanceDisconnectedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, context.DeadlineExceeded
}

func TestSeedance25TransportTimeoutIsNotResubmitted(t *testing.T) {
	transport := &seedanceDisconnectedTransport{}
	p := New(Config{HTTPClient: &http.Client{Transport: transport}})
	for i := 0; i < 2; i++ {
		_, err := p.Submit(context.Background(), seedance25Request())
		var providerErr *Error
		if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrSubmitIndeterminate {
			t.Fatalf("transport error = %v", err)
		}
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1", transport.calls)
	}
}
