package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func midjourneyRequest() domain.ProviderRequest {
	return domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: "midjourney", Prompt: "Synthetic landscape", Resolution: "relax", AspectRatio: "16:9", OutputCount: 1, IdempotencyKey: "mj-test"}
}

func TestMidjourneyImagineContract(t *testing.T) {
	for _, speed := range []string{"relax", "fast", "turbo"} {
		t.Run(speed, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/v1/midjourney/generations" || r.Header.Get("Idempotency-Key") == "" {
					t.Error("incorrect Imagine endpoint or headers")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["version"] != "7" || body["speed"] != speed || body["size"] != "16:9" || body["quality"] != "1" {
					t.Error("wrong fixed version, speed, aspect or quality")
				}
				for _, key := range []string{"model", "n", "resolution", "repeat", "extra", "niji"} {
					if _, ok := body[key]; ok {
						t.Errorf("unexpected native field %s", key)
					}
				}
				if refs, ok := body["image_urls"].([]any); !ok || len(refs) != 1 {
					t.Error("reference lost")
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"mj-task","status":"submitted"}]}`))
			}))
			defer srv.Close()
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			req := midjourneyRequest()
			req.Resolution = speed
			req.InputURLs = []string{"https://example.com/reference.png"}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					task, err := p.Submit(context.Background(), req)
					if err != nil || task.ExternalID != "mj-task" {
						t.Error("Imagine submission failed")
					}
				}()
			}
			wg.Wait()
			if calls.Load() != 1 {
				t.Fatal("concurrent replay created multiple paid tasks")
			}
		})
	}
}

func TestMidjourneyRejectsUnpricedOptions(t *testing.T) {
	for _, change := range []func(*domain.ProviderRequest){
		func(r *domain.ProviderRequest) { r.Prompt += " --repeat 5" },
		func(r *domain.ProviderRequest) { r.Prompt += " --v 8.2" },
		func(r *domain.ProviderRequest) { r.Prompt = "a {red,blue} bird" },
		func(r *domain.ProviderRequest) { r.Resolution = "1K" },
		func(r *domain.ProviderRequest) { r.AspectRatio = "auto" },
		func(r *domain.ProviderRequest) { r.AspectRatio = "invalid" },
		func(r *domain.ProviderRequest) { r.Size = "invalid" },
		func(r *domain.ProviderRequest) { r.OutputCount = 4 },
		func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"repeat":5}`) },
		func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"extra":"--turbo"}`) },
		func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"version":"8.2"}`) },
		func(r *domain.ProviderRequest) { r.InputURLs = []string{"http://localhost/input.png"} },
	} {
		req := midjourneyRequest()
		change(&req)
		p := New(Config{BaseURL: "http://127.0.0.1:1/v1"})
		_, err := p.Submit(context.Background(), req)
		var providerErr *Error
		if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest {
			t.Fatalf("expected local rejection, got %v", err)
		}
	}
}

func TestMidjourneyAmbiguousSubmitNeverReplays(t *testing.T) {
	testAmbiguousImageSubmitNeverReplays(t, midjourneyRequest())
}

func TestFlux2AmbiguousSubmitNeverReplays(t *testing.T) {
	testAmbiguousImageSubmitNeverReplays(t, flux2Request())
}

func testAmbiguousImageSubmitNeverReplays(t *testing.T, req domain.ProviderRequest) {
	t.Helper()
	for _, response := range []string{"server", "malformed", "missing task", "disconnect"} {
		t.Run(response, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch response {
				case "disconnect":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
				case "server":
					w.WriteHeader(502)
				case "malformed":
					_, _ = w.Write([]byte(`{`))
				case "missing task":
					_, _ = w.Write([]byte(`{"code":200,"data":[]}`))
				}
			}))
			defer srv.Close()
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			for i := 0; i < 2; i++ {
				_, err := p.Submit(context.Background(), req)
				var pe *Error
				if !errors.As(err, &pe) || pe.Class != domain.ProviderErrSubmitIndeterminate {
					t.Fatalf("unsafe error classification: %v", err)
				}
			}
			if calls.Load() != 1 {
				t.Fatal("ambiguous paid request replayed")
			}
		})
	}
}
