package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestConversationMessageHistoryReturnsSafeOwnerScopedPage(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	ownerID := uuid.New()
	foreignAccountID := uuid.New()
	owned := seedWebConversation(t, conversations, ownerID, "owned-thread")
	foreign := seedWebConversation(t, conversations, foreignAccountID, "foreign-thread")

	first := seedConversationMessage(t, conversations, owned.ID, domain.ConversationRoleUser, "Первая задача", 42)
	second := seedConversationMessage(t, conversations, owned.ID, domain.ConversationRoleAssistant, "Первый результат", 128)
	_ = seedConversationMessage(t, conversations, foreign.ID, domain.ConversationRoleUser, "Чужое сообщение", 777)

	req := authenticatedConversationRequest(
		t,
		http.MethodGet,
		"/web/v1/conversations/"+owned.ID.String()+"/messages?after_seq="+strconv.FormatInt(first.Seq, 10)+"&limit=1",
		sessions,
		ownerID,
	)
	req.Header.Set("X-Account-ID", foreignAccountID.String())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	items := assertSafeConversationMessageList(t, rec.Body.Bytes())
	if len(items) != 1 {
		t.Fatalf("message count = %d, want 1", len(items))
	}
	if items[0].ID != second.ID || items[0].Seq != second.Seq || items[0].Role != string(domain.ConversationRoleAssistant) || items[0].Text != second.Text {
		t.Fatalf("message = %#v, want safe second owned message", items[0])
	}
}

func TestConversationMessageHistoryDefaultsToNewestBoundedPageAndPagesBackward(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	ownerID := uuid.New()
	conversation := seedWebConversation(t, conversations, ownerID, "paged-thread")

	for i := 0; i < 201; i++ {
		role := domain.ConversationRoleUser
		if i%2 == 1 {
			role = domain.ConversationRoleAssistant
		}
		seedConversationMessage(t, conversations, conversation.ID, role, strconv.Itoa(i+1), i+1)
	}

	initialRequest := authenticatedConversationRequest(
		t,
		http.MethodGet,
		"/web/v1/conversations/"+conversation.ID.String()+"/messages?limit=100",
		sessions,
		ownerID,
	)
	initialRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(initialRecorder, initialRequest)

	if initialRecorder.Code != http.StatusOK {
		t.Fatalf("initial status = %d, body = %s", initialRecorder.Code, initialRecorder.Body.String())
	}
	initialItems := assertSafeConversationMessageList(t, initialRecorder.Body.Bytes())
	if len(initialItems) != 100 {
		t.Fatalf("initial message count = %d, want 100", len(initialItems))
	}
	if initialItems[0].Seq != 102 || initialItems[len(initialItems)-1].Seq != 201 {
		t.Fatalf("initial page = %d..%d, want 102..201", initialItems[0].Seq, initialItems[len(initialItems)-1].Seq)
	}
	if !assertHasMoreBefore(t, initialRecorder.Body.Bytes()) {
		t.Fatal("initial page must report older messages")
	}

	olderRequest := authenticatedConversationRequest(
		t,
		http.MethodGet,
		"/web/v1/conversations/"+conversation.ID.String()+"/messages?before_seq=102&limit=100",
		sessions,
		ownerID,
	)
	olderRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(olderRecorder, olderRequest)

	if olderRecorder.Code != http.StatusOK {
		t.Fatalf("older status = %d, body = %s", olderRecorder.Code, olderRecorder.Body.String())
	}
	olderItems := assertSafeConversationMessageList(t, olderRecorder.Body.Bytes())
	if len(olderItems) != 100 {
		t.Fatalf("older message count = %d, want 100", len(olderItems))
	}
	if olderItems[0].Seq != 2 || olderItems[len(olderItems)-1].Seq != 101 {
		t.Fatalf("older page = %d..%d, want 2..101", olderItems[0].Seq, olderItems[len(olderItems)-1].Seq)
	}
	if !assertHasMoreBefore(t, olderRecorder.Body.Bytes()) {
		t.Fatal("second page must report one older message")
	}

	firstRequest := authenticatedConversationRequest(
		t,
		http.MethodGet,
		"/web/v1/conversations/"+conversation.ID.String()+"/messages?before_seq=2&limit=100",
		sessions,
		ownerID,
	)
	firstRecorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstRecorder.Code, firstRecorder.Body.String())
	}
	firstItems := assertSafeConversationMessageList(t, firstRecorder.Body.Bytes())
	if len(firstItems) != 1 || firstItems[0].Seq != 1 {
		t.Fatalf("first page = %#v, want only sequence 1", firstItems)
	}
	if assertHasMoreBefore(t, firstRecorder.Body.Bytes()) {
		t.Fatal("oldest page must not report older messages")
	}
}

func TestConversationMessageHistoryRejectsForeignAndNonWebConversations(t *testing.T) {
	for _, test := range []struct {
		name               string
		requestAccountID   uuid.UUID
		conversationSource domain.ConversationSource
		conversationOwner  uuid.UUID
	}{
		{name: "foreign account", requestAccountID: uuid.New(), conversationSource: domain.ConversationSourceWeb, conversationOwner: uuid.New()},
		{name: "other surface", requestAccountID: uuid.New(), conversationSource: domain.ConversationSourceMiniApp},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, conversations, sessions := newConversationTestHandler(t)
			if test.conversationOwner == uuid.Nil {
				test.conversationOwner = test.requestAccountID
			}
			conversation := &domain.Conversation{
				AccountID:        test.conversationOwner,
				Source:           test.conversationSource,
				ExternalThreadID: uuid.NewString(),
			}
			if err := conversations.CreateConversation(context.Background(), conversation); err != nil {
				t.Fatalf("seed conversation: %v", err)
			}
			_ = seedConversationMessage(t, conversations, conversation.ID, domain.ConversationRoleUser, "Private", 1)

			req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations/"+conversation.ID.String()+"/messages", sessions, test.requestAccountID)
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if rec.Body.String() == "" {
				t.Fatal("not found response must remain a JSON error")
			}
		})
	}
}

