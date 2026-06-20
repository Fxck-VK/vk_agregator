package runway

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

func TestRunwayRatioMapsSupportedAspectRatios(t *testing.T) {
	tests := map[string]string{
		"":     "1280:720",
		"16:9": "1280:720",
		"9:16": "720:1280",
		"4:3":  "1104:832",
		"3:4":  "832:1104",
		"1:1":  "960:960",
		"21:9": "1584:672",
	}
	for aspect, want := range tests {
		if got := runwayRatio(aspect); got != want {
			t.Fatalf("runwayRatio(%q) = %q, want %q", aspect, got, want)
		}
	}
}

func TestSubmitGen4TurboSuccessAndIdempotency(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/image_to_video" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("X-Runway-Version"); got != DefaultAPIVersion {
			t.Fatalf("runway version = %q", got)
		}
		var body imageToVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelGen4Turbo || body.PromptImage != "data:image/png;base64,aW1hZ2U=" || body.PromptText != "safe motion" {
			t.Fatalf("bad body: %+v", body)
		}
		if body.Ratio != "720:1280" || body.Duration != 7 {
			t.Fatalf("bad ratio/duration: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task_runway","status":"PENDING","createdAt":"2026-06-15T10:00:00.000Z"}`))
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest()
	req.IdempotencyKey = "idem-1"
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	req.DurationSec = 7
	req.AspectRatio = "9:16"

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderRunway || task.ExternalID != "task_runway" || task.Status != domain.ProviderTaskPending {
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

func TestSubmitRejectsInvalidShapeBeforeHTTP(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest()
	req.InputURLs = nil
	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req = baseVideoRequest()
	req.DurationSec = 11
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req = baseVideoRequest()
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	req.Resolution = "1080p"
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req = baseVideoRequest()
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	req.Params = rawJSON(t, map[string]any{"ratio": "1920:1080"})
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	if called {
		t.Fatal("invalid request must be rejected before HTTP submit")
	}
}

func TestEstimateUsesRateAndRouteSnapshot(t *testing.T) {
	provider := New(Config{APISecret: "test-secret", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest()
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	req.DurationSec = 7

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 70 || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}

	req.Params = rawJSON(t, map[string]any{
		"resolved_video_route": domain.VideoRouteSnapshot{
			Alias:               domain.VideoRouteRunwayGen4Turbo,
			Provider:            domain.ProviderRunway,
			ProviderModelID:     ModelGen4Turbo,
			ModelClass:          "runway_gen4_turbo",
			DurationSec:         7,
			Resolution:          "720p",
			ProviderCostCredits: 35,
			InternalCostCredits: 70,
			PriceMultiplier:     2,
		},
	})
	estimate, err = provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("snapshot estimate: %v", err)
	}
	if estimate.AmountCredits != 70 || estimate.Estimated {
		t.Fatalf("bad snapshot estimate: %+v", estimate)
	}
}

func TestPollSucceededReturnsOutputAndSanitizesRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks/task_runway" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task_runway","status":"SUCCEEDED","createdAt":"2026-06-15T10:00:00.000Z","output":["https://assets.runwayml.com/output.mp4?token=secret"]}`))
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderRunway, ExternalID: "task_runway"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
		t.Fatalf("bad result: %+v", result)
	}
	raw := string(result.Raw)
	if strings.Contains(raw, "assets.runwayml.com") || strings.Contains(raw, "secret") || strings.Contains(raw, "output") {
		t.Fatalf("raw metadata leaked private output URL: %s", raw)
	}
}

func TestPollFailedNormalizesSafetyFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task_runway","status":"FAILED","createdAt":"2026-06-15T10:00:00.000Z","failure":"The provided image was flagged by content moderation.","failureCode":"SAFETY.INPUT.IMAGE"}`))
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderRunway, ExternalID: "task_runway"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskFailed || result.ErrorClass != domain.ProviderErrContentRejected {
		t.Fatalf("bad result: %+v", result)
	}
	if strings.Contains(string(result.Raw), "flagged by content moderation") {
		t.Fatalf("raw metadata leaked provider failure text: %s", string(result.Raw))
	}
}

func TestCancelUsesTaskEndpointAndIgnores404(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch len(paths) {
		case 1:
			w.WriteHeader(http.StatusNoContent)
		case 2:
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected call %d", len(paths))
		}
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "test-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	ref := domain.ProviderTaskRef{Provider: domain.ProviderRunway, ExternalID: "task_runway"}
	if err := provider.Cancel(context.Background(), ref); err != nil {
		t.Fatalf("cancel 204: %v", err)
	}
	if err := provider.Cancel(context.Background(), ref); err != nil {
		t.Fatalf("cancel 404: %v", err)
	}
	if strings.Join(paths, ",") != "DELETE /tasks/task_runway,DELETE /tasks/task_runway" {
		t.Fatalf("unexpected cancel paths: %v", paths)
	}
}

func TestSubmitHTTPErrorIsNormalized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	provider := New(Config{APISecret: "bad-secret", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest()
	req.InputURLs = []string{"data:image/png;base64,aW1hZ2U="}
	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrAuthFailed)
	if err != nil && strings.Contains(err.Error(), "bad-secret") {
		t.Fatalf("error leaked api secret: %v", err)
	}
}

func baseVideoRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		JobID:          uuid.New(),
		UserID:         uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelGen4Turbo,
		Provider:       domain.ProviderRunway,
		Prompt:         "safe motion",
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
