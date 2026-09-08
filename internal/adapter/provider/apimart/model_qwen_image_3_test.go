package apimart

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func qwenImageRequest() domain.ProviderRequest {
	return domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage,
		ModelCode: "qwen-image-3.0", Prompt: "Synthetic test image", Size: "1:1", Resolution: "1K", IdempotencyKey: "qwen-test"}
}

func TestModel_qwen_image_3(t *testing.T) {
	t.Run("contract and replay", func(t *testing.T) {
		fixture, err := os.ReadFile("testdata/contracts/qwen_image_3.request.json")
		if err != nil {
			t.Fatal(err)
		}
		var want map[string]any
		if err := json.Unmarshal(fixture, &want); err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" || r.Header.Get("Idempotency-Key") != "qwen-test" {
				t.Error("unexpected submit route or idempotency header")
			}
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("contract mismatch: %v", got)
			}
			_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"qwen-task"}]}`))
		}))
		defer srv.Close()
		p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
		for i := 0; i < 2; i++ {
			task, err := p.Submit(context.Background(), qwenImageRequest())
			if err != nil {
				t.Fatal(err)
			}
			if task.ExternalID != "qwen-task" || task.ModelCode != "qwen-image-3.0" {
				t.Fatal("wrong task mapping")
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("submit count = %d", calls.Load())
		}
		caps, err := p.Capabilities(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, cap := range caps {
			if cap.ModelCode == "qwen-image-3.0" && cap.SupportsPolling {
				found = true
			}
		}
		if !found {
			t.Fatal("missing Qwen capability")
		}
	})
	t.Run("2K references stay on base model", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			if got["model"] != "qwen-image-3.0" || got["resolution"] != "2K" || got["size"] != "16:9" || len(got["image_urls"].([]any)) != 3 || got["negative_prompt"] != "synthetic exclusion" {
				t.Errorf("wrong 2K reference contract: %v", got)
			}
			_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"qwen-edit"}]}`))
		}))
		defer srv.Close()
		p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
		req := qwenImageRequest()
		req.Resolution, req.Size, req.NegativePrompt = "2k", "16:9", "synthetic exclusion"
		req.InputURLs = []string{"https://example.com/a.png", "https://example.com/b.png", "https://example.com/c.png"}
		if _, err := p.Estimate(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Submit(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid inputs never submit", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
		defer srv.Close()
		cases := map[string]func(*domain.ProviderRequest){
			"typed batch":           func(r *domain.ProviderRequest) { r.OutputCount = 2 },
			"negative output count": func(r *domain.ProviderRequest) { r.OutputCount = -1 },
			"four references": func(r *domain.ProviderRequest) {
				r.InputURLs = []string{"https://example.com/a.png", "https://example.com/b.png", "https://example.com/c.png", "https://example.com/d.png"}
			},
			"4K":                 func(r *domain.ProviderRequest) { r.Resolution = "4K" },
			"4K in size":         func(r *domain.ProviderRequest) { r.Size = "4K" },
			"small pixels":       func(r *domain.ProviderRequest) { r.Size = "511x1024" },
			"large pixels":       func(r *domain.ProviderRequest) { r.Size = "2049x1024" },
			"unsupported aspect": func(r *domain.ProviderRequest) { r.AspectRatio = "21:9" },
			"auto":               func(r *domain.ProviderRequest) { r.Size = "auto" },
			"pro":                func(r *domain.ProviderRequest) { r.ModelCode = "qwen-image-3.0-pro" },
			"empty prompt":       func(r *domain.ProviderRequest) { r.Prompt = " " },
			"agent reference": func(r *domain.ProviderRequest) {
				r.InputURLs = []string{"https://example.com/a.png"}
				r.Params = json.RawMessage(`{"prompt_extend":true,"prompt_extend_mode":"agent"}`)
			},
			"batch":            func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"n":2}`) },
			"malformed params": func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) },
			"over 10 MiB": func(r *domain.ProviderRequest) {
				r.InputURLs = []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, 10*1024*1024+1))}
			},
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				req := qwenImageRequest()
				mutate(&req)
				p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
				_, err := p.Submit(context.Background(), req)
				if err == nil {
					t.Fatal("expected rejection")
				}
				var perr *Error
				if !errors.As(err, &perr) {
					t.Fatalf("unexpected error: %v", err)
				}
				class := perr.Class
				if class != domain.ProviderErrInvalidRequest && class != domain.ProviderErrUnsupportedCapab {
					t.Fatalf("unexpected error class: %s", class)
				}
			})
		}
		if calls.Load() != 0 {
			t.Fatalf("invalid requests caused %d calls", calls.Load())
		}
	})
	t.Run("supported sizes", func(t *testing.T) {
		p := New(Config{})
		for _, size := range []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "16x9", "512x512", "2048x2048", "1600x900"} {
			req := qwenImageRequest()
			req.Size = size
			if _, err := p.Estimate(context.Background(), req); err != nil {
				t.Errorf("%s: %v", size, err)
			}
		}
	})
	t.Run("pixel size selects explicit billing tier", func(t *testing.T) {
		for _, tc := range []struct{ size, resolution string }{{"1500x1500", "1K"}, {"1501x1500", "2K"}, {"2048x2048", "2K"}} {
			t.Run(tc.size, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["size"] != tc.size || body["resolution"] != tc.resolution {
						t.Error("wrong pixel size or billing tier")
					}
					_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"qwen-pixels"}]}`))
				}))
				defer srv.Close()
				req := qwenImageRequest()
				req.Size, req.Resolution = tc.size, ""
				p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
				if _, err := p.Submit(context.Background(), req); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
	t.Run("poll outputs and safe failures", func(t *testing.T) {
		for _, tc := range []struct {
			name, body string
			want       domain.ProviderTaskStatus
			outputs    int
			wantErr    bool
		}{
			{"completed", `{"code":200,"data":{"status":"completed","result":{"images":[{"url":["https://example.com/a.png","https://example.com/b.png"]},{"url":"https://example.com/c.png"}]}}}`, domain.ProviderTaskSucceeded, 3, false},
			{"failed", `{"code":200,"data":{"status":"failed","error":{"message":"private upstream detail"}}}`, domain.ProviderTaskFailed, 0, false},
			{"malformed", `{`, "", 0, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet || r.URL.Path != "/tasks/qwen-task" {
						t.Error("unexpected polling route")
					}
					_, _ = w.Write([]byte(tc.body))
				}))
				defer srv.Close()
				p := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
				result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "qwen-task"})
				if (err != nil) != tc.wantErr {
					t.Fatalf("poll error: %v", err)
				}
				if err == nil && (result.Status != tc.want || len(result.OutputURLs) != tc.outputs || strings.Contains(result.ErrorMessage, "private upstream")) {
					t.Fatal("incorrect poll result")
				}
			})
		}
	})
}
