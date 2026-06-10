package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/moderationservice"
)

func TestSubmitPollImageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing/incorrect auth header: %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:test:1" {
			t.Errorf("idempotency header = %q", got)
		}
		if r.URL.Path != "/images/generations" {
			t.Errorf("path = %q, want /images/generations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/img.png"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	ctx := context.Background()

	task, err := p.Submit(ctx, domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Prompt:         "a red apple",
		IdempotencyKey: "provider_submit:test:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.ExternalID == "" || task.Provider != domain.ProviderOpenAI {
		t.Fatalf("unexpected task: %+v", task)
	}

	res, err := p.Poll(ctx, domain.ProviderTaskRef{Provider: domain.ProviderOpenAI, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded {
		t.Fatalf("status = %q, want succeeded", res.Status)
	}
	if len(res.OutputURLs) != 1 || res.OutputURLs[0] != "https://cdn.example.com/img.png" {
		t.Fatalf("unexpected output urls: %+v", res.OutputURLs)
	}
}

func TestSubmitPollTextSuccess(t *testing.T) {
	var seen responsesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path = %q, want /responses", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output_text":"hello from responses"}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:           uuid.New(),
		Operation:       domain.OperationTextGenerate,
		Modality:        domain.ModalityText,
		Prompt:          "hello",
		MaxOutputTokens: 800,
	})
	if err != nil {
		t.Fatalf("submit text: %v", err)
	}
	if seen.Input != "hello" || !strings.Contains(seen.Instructions, "3000 characters") || !strings.Contains(seen.Instructions, "НейроХаб бот") || !strings.Contains(seen.Instructions, "model name") || seen.Store {
		t.Fatalf("unexpected text request: %+v", seen)
	}
	if seen.MaxOutputTokens != 800 {
		t.Fatalf("max_output_tokens = %d, want 800", seen.MaxOutputTokens)
	}

	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll text: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 {
		t.Fatalf("unexpected text result: %+v", res)
	}
	if res.Text != "hello from responses" {
		t.Fatalf("result text = %q", res.Text)
	}
	if !strings.HasPrefix(res.OutputURLs[0], "data:text/plain; charset=utf-8;base64,") {
		t.Fatalf("expected text data url, got %q", res.OutputURLs[0])
	}
}

func TestSubmitPollImageB64Success(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("png bytes"))
	var seen imageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode image request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + encoded + `"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		ModelCode: "gpt-image-2",
		Size:      "1536x1024",
		Prompt:    "a red apple",
	})
	if err != nil {
		t.Fatalf("submit image b64: %v", err)
	}
	if seen.Model != "gpt-image-2" || seen.Size != "1536x1024" {
		t.Fatalf("unexpected image request: %+v", seen)
	}
	if task.ModelCode != "gpt-image-2" {
		t.Fatalf("task model = %q, want gpt-image-2", task.ModelCode)
	}
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll image b64: %v", err)
	}
	if len(res.OutputURLs) != 1 || !strings.HasPrefix(res.OutputURLs[0], "data:image/png;base64,") {
		t.Fatalf("unexpected image data url: %+v", res.OutputURLs)
	}
}

func TestSubmitPollVideoSuccess(t *testing.T) {
	var contentFetched bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/videos":
			_, _ = w.Write([]byte(`{"id":"vid_123","status":"queued"}`))
		case "/videos/vid_123":
			_, _ = w.Write([]byte(`{"id":"vid_123","status":"completed"}`))
		case "/videos/vid_123/content":
			contentFetched = true
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("mp4 bytes"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Prompt:    "make a video",
	})
	if err != nil {
		t.Fatalf("submit video: %v", err)
	}
	if task.ExternalID != "vid_123" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("unexpected submitted video task: %+v", task)
	}
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll video: %v", err)
	}
	if !contentFetched || res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 ||
		!strings.HasPrefix(res.OutputURLs[0], "data:video/mp4;base64,") {
		t.Fatalf("unexpected video result: %+v contentFetched=%v", res, contentFetched)
	}
}

func TestSubmitRateLimitedClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "x",
	})
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if perr.ProviderErrorClass() != domain.ProviderErrRateLimited {
		t.Fatalf("class = %q, want rate_limited", perr.ProviderErrorClass())
	}
}

func TestUnsupportedOperation(t *testing.T) {
	p := New(Config{APIKey: "k"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{Operation: domain.OperationImageEdit})
	if perr, ok := err.(*Error); !ok || perr.Class != domain.ProviderErrUnsupportedCapab {
		t.Fatalf("expected unsupported_capability error, got %v", err)
	}
}

func TestOpenAIModerationBlocksText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/moderations" {
			t.Errorf("path = %q, want /moderations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"flagged":true,"categories":{"violence":true,"hate":false}}]}`))
	}))
	defer srv.Close()

	m := NewModerator(ModerationConfig{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	out, err := m.Check(context.Background(), moderationInput("bad text"))
	if err != nil {
		t.Fatalf("moderation check: %v", err)
	}
	if out.Decision != domain.ModerationBlock || len(out.Categories) != 1 || out.Categories[0] != "violence" {
		t.Fatalf("unexpected moderation outcome: %+v", out)
	}
}

func TestOpenAIScannerRejectsFlaggedImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"flagged":true,"categories":{"sexual":true}}]}`))
	}))
	defer srv.Close()

	m := NewModerator(ModerationConfig{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err := m.Scan(context.Background(), domain.MediaTypeImage, "image/png", []byte("png")); err == nil {
		t.Fatal("expected scanner rejection")
	}
}

func moderationInput(text string) moderationservice.Input {
	return moderationservice.Input{Stage: domain.ModerationStageOutput, Modality: domain.ModalityText, Text: text}
}
