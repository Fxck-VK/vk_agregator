package deepinfra

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	providertest "vk-ai-aggregator/internal/adapter/provider/providertest"
	"vk-ai-aggregator/internal/domain"
)

const defaultImageModel = "ByteDance/Seedream-4.5"

func TestSubmitPollTextSuccess(t *testing.T) {
	var seen chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:test:1" {
			t.Errorf("idempotency header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","choices":[{"message":{"role":"assistant","content":"DeepSeek answer"}}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, ImageModel: defaultImageModel, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:           uuid.New(),
		Operation:       domain.OperationTextGenerate,
		Modality:        domain.ModalityText,
		Prompt:          "hello",
		MaxOutputTokens: 800,
		IdempotencyKey:  "provider_submit:test:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderDeepInfra || task.ModelCode != defaultTextModel {
		t.Fatalf("unexpected task: %+v", task)
	}
	systemPrompt := seen.Messages[0].Content
	if seen.Model != defaultTextModel || len(seen.Messages) != 2 || seen.Messages[0].Role != "system" || !strings.Contains(systemPrompt, "3000") || !strings.Contains(systemPrompt, "НейроХаб") || !strings.Contains(systemPrompt, "Факты НейроХаб") || !strings.Contains(systemPrompt, "код модели") || strings.Contains(systemPrompt, "NeuroHub") || seen.Messages[1].Content != "hello" || seen.Stream {
		t.Fatalf("unexpected request body: %+v", seen)
	}

	if seen.MaxTokens != 800 {
		t.Fatalf("max_tokens = %d, want 800", seen.MaxTokens)
	}

	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Text != "DeepSeek answer" {
		t.Fatalf("result text = %q", res.Text)
	}
	const prefix = "data:text/plain; charset=utf-8;base64,"
	if !strings.HasPrefix(res.OutputURLs[0], prefix) {
		t.Fatalf("expected text data url, got %q", res.OutputURLs[0])
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(res.OutputURLs[0], prefix))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if string(decoded) != "DeepSeek answer" {
		t.Fatalf("output = %q", decoded)
	}
}

func TestSubmitTextIdempotencyContract(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:text:same" {
			t.Fatalf("idempotency header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_` + uuid.NewString() + `","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Prompt:         "hello",
		IdempotencyKey: "provider_submit:text:same",
	}
	first, err := p.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := p.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if calls != 1 {
		t.Fatalf("idempotent submit called provider %d times, want 1", calls)
	}
	if first.ExternalID != second.ExternalID {
		t.Fatalf("idempotent external id mismatch: %q vs %q", first.ExternalID, second.ExternalID)
	}
}

func TestSubmitHTTPErrorDoesNotLeakSecretFixture(t *testing.T) {
	const fakeSecret = "deepinfra-secret-fixture"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid token ` + fakeSecret + `"}}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: fakeSecret, BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationTextGenerate,
		Modality:  domain.ModalityText,
		Prompt:    "hello",
	})
	providertest.RequireErrorClass(t, err, domain.ProviderErrAuthFailed)
	providertest.RequireErrorDoesNotContain(t, err, fakeSecret)
}

func TestSubmitUsesExplicitModelCode(t *testing.T) {
	var seen chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, TextModel: "default-model", HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationTextGenerate,
		Modality:  domain.ModalityText,
		ModelCode: "custom-model",
		Prompt:    "hello",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if seen.Model != "custom-model" || task.ModelCode != "custom-model" {
		t.Fatalf("model mismatch, request=%q task=%q", seen.Model, task.ModelCode)
	}
}

func TestCapabilitiesAreTextOnlyByDefault(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	var imageFound, textFound bool
	for _, cap := range caps {
		if cap.Operation == domain.OperationImageGenerate &&
			cap.Modality == domain.ModalityImage &&
			cap.SupportsPolling {
			imageFound = true
		}
		if cap.Operation == domain.OperationTextGenerate &&
			cap.Modality == domain.ModalityText &&
			cap.ModelCode == defaultTextModel &&
			cap.SupportsPolling {
			textFound = true
		}
	}
	if !textFound || imageFound {
		t.Fatalf("default capabilities should be text-only, got %+v", caps)
	}
}

func TestSubmitPollImageSuccess(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	encoded := base64.StdEncoding.EncodeToString(png)
	var seen nativeImageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); !strings.Contains(got, "/v1/inference/ByteDance%2FSeedream-4.5") {
			t.Errorf("path = %q, want DeepInfra native Seedream endpoint", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:image:1" {
			t.Errorf("idempotency header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["data:image/png;base64,` + encoded + `"],"nsfw_content_detected":[false],"seed":42}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, ImageModel: defaultImageModel, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Prompt:         "a cat",
		Size:           "1024x1024",
		IdempotencyKey: "provider_submit:image:1",
	})
	if err != nil {
		t.Fatalf("submit image: %v", err)
	}
	if seen.Prompt != "a cat" || seen.Size != "1024x1024" {
		t.Fatalf("unexpected image request: %+v", seen)
	}
	if task.Provider != domain.ProviderDeepInfra || task.ModelCode != defaultImageModel {
		t.Fatalf("unexpected task: %+v", task)
	}

	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll image: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(res.OutputURLs[0], prefix) {
		t.Fatalf("expected png data URL, got %q", res.OutputURLs[0])
	}
}

