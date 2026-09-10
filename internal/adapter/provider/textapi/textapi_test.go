package textapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestPaidPOSTNeverFollowsRedirectOrRetries(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusInternalServerError, http.StatusTooManyRequests} {
		calls, redirects := 0, 0
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects++ }))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Location", target.URL)
			w.WriteHeader(status)
		}))
		p := New(Config{Provider: domain.ProviderAPIMart, APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{"claude-fable-5.1"}})
		req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: "claude-fable-5.1", Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic"}
		_, err := p.Submit(context.Background(), req)
		srv.Close()
		target.Close()
		if err == nil || calls != 1 || redirects != 0 {
			t.Fatalf("status=%d calls=%d redirects=%d", status, calls, redirects)
		}
	}
}

func TestTextRejectsCrossProviderRouteBeforeHTTP(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	p := New(Config{Provider: domain.ProviderAPIMart, APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{"claude-fable-5.1", "gpt-6-astra"}})
	for _, req := range []domain.ProviderRequest{
		{Provider: domain.ProviderKIE, ModelCode: "claude-fable-5.1"},
		{Provider: domain.ProviderAPIMart, ModelCode: "gpt-6-astra"},
	} {
		req.Operation, req.Modality, req.Prompt = domain.OperationTextGenerate, domain.ModalityText, "Synthetic"
		if _, err := p.Submit(context.Background(), req); err == nil || calls != 0 {
			t.Fatal("cross-provider route sent")
		}
	}
}
