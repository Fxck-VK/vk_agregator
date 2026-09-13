package websession

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func TestWebPaidTextUsesServerRoutingAndPrice(t *testing.T) {
	for _, id := range []string{"gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro", "claude_opus_4_8", "gpt_5_6_terra", "gpt_6_astra", "claude_opus_5", "gemini_3_7_flash", "claude_fable_5_1", "claude_fable_5", "gemini_3_6_flash"} {
		t.Run(id, func(t *testing.T) {
			h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
			prices, _ := pricingcatalog.NewStaticCatalog()
			h.deps.ImagePricing = prices
			h.cfg.TextModels = textgeneration.Models([]string{id}, prices)
			owner := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
			jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
			body := `{"prompt":"Synthetic text","model_id":"` + id + `"}`
			recorder := httptest.NewRecorder()
			h.Routes().ServeHTTP(recorder, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
			if recorder.Code != http.StatusCreated || len(jobs.inputs) != 1 {
				t.Fatalf("status %d", recorder.Code)
			}
			in := jobs.inputs[0]
			expectedProvider := domain.ProviderKIE
			if id == "claude_fable_5_1" {
				expectedProvider = domain.ProviderAPIMart
			}
			var params webChatJobParams
			_ = json.Unmarshal(in.Params, &params)
			if !in.PricingSnapshot.Valid() || in.PricingSnapshot.Key.TextModelID != id || params.Provider != expectedProvider || params.ModelID != id {
				t.Fatal("missing trusted price/route")
			}
			h.cfg.TextModels = nil
			recorder = httptest.NewRecorder()
			h.Routes().ServeHTTP(recorder, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
			if recorder.Code < 400 || len(jobs.inputs) != 1 {
				t.Fatal("disabled model submitted")
			}
		})
	}
}

func TestWebPaidTextRejectsClientProviderAndPrice(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	for _, field := range []string{`"provider":"kie"`, `"cost_estimate":0`, `"pricing_snapshot":{}`, `"model_code":"gpt-5-5"`} {
		recorder := httptest.NewRecorder()
		h.Routes().ServeHTTP(recorder, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), `{"prompt":"Synthetic","model_id":"gpt_5_5",`+field+`}`))
		if recorder.Code != http.StatusBadRequest || len(jobs.inputs) != 0 {
			t.Fatal("client execution fields accepted")
		}
	}
}
