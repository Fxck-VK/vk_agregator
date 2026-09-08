package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestModel_grok_image_1_5(t *testing.T) {
	testGrokImageContract(t, "1_5", "grok-imagine-1.5-apimart")
}
func TestModel_grok_image_2_0(t *testing.T) { testGrokImageContract(t, "2_0", "grok-imagine-2.0-ext") }

func TestGrok20SafeSubmitReplay(t *testing.T) {
	for _, scenario := range []string{"in progress", "indeterminate", "key reused", "persistent conflict", "server failure", "connection lost", "retry after exceeds deadline"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			var firstBody []byte
			started := time.Now()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				body, _ := io.ReadAll(r.Body)
				if n == 1 {
					firstBody = body
				} else if string(firstBody) != string(body) {
					t.Error("retry changed body")
				}
				if r.Header.Get("Idempotency-Key") != "stable-intent" || r.Header.Get("X-APIMart-Response-Version") != "2026-07-27" {
					t.Error("retry changed key or version")
				}
				if n > 1 && (scenario == "in progress" || scenario == "server failure" || scenario == "connection lost") {
					if scenario == "in progress" && time.Since(started) < time.Second {
						t.Error("Retry-After ignored")
					}
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"code":202,"data":{"id":"recovered-task"}}`))
					return
				}
				w.Header().Set("Retry-After", "0")
				code := "idempotency_in_progress"
				switch scenario {
				case "in progress":
					w.Header().Set("Retry-After", "1")
				case "retry after exceeds deadline":
					w.Header().Set("Retry-After", "60")
				case "indeterminate":
					code = "idempotency_result_indeterminate"
				case "key reused":
					code = "idempotency_key_reused"
				case "server failure":
					w.WriteHeader(http.StatusBadGateway)
					return
				case "connection lost":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "synthetic-private-message"}})
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			ctx := context.Background()
			if scenario == "retry after exceeds deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			task, err := p.Submit(ctx, domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: ModelGrokImage20, Prompt: "Synthetic test image", IdempotencyKey: "stable-intent"})
			if scenario == "in progress" || scenario == "server failure" || scenario == "connection lost" {
				if err != nil || task.ExternalID != "recovered-task" || calls.Load() != 2 {
					t.Fatal("same-key replay did not recover task")
				}
				return
			}
			wantClass, wantCalls := domain.ProviderErrSubmitIndeterminate, int32(1)
			if scenario == "key reused" {
				wantClass = domain.ProviderErrInvalidRequest
			}
			if scenario == "persistent conflict" {
				wantCalls = 3
			}
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != wantClass || calls.Load() != wantCalls || strings.Contains(err.Error(), "synthetic-private-message") {
				t.Fatal("unsafe retry or error")
			}
		})
	}
}

func testGrokImageContract(t *testing.T, version, model string) {
	t.Helper()
	request := func() domain.ProviderRequest {
		return domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage,
			ModelCode: model, Prompt: "Synthetic test image", Size: "1:1", OutputCount: 1, IdempotencyKey: "grok-test"}
	}
	submitResponse := `{"code":200,"data":[{"status":"submitted","task_id":"grok-task"}]}`
	status := http.StatusOK
	if version == "2_0" {
		submitResponse = `{"code":202,"data":{"id":"grok-task","status":"queued"}}`
		status = http.StatusAccepted
	}
	t.Run("exact request and asynchronous result", func(t *testing.T) {
		fixture, err := os.ReadFile("testdata/contracts/grok_image_" + version + ".request.json")
		if err != nil {
			t.Fatal(err)
		}
		var want map[string]any
		if err := json.Unmarshal(fixture, &want); err != nil {
			t.Fatal(err)
		}
		var submits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/grok-task" {
				_, _ = w.Write([]byte(`{"code":200,"data":{"id":"grok-task","status":"completed","cost":0.015,"credits_cost":0.15,"result":{"images":[{"url":["https://example.com/a.png","https://example.com/b.png"]},{"url":"https://example.com/c.png"}]}}}`))
				return
			}
			submits.Add(1)
			if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" || r.Header.Get("Idempotency-Key") != "grok-test" {
				t.Error("incorrect endpoint or idempotency header")
			}
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("wrong request fields: %v", got)
			}
			wantVersion := ""
			if version == "2_0" {
				wantVersion = "2026-07-27"
			}
			if r.Header.Get("X-APIMart-Response-Version") != wantVersion {
				t.Error("wrong response version header")
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(submitResponse))
		}))
		defer srv.Close()
		p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
		for i := 0; i < 2; i++ {
			task, err := p.Submit(context.Background(), request())
			if err != nil {
				t.Fatal(err)
			}
			if task.ExternalID != "grok-task" || task.ModelCode != model {
				t.Fatal("incorrect task identity")
			}
		}
		if submits.Load() != 1 {
			t.Fatal("duplicate submit")
		}
		// A new adapter instance must send the same provider idempotency key.
		p = New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
		if _, err := p.Submit(context.Background(), request()); err != nil {
			t.Fatal(err)
		}
		result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "grok-task"})
		if err != nil || result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 3 {
			t.Fatalf("poll status/outputs: %v", err)
		}
		if strings.Contains(string(result.Raw), "example.com") {
			t.Fatal("private media URL in metadata")
		}
		if estimate, err := p.Estimate(context.Background(), request()); err != nil || estimate.AmountCredits != 1 {
			t.Fatalf("estimate unavailable: %v", err)
		}
	})
	t.Run("supported options", func(t *testing.T) {
		sizes := []string{"1:1", "16:9", "9:16", "3:2", "2:3"}
		if version == "2_0" {
			sizes = append(sizes, "3:4", "4:3", "1024x1024", "1024x1792", "1792x1024", "720x1280", "1280x720")
		}
		p := New(Config{})
		for _, size := range sizes {
			req := request()
			req.Size = size
			if version == "2_0" {
				req.Resolution = "quality"
			} else {
				req.InputURLs = []string{"https://example.com/reference.png"}
			}
			if _, err := p.Estimate(context.Background(), req); err != nil {
				t.Errorf("size %s: %v", size, err)
			}
		}
	})
	t.Run("provider rejection has no output or private error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":200,"data":{"status":"failed","error":{"code":"content_rejected","message":"synthetic-private-message"}}}`))
		}))
		defer srv.Close()
		p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
		result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "grok-task"})
		if err != nil || result.Status != domain.ProviderTaskFailed || result.ErrorClass != domain.ProviderErrContentRejected || len(result.OutputURLs) != 0 || strings.Contains(result.ErrorMessage, "synthetic-private-message") {
			t.Fatalf("provider rejection: status=%s class=%s outputs=%d err=%v", result.Status, result.ErrorClass, len(result.OutputURLs), err)
		}
	})
	t.Run("invalid options never submit", func(t *testing.T) {
		cases := map[string]func(*domain.ProviderRequest){
			"batch":                  func(r *domain.ProviderRequest) { r.OutputCount = 2 },
			"negative count":         func(r *domain.ProviderRequest) { r.OutputCount = -1 },
			"unsupported resolution": func(r *domain.ProviderRequest) { r.Resolution = "2K" },
			"size as resolution":     func(r *domain.ProviderRequest) { r.Size = "1K" },
			"unsupported aspect":     func(r *domain.ProviderRequest) { r.AspectRatio = "21:9" },
			"unsupported size":       func(r *domain.ProviderRequest) { r.Size = "4:5" },
			"negative prompt":        func(r *domain.ProviderRequest) { r.NegativePrompt = "synthetic exclusion" },
			"raw batch":              func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"n":2}`) },
			"explicit zero":          func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"n":0}`) },
			"raw quality":            func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"quality":"high"}`) },
			"style":                  func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"style":"vivid"}`) },
			"stream":                 func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"stream":true}`) },
			"base64 result":          func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"response_format":"b64_json"}`) },
			"malformed params":       func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) },
			"empty prompt":           func(r *domain.ProviderRequest) { r.Prompt = " " },
			"too many refs": func(r *domain.ProviderRequest) {
				r.InputURLs = []string{"https://example.com/a.png", "https://example.com/b.png"}
			},
		}
		if version == "2_0" {
			cases["reference"] = func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://example.com/a.png"} }
			cases["raw reference"] = func(r *domain.ProviderRequest) {
				r.Params = json.RawMessage(`{"image_urls":["https://example.com/a.png"]}`)
			}
			cases["long idempotency key"] = func(r *domain.ProviderRequest) { r.IdempotencyKey = strings.Repeat("x", 192) }
		}
		for name, change := range cases {
			t.Run(name, func(t *testing.T) {
				var calls atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(500) }))
				defer srv.Close()
				p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
				req := request()
				change(&req)
				_, err := p.Submit(context.Background(), req)
				var providerErr *Error
				if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest || calls.Load() != 0 {
					t.Fatalf("local rejection: %v, HTTP calls %d", err, calls.Load())
				}
			})
		}
	})
	t.Run("malformed submit never accepted", func(t *testing.T) {
		for _, body := range []string{`{`, `{"code":200,"data":null}`, `{"code":202,"data":{"id":""}}`, `{"code":400,"error":{"code":"invalid_request","message":"synthetic-private-message"}}`} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			_, err := p.Submit(context.Background(), request())
			srv.Close()
			if err == nil || strings.Contains(err.Error(), "synthetic-private-message") {
				t.Fatal("malformed response accepted or private error leaked")
			}
		}
	})
}
