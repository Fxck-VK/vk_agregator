package websession

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

func TestCreateWebConversationMessageQueuesOwnedTextJob(t *testing.T) {
	h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
	accountID := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
	jobID := uuid.New()
	jobs.job = &domain.Job{ID: jobID, Status: domain.JobStatusQueued}
	idempotencyKey := uuid.New()

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, idempotencyKey, `{"prompt":"  hello web  "}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	assertSafeQueuedWebChatResponse(t, rec.Body.Bytes(), jobID)
	if limiter.calls != 1 || len(limiter.keys) != 1 || limiter.keys[0] != "account:"+accountID.String() {
		t.Fatalf("limiter calls/keys = %d/%v", limiter.calls, limiter.keys)
	}
	if len(jobs.inputs) != 1 {
		t.Fatalf("CreateJob calls = %d, want 1", len(jobs.inputs))
	}
	in := jobs.inputs[0]
	if in.AccountID != accountID || in.UserID != uuid.Nil || in.Source != "web" {
		t.Fatalf("owner/source contract = account:%s user:%s source:%q", in.AccountID, in.UserID, in.Source)
	}
	if in.ChannelContext == nil || in.ChannelContext.Channel != domain.ChannelWeb || in.ChannelContext.RecipientRef != "" || in.ChannelContext.ThreadRef != "" {
		t.Fatalf("channel context = %+v, want web-only provenance", in.ChannelContext)
	}
	if in.ResultMode != domain.ResultModeAccountHistory || in.DeliveryTarget != nil || in.VKPeerID != 0 || in.CommandID != uuid.Nil {
		t.Fatalf("result contract = mode:%q target:%+v peer:%d command:%s", in.ResultMode, in.DeliveryTarget, in.VKPeerID, in.CommandID)
	}
	if in.Operation != domain.OperationTextGenerate || in.Modality != domain.ModalityText {
		t.Fatalf("operation/modality = %q/%q", in.Operation, in.Modality)
	}
	wantIdempotencyKey := "web-chat:" + accountID.String() + ":" + idempotencyKey.String()
	if in.IdempotencyKey != wantIdempotencyKey {
		t.Fatalf("idempotency key = %q, want %q", in.IdempotencyKey, wantIdempotencyKey)
	}
	model, ok := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, "")
	if !ok {
		t.Fatal("default text model is missing")
	}
	var params struct {
		Prompt             string                    `json:"prompt"`
		ModelID            string                    `json:"model_id"`
		ModelName          string                    `json:"model_name"`
		Provider           domain.ProviderName       `json:"provider"`
		ModelCode          string                    `json:"model_code"`
		ConversationID     string                    `json:"conversation_id"`
		ConversationSource domain.ConversationSource `json:"conversation_source"`
	}
	if err := json.Unmarshal(in.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.Prompt != "hello web" || params.ModelID != model.ModelID || params.ModelName != model.ModelName || params.Provider != model.Provider || params.ModelCode != model.ModelCode {
		t.Fatalf("text params = %+v, model = %+v", params, model)
	}
	if params.ConversationID != conversation.ID.String() || params.ConversationSource != domain.ConversationSourceWeb {
		t.Fatalf("conversation params = id:%q source:%q", params.ConversationID, params.ConversationSource)
	}
}

func TestCreateWebConversationMessageHidesForeignOrWrongSourceConversation(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		owner  func(accountID uuid.UUID) uuid.UUID
		source domain.ConversationSource
	}{
		{name: "foreign", owner: func(uuid.UUID) uuid.UUID { return uuid.New() }, source: domain.ConversationSourceWeb},
		{name: "wrong source", owner: func(accountID uuid.UUID) uuid.UUID { return accountID }, source: domain.ConversationSourceMiniApp},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, testCase.owner(accountID), testCase.source)

			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), `{"prompt":"hello"}`))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if len(jobs.inputs) != 0 || limiter.calls != 0 {
				t.Fatalf("downstream calls before owner/source rejection = jobs:%d limiter:%d", len(jobs.inputs), limiter.calls)
			}
		})
	}
}

func TestCreateWebConversationMessageRejectsArchivedConversation(t *testing.T) {
	h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
	accountID := uuid.New()
	conversation := seedWebMessageConversationWithStatus(t, conversations, accountID, domain.ConversationSourceWeb, domain.ConversationArchived)

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), `{"prompt":"hello"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(jobs.inputs) != 0 || limiter.calls != 0 {
		t.Fatalf("downstream calls for archived conversation = jobs:%d limiter:%d", len(jobs.inputs), limiter.calls)
	}
}

func TestCreateWebConversationMessageRequiresCSRFAndUUIDIdempotency(t *testing.T) {
	t.Run("unsafe request rejected before dependencies", func(t *testing.T) {
		h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
		accountID := uuid.New()
		conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
		conversationSpy := &webConversationRepositorySpy{ConversationRepository: conversations}
		h.deps.Conversations = conversationSpy
		req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/conversations/"+conversation.ID.String()+"/messages", sessions, accountID)
		req.Body = http.NoBody
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", uuid.NewString())

		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if conversationSpy.getCalls != 0 || len(jobs.inputs) != 0 || limiter.calls != 0 {
			t.Fatalf("dependency calls = conversations:%d jobs:%d limiter:%d", conversationSpy.getCalls, len(jobs.inputs), limiter.calls)
		}
	})

	for _, key := range []string{"", "not-a-uuid", uuid.Nil.String()} {
		t.Run("invalid key "+key, func(t *testing.T) {
			h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
			conversationSpy := &webConversationRepositorySpy{ConversationRepository: conversations}
			h.deps.Conversations = conversationSpy
			req := safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), `{"prompt":"hello"}`)
			req.Header.Set("X-Idempotency-Key", key)

			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if conversationSpy.getCalls != 0 || len(jobs.inputs) != 0 || limiter.calls != 0 {
				t.Fatalf("dependency calls = conversations:%d jobs:%d limiter:%d", conversationSpy.getCalls, len(jobs.inputs), limiter.calls)
			}
		})
	}
}

