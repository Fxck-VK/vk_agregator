package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	providertest "vk-ai-aggregator/internal/adapter/provider/providertest"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/moderationservice"
)

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

func TestOpenAIModerationHTTPErrorIsNormalizedAndRedacted(t *testing.T) {
	const fakeSecret = "openai-secret-fixture"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid bearer ` + fakeSecret + `"}}`))
	}))
	defer srv.Close()

	m := NewModerator(ModerationConfig{APIKey: fakeSecret, BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := m.Check(context.Background(), moderationInput("safe text"))
	providertest.RequireErrorClass(t, err, domain.ProviderErrAuthFailed)
	providertest.RequireErrorDoesNotContain(t, err, fakeSecret)
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

func TestOpenAIScannerSkipsVideoArtifactScan(t *testing.T) {
	m := NewModerator(ModerationConfig{APIKey: "test-key"})
	if err := m.Scan(context.Background(), domain.MediaTypeVideo, "video/mp4", []byte("mp4")); err != nil {
		t.Fatalf("expected video scan to be skipped, got: %v", err)
	}
}

func TestOpenAIScannerFailsClosedForUnknownMediaType(t *testing.T) {
	m := NewModerator(ModerationConfig{APIKey: "test-key"})
	if err := m.Scan(context.Background(), domain.MediaType("unknown"), "application/octet-stream", []byte("x")); err == nil {
		t.Fatal("expected unsupported media type scanner error")
	}
}

func moderationInput(text string) moderationservice.Input {
	return moderationservice.Input{Stage: domain.ModerationStageOutput, Modality: domain.ModalityText, Text: text}
}
