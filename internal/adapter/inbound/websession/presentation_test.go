package websession

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func TestPresentationQuoteAndAccountJob(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	h.deps.ImagePricing = prices
	h.cfg.Environment = "development"
	h.cfg.TextModels = textgeneration.Models([]string{providermodels.PublicTextGemini37Flash}, prices)
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	quoteRequest := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/presentations/quote", sessions, owner)
	quoteRequest.Header.Set("X-Epifera-Release-View", "dev")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, quoteRequest)
	var quote struct {
		Model   string `json:"model_id"`
		Credits int64  `json:"credits"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &quote) != nil || quote.Credits <= 0 || quote.Model != providermodels.PublicTextGemini37Flash {
		t.Fatalf("unexpected presentation quote, status %d", w.Code)
	}
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	generate := func(account uuid.UUID, price int64, view string) int {
		body := fmt.Sprintf(`{"prompt":"Test","model_id":%q,"expected_presentation_credits":%d}`, quote.Model, price)
		req := safeWebConversationMessageRequest(t, sessions, account, conversation.ID, uuid.New(), body)
		req.URL.Path = "/web/v1/presentations/" + conversation.ID.String() + "/generate"
		req.Header.Set("X-Epifera-Release-View", view)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec.Code
	}
	if generate(owner, quote.Credits+1, "dev") != http.StatusConflict ||
		generate(uuid.New(), quote.Credits, "dev") != http.StatusNotFound ||
		generate(owner, quote.Credits, "prod") != http.StatusNotFound || len(jobs.inputs) != 0 {
		t.Fatal("presentation admitted a mismatched quote, account or release")
	}
	if code := generate(owner, quote.Credits, "dev"); code != http.StatusCreated || len(jobs.inputs) != 1 {
		t.Fatalf("presentation job not accepted, status %d", code)
	}
	input := jobs.inputs[0]
	if input.AccountID != owner || input.ResultMode != domain.ResultModeAccountHistory || input.PricingSnapshot.InternalCredits != quote.Credits {
		t.Fatal("presentation job lost its account, delivery scope or server price")
	}
}