func TestCreateWebConversationMessageRateLimitsAccount(t *testing.T) {
	h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
	accountID := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
	limiter.allowed = false

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), `{"prompt":"hello"}`))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(jobs.inputs) != 0 {
		t.Fatalf("CreateJob calls = %d, want 0", len(jobs.inputs))
	}
	if limiter.calls != 1 || len(limiter.keys) != 1 || limiter.keys[0] != "account:"+accountID.String() {
		t.Fatalf("limiter calls/keys = %d/%v", limiter.calls, limiter.keys)
	}
}

func TestCreateWebConversationMessageRejectsInvalidJSONBeforeDependencies(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
	}{
		{name: "empty prompt", body: `{"prompt":"   "}`},
		{name: "unknown field", body: `{"prompt":"hello","provider":"attacker"}`},
		{name: "trailing object", body: `{"prompt":"hello"}{}`},
		{name: "oversized", body: `{"prompt":"` + strings.Repeat("x", maxRequestBytes) + `"}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
			conversationSpy := &webConversationRepositorySpy{ConversationRepository: conversations}
			h.deps.Conversations = conversationSpy

			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), testCase.body))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if conversationSpy.getCalls != 0 || len(jobs.inputs) != 0 || limiter.calls != 0 {
				t.Fatalf("dependency calls = conversations:%d jobs:%d limiter:%d", conversationSpy.getCalls, len(jobs.inputs), limiter.calls)
			}
		})
	}
}

func TestCreateWebConversationMessageMapsOrchestrationFailuresSafely(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		err        error
		wantStatus int
		wantRetry  string
	}{
		{name: "active job limit", err: domain.ErrActiveJobLimitExceeded, wantStatus: http.StatusTooManyRequests},
		{name: "insufficient credits", err: domain.ErrInsufficientCredits, wantStatus: http.StatusPaymentRequired},
		{name: "cost cap", err: domain.ErrCostCapExceeded, wantStatus: http.StatusBadRequest},
		{name: "capacity", err: domain.ErrCapacityDegraded, wantStatus: http.StatusServiceUnavailable, wantRetry: "30"},
		{name: "unknown", err: errors.New("private database failure"), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
			accountID := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
			jobs.err = testCase.err

			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, uuid.New(), `{"prompt":"hello"}`))

			if rec.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, testCase.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("Retry-After"); got != testCase.wantRetry {
				t.Fatalf("Retry-After = %q, want %q", got, testCase.wantRetry)
			}
			if strings.Contains(rec.Body.String(), testCase.err.Error()) || strings.Contains(rec.Body.String(), "database") {
				t.Fatalf("response exposes private error: %s", rec.Body.String())
			}
		})
	}
}

func TestCreateWebConversationMessageReplayUsesStableOrchestratorKey(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	jobs := &statefulWebChatJobCreatorStub{jobsByKey: make(map[string]*domain.Job)}
	h.deps.WebChatJobs = jobs
	accountID := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, accountID, domain.ConversationSourceWeb)
	idempotencyKey := uuid.New()
	var firstJobID uuid.UUID

	for attempt := range 2 {
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversation.ID, idempotencyKey, `{"prompt":"hello"}`))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var response struct {
			JobID uuid.UUID `json:"job_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if attempt == 0 {
			firstJobID = response.JobID
		} else if response.JobID != firstJobID {
			t.Fatalf("replay job = %s, want original %s", response.JobID, firstJobID)
		}
	}
	if len(jobs.inputs) != 2 {
		t.Fatalf("CreateJob calls = %d, want 2 adapter delegations to the idempotent orchestrator", len(jobs.inputs))
	}
	if len(jobs.jobsByKey) != 1 {
		t.Fatalf("durable jobs created by idempotent fake = %d, want 1", len(jobs.jobsByKey))
	}
	wantKey := "web-chat:" + accountID.String() + ":" + idempotencyKey.String()
	if jobs.inputs[0].IdempotencyKey != wantKey || jobs.inputs[1].IdempotencyKey != jobs.inputs[0].IdempotencyKey {
		t.Fatalf("replay keys = %q/%q", jobs.inputs[0].IdempotencyKey, jobs.inputs[1].IdempotencyKey)
	}
}

