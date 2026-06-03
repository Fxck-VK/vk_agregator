package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestSubmitPollImageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing/incorrect auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/img.png"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	ctx := context.Background()

	task, err := p.Submit(ctx, domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Prompt:    "a red apple",
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
	_, err := p.Submit(context.Background(), domain.ProviderRequest{Operation: domain.OperationTextGenerate})
	if perr, ok := err.(*Error); !ok || perr.Class != domain.ProviderErrUnsupportedCapab {
		t.Fatalf("expected unsupported_capability error, got %v", err)
	}
}
