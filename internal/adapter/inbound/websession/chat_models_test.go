package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

func TestWebChatModelsRequireSessionAndExposeOnlyPublicChoices(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	unauthenticated := httptest.NewRecorder()
	h.Routes().ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/web/v1/chat-models", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.Code)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/chat-models", sessions, uuid.New()))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("catalog status/cache = %d/%s", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var payload struct {
		Items          []map[string]string `json:"items"`
		DefaultModelID string              `json:"default_model_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	defaultModel, _ := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, "")
	if len(payload.Items) == 0 || payload.DefaultModelID != defaultModel.ModelID || payload.Items[0]["id"] != payload.DefaultModelID {
		t.Fatal("catalog must contain its default model first")
	}
	seen := map[string]bool{}
	for _, item := range payload.Items {
		model, ok := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, item["id"])
		if len(item) != 2 || !ok || item["id"] != model.ModelID || item["name"] != model.ModelName || seen[item["id"]] {
			t.Fatal("catalog must contain unique public IDs and names only")
		}
		seen[item["id"]] = true
	}
}

func TestWebChatUsesTheRequestedPublicModelInTheSameConversation(t *testing.T) {
	for _, model := range modelcatalog.ListMiniAppModels(domain.OperationTextGenerate) {
		t.Run(model.ModelID, func(t *testing.T) {
			h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
			jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
			body, _ := json.Marshal(map[string]string{"prompt": "continue", "model_id": model.ModelID})
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), string(body)))
			if rec.Code != http.StatusCreated || len(jobs.inputs) != 1 {
				t.Fatalf("status = %d, jobs = %d", rec.Code, len(jobs.inputs))
			}
			var params webChatJobParams
			if err := json.Unmarshal(jobs.inputs[0].Params, &params); err != nil {
				t.Fatal(err)
			}
			if params.ModelID != model.ModelID || params.ConversationID != conversation.ID.String() {
				t.Fatal("model choice must be captured on a job in the existing conversation")
			}
		})
	}
}
