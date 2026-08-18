package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestConversationMessageRatingPersistsAndCanBeCleared(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	accountID := uuid.New()
	conversation := seedWebConversation(t, conversations, accountID, "rated-thread")
	assistant := seedConversationMessage(t, conversations, conversation.ID, domain.ConversationRoleAssistant, "Answer", 12)

	path := "/web/v1/conversations/" + conversation.ID.String() + "/messages/" + assistant.ID.String() + "/rating"
	likeRequest := safeConversationManagementRequest(t, http.MethodPut, path, sessions, accountID, `{"rating":"like"}`)
	likeRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(likeRecorder, likeRequest)

	if likeRecorder.Code != http.StatusOK {
		t.Fatalf("like status = %d, body = %s", likeRecorder.Code, likeRecorder.Body.String())
	}
	assertSafeMessageRating(t, likeRecorder.Body.Bytes(), domain.ConversationMessageRatingLike)

	historyRequest := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations/"+conversation.ID.String()+"/messages", sessions, accountID)
	historyRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(historyRecorder, historyRequest)
	items := assertSafeConversationMessageList(t, historyRecorder.Body.Bytes())
	if len(items) != 1 || items[0].Rating != domain.ConversationMessageRatingLike {
		t.Fatalf("history rating = %#v, want like", items)
	}

	clearRequest := safeConversationManagementRequest(t, http.MethodPut, path, sessions, accountID, `{"rating":null}`)
	clearRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(clearRecorder, clearRequest)
	if clearRecorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d, body = %s", clearRecorder.Code, clearRecorder.Body.String())
	}
	assertSafeMessageRating(t, clearRecorder.Body.Bytes(), domain.ConversationMessageRatingNone)
}

func TestConversationMessageRatingRejectsInvalidOrUnownedTargets(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		body       string
		foreign    bool
		role       domain.ConversationMessageRole
		wantStatus int
	}{
		{name: "invalid rating", body: `{"rating":"love"}`, role: domain.ConversationRoleAssistant, wantStatus: http.StatusBadRequest},
		{name: "missing rating", body: `{}`, role: domain.ConversationRoleAssistant, wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"rating":"like","extra":true}`, role: domain.ConversationRoleAssistant, wantStatus: http.StatusBadRequest},
		{name: "user message", body: `{"rating":"like"}`, role: domain.ConversationRoleUser, wantStatus: http.StatusNotFound},
		{name: "foreign account", body: `{"rating":"like"}`, foreign: true, role: domain.ConversationRoleAssistant, wantStatus: http.StatusNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h, conversations, sessions := newConversationTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebConversation(t, conversations, accountID, uuid.NewString())
			message := seedConversationMessage(t, conversations, conversation.ID, testCase.role, "Message", 1)
			requestAccountID := accountID
			if testCase.foreign {
				requestAccountID = uuid.New()
			}
			path := "/web/v1/conversations/" + conversation.ID.String() + "/messages/" + message.ID.String() + "/rating"
			req := safeConversationManagementRequest(t, http.MethodPut, path, sessions, requestAccountID, testCase.body)
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != testCase.wantStatus {
				t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), testCase.wantStatus)
			}
		})
	}
}

func assertSafeMessageRating(t *testing.T, body []byte, want domain.ConversationMessageRating) {
	t.Helper()
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode rating response: %v", err)
	}
	if len(response) != 1 || response["rating"] == nil {
		t.Fatalf("rating response fields = %v", response)
	}
	if want == domain.ConversationMessageRatingNone {
		if string(response["rating"]) != "null" {
			t.Fatalf("rating = %s, want null", response["rating"])
		}
		return
	}
	var got domain.ConversationMessageRating
	if err := json.Unmarshal(response["rating"], &got); err != nil || got != want {
		t.Fatalf("rating = %q, %v, want %q", got, err, want)
	}
}
