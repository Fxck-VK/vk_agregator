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

	"vk-ai-aggregator/internal/domain"
)

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

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Prompt:         "hello",
		IdempotencyKey: "provider_submit:test:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderDeepInfra || task.ModelCode != defaultTextModel {
		t.Fatalf("unexpected task: %+v", task)
	}
	if seen.Model != defaultTextModel || len(seen.Messages) != 2 || seen.Messages[0].Role != "system" || !strings.Contains(seen.Messages[0].Content, "3000 characters") || !strings.Contains(seen.Messages[0].Content, "НейроХаб бот") || !strings.Contains(seen.Messages[0].Content, "model name") || seen.Messages[1].Content != "hello" || seen.Stream {
		t.Fatalf("unexpected request body: %+v", seen)
	}

	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: task.Provider, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 {
		t.Fatalf("unexpected result: %+v", res)
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

func TestUnsupportedOperation(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{Operation: domain.OperationImageGenerate})
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