func TestSubmitImageUsesExplicitModelAndSize(t *testing.T) {
	var seen nativeImageRequest
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["data:image/png;base64,iVBORw0KGgo="]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, ImageModel: "default-image", ImageSize: "1024x1024", HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		ModelCode: "ByteDance/Seedream-4.5",
		Size:      "2048x2048",
		Prompt:    "a neon city",
	})
	if err != nil {
		t.Fatalf("submit image: %v", err)
	}
	if !strings.Contains(seenPath, "/v1/inference/ByteDance%2FSeedream-4.5") || seen.Size != "2048x2048" {
		t.Fatalf("model/size mismatch: path=%q request=%+v", seenPath, seen)
	}
	if task.ModelCode != "ByteDance/Seedream-4.5" {
		t.Fatalf("task model = %q", task.ModelCode)
	}
}

func TestSubmitImageFallsBackAfterRetryablePrimaryError(t *testing.T) {
	var seenPaths []string
	var seenBodies []nativeImageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.EscapedPath())
		var body nativeImageRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenBodies = append(seenBodies, body)
		if strings.Contains(r.URL.EscapedPath(), "ByteDance%2FSeedream-4.5") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["data:image/png;base64,iVBORw0KGgo="]}`))
	}))
	defer srv.Close()

	p := New(Config{
		APIKey:             "test-key",
		BaseURL:            srv.URL + "/v1/openai",
		ImageModel:         "ByteDance/Seedream-4.5",
		ImageFallbackModel: "stabilityai/sdxl-turbo",
		ImageSize:          "2K",
		HTTPClient:         srv.Client(),
	})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "a cat",
	})
	if err != nil {
		t.Fatalf("submit image: %v", err)
	}
	if task.ModelCode != "stabilityai/sdxl-turbo" {
		t.Fatalf("task model = %q, want fallback", task.ModelCode)
	}
	if len(seenPaths) != 2 {
		t.Fatalf("requests = %d, want 2: %#v", len(seenPaths), seenPaths)
	}
	if !strings.Contains(seenPaths[0], "ByteDance%2FSeedream-4.5") || !strings.Contains(seenPaths[1], "stabilityai%2Fsdxl-turbo") {
		t.Fatalf("unexpected paths: %#v", seenPaths)
	}
	if seenBodies[0].Size != "2K" || seenBodies[1].Size != "" {
		t.Fatalf("unexpected fallback sizes: %#v", seenBodies)
	}
}

func TestSubmitImageReferenceDisabled(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "edit this",
		InputURLs: []string{"https://example.com/ref.png"},
		ModelCode: defaultImageModel,
	})
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if perr.ProviderErrorClass() != domain.ProviderErrUnsupportedCapab {
		t.Fatalf("class = %q, want unsupported_capability", perr.ProviderErrorClass())
	}
}

func TestSubmitImageWithReferenceEnabled(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	inputURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	outputURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	var seen nativeImageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); !strings.Contains(got, "/v1/inference/ByteDance%2FSeedream-4.5") {
			t.Errorf("path = %q, want DeepInfra native Seedream endpoint", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["` + outputURL + `"]}`))
	}))
	defer srv.Close()

	p := New(Config{
		APIKey:                "test-key",
		BaseURL:               srv.URL,
		ImageReferenceEnabled: true,
		HTTPClient:            srv.Client(),
	})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Prompt:         "edit this",
		Size:           "2K",
		InputURLs:      []string{inputURL},
		ModelCode:      defaultImageModel,
		IdempotencyKey: "provider_submit:image-ref:1",
	})
	if err != nil {
		t.Fatalf("submit image reference: %v", err)
	}
	if seen.Prompt != "edit this" || seen.Size != "2K" {
		t.Fatalf("unexpected native request prompt/size: %+v", seen)
	}
	if len(seen.Images) != 1 || seen.Images[0] != inputURL {
		t.Fatalf("native request images = %v, want one input data URL", seen.Images)
	}
	if task.Provider != domain.ProviderDeepInfra || task.ModelCode != defaultImageModel {
		t.Fatalf("unexpected task: %+v", task)
	}
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 || res.OutputURLs[0] != outputURL {
		t.Fatalf("unexpected poll result: %+v", res)
	}
}

