package poyo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestSubmitKlingO3SuccessAndIdempotency(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Fatalf("idempotency header = %q", got)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelKlingO3Standard {
			t.Fatalf("model = %q", body.Model)
		}
		if body.Input["prompt"] != "safe prompt" {
			t.Fatalf("bad input prompt: %+v", body.Input)
		}
		if refs, ok := body.Input["image_urls"].([]any); !ok || len(refs) != 1 || refs[0] != "https://cdn.test/input.png" {
			t.Fatalf("image_urls = %#v", body.Input["image_urls"])
		}
		if body.Input["duration"].(float64) != 10 || body.Input["aspect_ratio"] != "16:9" {
			t.Fatalf("bad input options: %+v", body.Input)
		}
		if body.Input["sound"] != false {
			t.Fatalf("kling sound must be explicitly disabled, got %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"poyo_task_1","status":"not_started","created_time":"2026-06-19T15:00:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest(ModelKlingO3Standard)
	req.IdempotencyKey = "idem-1"
	req.DurationSec = 10
	req.Resolution = "1080p"
	req.AspectRatio = "16:9"
	req.InputURLs = []string{"https://cdn.test/input.png"}

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ExternalID != "poyo_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
	task2, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if task2.ExternalID != task.ExternalID || calls != 1 {
		t.Fatalf("idempotency failed task2=%+v calls=%d", task2, calls)
	}
}

func TestSubmitRejectsKlingAudioByDefault(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest(ModelKlingO3Standard)
	req.Params = rawJSON(t, map[string]any{"audio": true})

	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
	if called {
		t.Fatal("audio request must be rejected before HTTP submit")
	}
}

func TestSeedanceEstimateAndReferenceValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelSeedance20Fast)
	req.DurationSec = 10
	req.Resolution = "720p"

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 560 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}

	req.InputURLs = []string{"1", "2", "3", "4", "5"}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.InputURLs = nil
	req.Resolution = "1080p"
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
}

func TestRunwayGen45DurationAndReferenceValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelRunwayGen45)
	req.DurationSec = 7

	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.DurationSec = 5
	req.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.InputURLs = nil
	_, err = provider.Estimate(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrUnsupportedCapab)
}

func TestEstimateUsesResolvedRouteSnapshot(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelRunwayGen45)
	req.Params = rawJSON(t, map[string]any{
		"resolved_video_route": domain.VideoRouteSnapshot{
			Alias:                  domain.VideoRouteRunwayGen45,
			Provider:               domain.ProviderPoYo,
			ProviderModelID:        ModelRunwayGen45,
			ModelClass:             "runway_gen4_5",
			DurationSec:            5,
			Resolution:             "720p",
			ProviderCostCredits:    25,
			InternalCostCredits:    50,
			PriceMultiplier:        2,
			MaxProviderCostCredits: 25,
			MaxInternalCostCredits: 50,
		},
	})

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 50 || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}
}

func TestPollCompletedReturnsOutputAndSanitizesRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/generate/status/task_1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"task_1","status":"finished","files":[{"file_type":"video","file_url":"https://private.poyo.ai/output.mp4?token=secret","format":"mp4"}],"credits_amount":50,"created_time":"2026-06-19T15:00:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "task_1"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
		t.Fatalf("bad result: %+v", result)
	}
	raw := string(result.Raw)
	if strings.Contains(raw, "private.poyo.ai") || strings.Contains(raw, "secret") || strings.Contains(raw, "video_url") {
		t.Fatalf("raw metadata leaked private output URL: %s", raw)
	}
}

func TestPollFailureNormalizesModeration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"task_1","status":"failed","error_message":"policy rejected prompt"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "task_1"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskFailed || result.ErrorClass != domain.ProviderErrContentRejected {
		t.Fatalf("bad result: %+v", result)
	}
	if strings.Contains(string(result.Raw), "policy rejected prompt") {
		t.Fatalf("raw metadata leaked provider error text: %s", string(result.Raw))
	}
}

func TestSubmitHTTPErrorIsNormalized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"unauthorized token"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "bad-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := provider.Submit(context.Background(), baseVideoRequest(ModelKlingO3Standard))
	requireErrorClass(t, err, domain.ProviderErrAuthFailed)
	if err != nil && strings.Contains(err.Error(), "bad-key") {
		t.Fatalf("error leaked api key: %v", err)
	}
}

func baseVideoRequest(model string) domain.ProviderRequest {
	return domain.ProviderRequest{
		JobID:          uuid.New(),
		UserID:         uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      model,
		Provider:       domain.ProviderPoYo,
		Prompt:         "safe prompt",
		DurationSec:    5,
		Resolution:     "720p",
		AspectRatio:    "16:9",
		IdempotencyKey: "idem-" + uuid.NewString(),
	}
}

func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func requireErrorClass(t *testing.T, err error, class domain.ProviderErrorClass) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", class)
	}
	var classified interface {
		ProviderErrorClass() domain.ProviderErrorClass
	}
	if !errors.As(err, &classified) {
		t.Fatalf("error has no provider class: %v", err)
	}
	if got := classified.ProviderErrorClass(); got != class {
		t.Fatalf("error class = %s, want %s; err=%v", got, class, err)
	}
}