func TestConversationMessageHistoryRejectsMalformedPathAndPaginationBeforeRepository(t *testing.T) {
	for _, path := range []string{
		"/web/v1/conversations/not-a-uuid/messages",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?after_seq=",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?after_seq=-1",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?after_seq=not-a-number",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?after_seq=1&after_seq=2",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?before_seq=",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?before_seq=0",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?before_seq=-1",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?before_seq=not-a-number",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?before_seq=1&before_seq=2",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?after_seq=1&before_seq=2",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?limit=",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?limit=0",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?limit=101",
		"/web/v1/conversations/" + uuid.NewString() + "/messages?limit=1&limit=2",
	} {
		t.Run(path, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			req := authenticatedConversationRequest(t, http.MethodGet, path, sessions, uuid.New())
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestConversationMessageHistoryRequiresAccessCookie(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/web/v1/conversations/"+uuid.NewString()+"/messages", nil)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func seedWebConversation(t *testing.T, conversations *memory.ConversationRepo, accountID uuid.UUID, externalThreadID string) *domain.Conversation {
	t.Helper()
	conversation := &domain.Conversation{
		AccountID:        accountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: externalThreadID,
	}
	if err := conversations.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("seed web conversation: %v", err)
	}
	return conversation
}

func seedConversationMessage(t *testing.T, conversations *memory.ConversationRepo, conversationID uuid.UUID, role domain.ConversationMessageRole, text string, tokenCount int) *domain.ConversationMessage {
	t.Helper()
	message, err := conversations.UpsertMessage(context.Background(), &domain.ConversationMessage{
		ConversationID: conversationID,
		JobID:          uuid.New(),
		Role:           role,
		Text:           text,
		TokenCount:     tokenCount,
	})
	if err != nil {
		t.Fatalf("seed conversation message: %v", err)
	}
	return message
}

type safeConversationMessageDTO struct {
	ID     uuid.UUID
	Seq    int64
	Role   string
	Text   string
	Rating domain.ConversationMessageRating
}

func assertSafeConversationMessageList(t *testing.T, body []byte) []safeConversationMessageDTO {
	t.Helper()
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["items"] == nil {
		t.Fatalf("response fields = %v, want items", response)
	}
	for field := range response {
		if field != "items" && field != "has_more_before" {
			t.Fatalf("response exposes unexpected field %q: %v", field, response)
		}
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(response["items"], &items); err != nil {
		t.Fatalf("decode message items: %v", err)
	}
	if items == nil {
		t.Fatal("message items must be an empty array, not null")
	}
	result := make([]safeConversationMessageDTO, 0, len(items))
	for _, item := range items {
		result = append(result, safeConversationMessageFields(t, item))
	}
	return result
}

func assertHasMoreBefore(t *testing.T, body []byte) bool {
	t.Helper()
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	value, ok := response["has_more_before"]
	if !ok {
		t.Fatalf("response fields = %v, want has_more_before", response)
	}
	var hasMoreBefore bool
	if err := json.Unmarshal(value, &hasMoreBefore); err != nil {
		t.Fatalf("has_more_before = %s, want boolean: %v", value, err)
	}
	return hasMoreBefore
}

func safeConversationMessageFields(t *testing.T, fields map[string]json.RawMessage) safeConversationMessageDTO {
	t.Helper()
	forbidden := []string{"conversation_id", "job_id", "token_count", "account_id", "user_id", "source"}
	for _, field := range forbidden {
		if _, ok := fields[field]; ok {
			t.Fatalf("response exposes forbidden field %q: %v", field, fields)
		}
	}
	if len(fields) != 6 || fields["id"] == nil || fields["seq"] == nil || fields["role"] == nil || fields["text"] == nil || fields["rating"] == nil || fields["created_at"] == nil {
		t.Fatalf("response fields = %v, want id, seq, role, text, rating, created_at", fields)
	}
	var dto safeConversationMessageDTO
	var idRaw string
	if err := json.Unmarshal(fields["id"], &idRaw); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	parsedID, err := uuid.Parse(idRaw)
	if err != nil || parsedID == uuid.Nil {
		t.Fatalf("id = %q, want UUID: %v", idRaw, err)
	}
	dto.ID = parsedID
	if err := json.Unmarshal(fields["seq"], &dto.Seq); err != nil || dto.Seq < 1 {
		t.Fatalf("seq = %s, want positive integer: %v", fields["seq"], err)
	}
	if err := json.Unmarshal(fields["role"], &dto.Role); err != nil || (dto.Role != string(domain.ConversationRoleUser) && dto.Role != string(domain.ConversationRoleAssistant)) {
		t.Fatalf("role = %s, want supported role: %v", fields["role"], err)
	}
	if err := json.Unmarshal(fields["text"], &dto.Text); err != nil {
		t.Fatalf("decode text: %v", err)
	}
	if string(fields["rating"]) != "null" {
		if err := json.Unmarshal(fields["rating"], &dto.Rating); err != nil || !dto.Rating.Valid() || dto.Rating == domain.ConversationMessageRatingNone {
			t.Fatalf("rating = %s, want null, like or dislike: %v", fields["rating"], err)
		}
	}
	var createdAt string
	if err := json.Unmarshal(fields["created_at"], &createdAt); err != nil {
		t.Fatalf("decode created_at: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		t.Fatalf("created_at = %q, want RFC3339: %v", createdAt, err)
	}
	return dto
}