func TestSubmitImageWithReferenceNoSDXLFallback(t *testing.T) {
	var seenPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.EscapedPath())
		if strings.Contains(r.URL.EscapedPath(), "stabilityai%2Fsdxl-turbo") {
			t.Fatalf("fallback endpoint must not be called when references are present")
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"seedream unavailable"}}`))
	}))
	defer srv.Close()

	p := New(Config{
		APIKey:                "test-key",
		BaseURL:               srv.URL + "/v1/openai",
		ImageModel:            defaultImageModel,
		ImageFallbackModel:    "stabilityai/sdxl-turbo",
		ImageReferenceEnabled: true,
		HTTPClient:            srv.Client(),
	})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "edit this",
		InputURLs: []string{"data:image/png;base64,iVBORw0KGgo="},
		ModelCode: defaultImageModel,
	})
	if err == nil {
		t.Fatal("expected primary reference submit error")
	}
	if len(seenPaths) != 1 || !strings.Contains(seenPaths[0], "ByteDance%2FSeedream-4.5") {
		t.Fatalf("paths = %#v, want one primary Seedream request", seenPaths)
	}
}

func TestSubmitImageReferenceRequiresInputURLs(t *testing.T) {
	p := New(Config{APIKey: "test-key", ImageReferenceEnabled: true})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:                uuid.New(),
		Operation:            domain.OperationImageGenerate,
		Modality:             domain.ModalityImage,
		Prompt:               "edit this",
		ReferenceArtifactIDs: []uuid.UUID{uuid.New()},
		ModelCode:            defaultImageModel,
	})
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if perr.ProviderErrorClass() != domain.ProviderErrUnsupportedCapab || !strings.Contains(perr.Error(), "resolved input urls") {
		t.Fatalf("unexpected error: %v", perr)
	}
}

func TestSubmitImageInvalidRequestClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad image request"}}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "hello",
	})
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if perr.ProviderErrorClass() != domain.ProviderErrInvalidRequest {
		t.Fatalf("class = %q, want invalid_request", perr.ProviderErrorClass())
	}
}

func TestSubmitRateLimitedClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationTextGenerate,
		Modality:  domain.ModalityText,
		Prompt:    "hello",
	})
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if perr.ProviderErrorClass() != domain.ProviderErrRateLimited {
		t.Fatalf("class = %q, want rate_limited", perr.ProviderErrorClass())
	}
}

func TestSubmitPollVideoSuccess(t *testing.T) {
	const videoURL = "https://cdn.example.test/output.mp4"
	var seen nativeVideoRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); !strings.Contains(got, "/v1/inference/PrunaAI%2Fp-video") {
			t.Fatalf("path = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"video_url":"` + videoURL + `"}`))
	}))
	defer srv.Close()

	p := New(Config{
		APIKey:           "test-key",
		BaseURL:          srv.URL,
		HTTPClient:       srv.Client(),
		VideoDraft:       true,
		VideoDurationSec: 5,
		VideoResolution:  "720p",
		VideoAspectRatio: "16:9",
	})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      defaultVideoModel,
		Prompt:         "snow in neon city",
		DurationSec:    5,
		Resolution:     "720p",
		AspectRatio:    "16:9",
		Draft:          true,
		IdempotencyKey: "provider_submit:video:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.ModelCode != defaultVideoModel {
		t.Fatalf("model = %q, want %q", task.ModelCode, defaultVideoModel)
	}
	if !seen.Draft || seen.Duration != 5 || seen.Resolution != "720p" || seen.AspectRatio != "16:9" || seen.Prompt != "snow in neon city" {
		t.Fatalf("unexpected video body: %+v", seen)
	}

	task, err = p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      defaultVideoModel,
		Prompt:         "short clip",
		DurationSec:    3,
		Resolution:     "720p",
		AspectRatio:    "16:9",
		Draft:          true,
		IdempotencyKey: "provider_submit:video:3",
	})
	if err != nil {
		t.Fatalf("submit duration 3: %v", err)
	}
	if seen.Duration != 3 {
		t.Fatalf("duration = %d, want 3", seen.Duration)
	}
	_ = task

	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 || res.OutputURLs[0] != videoURL {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSubmitVideoRejectsInvalidDuration(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		Prompt:      "clip",
		DurationSec: 15,
	})
	perr, ok := err.(*Error)
	if !ok || perr.Class != domain.ProviderErrInvalidRequest {
		t.Fatalf("expected invalid_request, got %v", err)
	}
}

func TestUnsupportedOperation(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{Operation: domain.OperationImageEdit, Modality: domain.ModalityImage})
	if perr, ok := err.(*Error); !ok || perr.Class != domain.ProviderErrUnsupportedCapab {
		t.Fatalf("expected unsupported_capability error, got %v", err)
	}
}

func TestPollUnknownTask(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderDeepInfra, ExternalID: "missing"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskFailed || res.ErrorClass != domain.ProviderErrTaskNotFound {
		t.Fatalf("unexpected result: %+v", res)
	}
}