func TestCreateWebConversationMessageScopesIdempotencyByAccount(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	jobs := &statefulWebChatJobCreatorStub{jobsByKey: make(map[string]*domain.Job)}
	h.deps.WebChatJobs = jobs
	accountA := uuid.New()
	accountB := uuid.New()
	conversationA := seedWebMessageConversation(t, conversations, accountA, domain.ConversationSourceWeb)
	conversationB := seedWebMessageConversation(t, conversations, accountB, domain.ConversationSourceWeb)
	clientKey := uuid.New()

	ids := make([]uuid.UUID, 0, 2)
	for _, request := range []struct {
		accountID      uuid.UUID
		conversationID uuid.UUID
	}{
		{accountID: accountA, conversationID: conversationA.ID},
		{accountID: accountB, conversationID: conversationB.ID},
	} {
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, request.accountID, request.conversationID, clientKey, `{"prompt":"hello"}`))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var response struct {
			JobID uuid.UUID `json:"job_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		ids = append(ids, response.JobID)
	}
	if ids[0] == ids[1] {
		t.Fatalf("two accounts received the same job %s for one public UUID", ids[0])
	}
	if len(jobs.inputs) != 2 || jobs.inputs[0].IdempotencyKey == jobs.inputs[1].IdempotencyKey {
		t.Fatalf("account-scoped orchestration keys = %q/%q", jobs.inputs[0].IdempotencyKey, jobs.inputs[1].IdempotencyKey)
	}
}

type webChatJobCreatorStub struct {
	inputs []joborchestrator.CreateJobInput
	job    *domain.Job
	err    error
}

func (s *webChatJobCreatorStub) CreateJob(_ context.Context, input joborchestrator.CreateJobInput) (*domain.Job, error) {
	s.inputs = append(s.inputs, input)
	return s.job, s.err
}

type webChatMessageLimiterStub struct {
	allowed bool
	err     error
	calls   int
	keys    []string
}

type statefulWebChatJobCreatorStub struct {
	inputs    []joborchestrator.CreateJobInput
	jobsByKey map[string]*domain.Job
}

func (s *statefulWebChatJobCreatorStub) CreateJob(_ context.Context, input joborchestrator.CreateJobInput) (*domain.Job, error) {
	s.inputs = append(s.inputs, input)
	if job := s.jobsByKey[input.IdempotencyKey]; job != nil {
		return job, nil
	}
	job := &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	s.jobsByKey[input.IdempotencyKey] = job
	return job, nil
}

func (s *webChatMessageLimiterStub) Allow(_ context.Context, key string) (bool, error) {
	s.calls++
	s.keys = append(s.keys, key)
	return s.allowed, s.err
}

type webConversationRepositorySpy struct {
	domain.ConversationRepository
	getCalls int
}

func (s *webConversationRepositorySpy) GetByIDForAccount(ctx context.Context, accountID, conversationID uuid.UUID) (*domain.Conversation, error) {
	s.getCalls++
	return s.ConversationRepository.GetByIDForAccount(ctx, accountID, conversationID)
}

func newWebConversationMessageTestHandler(t *testing.T) (*Handler, *memory.ConversationRepo, *sessionStub, *webChatJobCreatorStub, *webChatMessageLimiterStub) {
	t.Helper()
	h, _, sessions := newTestHandler(t)
	conversations := memory.NewConversationRepo()
	jobs := &webChatJobCreatorStub{}
	limiter := &webChatMessageLimiterStub{allowed: true}
	h.deps.Conversations = conversations
	h.deps.WebChatJobs = jobs
	h.deps.WebChatMessageLimiter = limiter
	return h, conversations, sessions, jobs, limiter
}

func seedWebMessageConversation(t *testing.T, conversations *memory.ConversationRepo, accountID uuid.UUID, source domain.ConversationSource) *domain.Conversation {
	return seedWebMessageConversationWithStatus(t, conversations, accountID, source, domain.ConversationActive)
}

func seedWebMessageConversationWithStatus(t *testing.T, conversations *memory.ConversationRepo, accountID uuid.UUID, source domain.ConversationSource, status domain.ConversationStatus) *domain.Conversation {
	t.Helper()
	conversation := &domain.Conversation{
		AccountID:        accountID,
		Source:           source,
		ExternalThreadID: uuid.NewString(),
		Status:           status,
	}
	if err := conversations.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	return conversation
}

func safeWebConversationMessageRequest(t *testing.T, sessions *sessionStub, accountID, conversationID, idempotencyKey uuid.UUID, body string) *http.Request {
	t.Helper()
	req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/conversations/"+conversationID.String()+"/messages", sessions, accountID)
	req.Body = http.NoBody
	if body != "" {
		req.Body = io.NopCloser(strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.Header.Set("X-Idempotency-Key", idempotencyKey.String())
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	return req
}

func assertSafeQueuedWebChatResponse(t *testing.T, body []byte, jobID uuid.UUID) {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(fields) != 2 || fields["job_id"] == nil || fields["status"] == nil {
		t.Fatalf("response fields = %v, want only job_id/status", fields)
	}
	var gotID uuid.UUID
	var status domain.JobStatus
	if err := json.Unmarshal(fields["job_id"], &gotID); err != nil {
		t.Fatalf("decode job id: %v", err)
	}
	if err := json.Unmarshal(fields["status"], &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if gotID != jobID || status != domain.JobStatusQueued {
		t.Fatalf("response = job:%s status:%q", gotID, status)
	}
}
