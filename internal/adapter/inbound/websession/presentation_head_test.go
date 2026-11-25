package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestPresentationHeadProvesOwnedPresentationWithoutReturningContent(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	h.cfg.Environment = "development"
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobID := uuid.New()
	key := "web-presentation:" + owner.String() + ":" + uuid.NewString()
	params, err := json.Marshal(webChatJobParams{
		Prompt: "private presentation topic", ConversationID: conversation.ID.String(), ConversationSource: domain.ConversationSourceWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	job := &domain.Job{
		ID: jobID, AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode: domain.ResultModeAccountHistory, OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText,
		Status: domain.JobStatusQueued, IdempotencyKey: key, CorrelationID: key, Params: params,
	}
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	base := "/web/v1/presentations/" + conversation.ID.String()
	probe := func(account uuid.UUID, path, release string, expected int) {
		t.Helper()
		req := authenticatedConversationRequest(t, http.MethodHead, path, sessions, account)
		req.Header.Set("X-Epifera-Release-View", release)
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, req)
		if w.Code != expected {
			t.Errorf("status = %d, want %d", w.Code, expected)
		}
		if expected == http.StatusNoContent && (w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-store") {
			t.Error("successful HEAD must not return content and must not be cached")
		}
	}

	probe(owner, base+"?job_id="+jobID.String(), "dev", http.StatusNoContent)
	probe(uuid.New(), base+"?job_id="+jobID.String(), "dev", http.StatusNotFound)
	probe(owner, base+"?job_id="+jobID.String(), "prod", http.StatusNotFound)
	if _, err := conversations.UpsertMessage(context.Background(), &domain.ConversationMessage{
		ConversationID: conversation.ID, JobID: jobID, Role: domain.ConversationRoleUser, Text: "Stored private prompt",
	}); err != nil {
		t.Fatal(err)
	}
	probe(owner, base, "dev", http.StatusNoContent)
	job.IdempotencyKey = "web-chat:" + owner.String() + ":" + uuid.NewString()
	probe(owner, base, "dev", http.StatusNotFound)
}
