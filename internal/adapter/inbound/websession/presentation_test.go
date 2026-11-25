package websession

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/resultservice"
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
	if input.ConversationTitleRequested {
		t.Fatal("presentation job must omit the separate chat title generation request")
	}
}

func TestPresentationPrefersGeminiLiteAndRejectsStaleQwenQuote(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	h, conversations, sessions, jobs, limiter := newWebConversationMessageTestHandler(t)
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	const lite, qwen = "gemini_2_5_flash_lite", "qwen_3_7_flash"
	h.deps.ImagePricing = prices
	h.cfg.Environment = "development"
	h.cfg.TextModels = textgeneration.Models([]string{providermodels.PublicTextGemini37Flash, providermodels.PublicTextGemini36Flash, qwen, lite}, prices)
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	routes := h.Routes()
	quote := func(wantModel string, wantCredits int64) {
		t.Helper()
		req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/presentations/quote", sessions, owner)
		req.Header.Set("X-Epifera-Release-View", "dev")
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)
		var out struct {
			Model   string `json:"model_id"`
			Credits int64  `json:"credits"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Model != wantModel || out.Credits <= 0 || wantCredits != 0 && out.Credits != wantCredits {
			t.Errorf("quote: status %d and body %s, want model %s / quoted stars %d", w.Code, w.Body, wantModel, wantCredits)
		}
	}
	quote(lite, 5)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	submit := func(model string, presentation bool) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"prompt":"Test","model_id":%q}`, model)
		if presentation {
			body = fmt.Sprintf(`{"prompt":"Test","model_id":%q,"expected_presentation_credits":5}`, model)
		}
		req := safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body)
		if presentation {
			req.URL.Path = "/web/v1/presentations/" + conversation.ID.String() + "/generate"
		}
		req.Header.Set("X-Epifera-Release-View", "dev")
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)
		return w
	}
	if w := submit(qwen, true); w.Code != http.StatusConflict || len(jobs.inputs) != 0 || limiter.calls != 0 {
		t.Fatalf("stale Qwen quote should conflict before any job or rate-limit use: %d %s, %d jobs, %d limits", w.Code, w.Body, len(jobs.inputs), limiter.calls)
	}
	if w := submit(lite, true); w.Code != http.StatusCreated || len(jobs.inputs) != 1 || jobs.inputs[0].PricingSnapshot.Key.TextModelID != lite || jobs.inputs[0].PricingSnapshot.InternalCredits != 5 {
		t.Fatalf("valid Gemini quote did not queue the exact five-star presentation: %d %s", w.Code, w.Body)
	}
	if w := submit(qwen, false); w.Code != http.StatusCreated || len(jobs.inputs) != 2 || jobs.inputs[1].PricingSnapshot.Key.TextModelID != qwen || !strings.HasPrefix(jobs.inputs[1].IdempotencyKey, "web-chat:") {
		t.Fatalf("ordinary Qwen chat changed: %d %s", w.Code, w.Body)
	}
	h.cfg.TextModels = textgeneration.Models([]string{providermodels.PublicTextGemini36Flash, qwen}, prices)
	quote(providermodels.PublicTextGemini36Flash, 0)
}

func TestReadPresentationByOwnedJobAndRecoverByConversation(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	h.cfg.Environment = "development"
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	otherConversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobID := uuid.New()
	prompt := "A presentation about the library"
	key := "web-presentation:" + owner.String() + ":" + uuid.NewString()
	params, err := json.Marshal(webChatJobParams{
		Prompt: prompt, ModelID: "qwen_3_7_flash", ModelName: "Presentation model",
		ConversationID: conversation.ID.String(), ConversationSource: domain.ConversationSourceWeb,
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
	read := func(account uuid.UUID, location, release string) *httptest.ResponseRecorder {
		req := authenticatedConversationRequest(t, http.MethodGet, location, sessions, account)
		req.Header.Set("X-Epifera-Release-View", release)
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, req)
		return w
	}
	readOK := func(location string) (status domain.JobStatus, gotPrompt, answer, failure string) {
		t.Helper()
		w := read(owner, location, "dev")
		var out struct {
			JobID       uuid.UUID        `json:"job_id"`
			Status      domain.JobStatus `json:"status"`
			Prompt      string           `json:"prompt"`
			Answer      string           `json:"answer"`
			ModelName   string           `json:"model_name"`
			FailureCode string           `json:"failure_code"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil || out.JobID != jobID || out.ModelName != "Presentation model" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected presentation response: %d %s", w.Code, w.Body)
		}
		return out.Status, out.Prompt, out.Answer, out.FailureCode
	}
	if status, gotPrompt, answer, failure := readOK(base + "?job_id=" + jobID.String()); status != domain.JobStatusQueued || gotPrompt != prompt || answer != "" || failure != "" {
		t.Fatalf("queued response = %s, %q, %q, %q", status, gotPrompt, answer, failure)
	}
	h.deps.Conversations = redactedPresentationMessage{conversations, domain.ConversationRoleUser}
	if _, gotPrompt, _, _ := readOK(base + "?job_id=" + jobID.String()); gotPrompt != "" {
		t.Fatal("redacted prompts must not be recovered from job parameters")
	}
	h.deps.Conversations = conversations
	addMessage := func(conversationID, messageJobID uuid.UUID, role domain.ConversationMessageRole, text string) {
		t.Helper()
		if _, err := conversations.UpsertMessage(context.Background(), &domain.ConversationMessage{ConversationID: conversationID, JobID: messageJobID, Role: role, Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	addMessage(conversation.ID, jobID, domain.ConversationRoleUser, prompt)
	addMessage(otherConversation.ID, uuid.New(), domain.ConversationRoleUser, "Another conversation's sequence number")
	addMessage(conversation.ID, uuid.New(), domain.ConversationRoleAssistant, "A different job's response in the same conversation")
	job.Status = domain.JobStatusSucceeded
	if status, gotPrompt, answer, failure := readOK(base); status != domain.JobStatusSucceeded || gotPrompt != prompt || answer != "" || failure != "answer_missing" {
		t.Fatalf("missing result response = %s, %q, %q, %q", status, gotPrompt, answer, failure)
	}
	const deckAnswer = "## The local library\n- Open every day."
	artifactID := uuid.New()
	job.OutputArtifactIDs = []uuid.UUID{artifactID}
	metadata := resultservice.ArtifactMetadata{ID: artifactID, MediaType: domain.MediaTypeText, MIMEType: "text/plain; charset=utf-8", SizeBytes: int64(len(deckAnswer))}
	outputs := &imageResultReaderStub{result: resultservice.Result{
		ID: jobID, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText,
		Status: domain.JobStatusSucceeded, Artifacts: []resultservice.ArtifactMetadata{metadata},
	}}
	h.deps.ImageResults = outputs
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: &domain.Artifact{
		ID: artifactID, JobID: &jobID, OwnerAccountID: owner, Status: domain.ArtifactStatusReady,
		Kind: domain.ArtifactKindOutput, MediaType: metadata.MediaType, MimeType: metadata.MIMEType,
		SizeBytes: metadata.SizeBytes, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(deckAnswer))), StorageBucket: "fixture", StorageKey: "deck.txt",
	}}
	objects := &boundedPresentationStore{data: []byte(deckAnswer)}
	h.deps.ImageArtifactURLSigner = objects
	if _, _, answer, failure := readOK(base); answer != deckAnswer || failure != "" || objects.calls != 1 || objects.limit != int64(len(deckAnswer)) {
		t.Fatalf("moderated artifact recovery = %q, %q, %d reads at %d bytes", answer, failure, objects.calls, objects.limit)
	}
	outputs.result.Artifacts[0].Deleted = true
	if _, _, answer, failure := readOK(base); answer != "" || failure != "answer_missing" || objects.calls != 1 {
		t.Fatal("deleted artifacts must not be recovered")
	}
	outputs.result.Artifacts[0].Deleted = false
	h.deps.Conversations = redactedPresentationMessage{conversations, domain.ConversationRoleAssistant}
	if _, _, answer, failure := readOK(base); answer != "" || failure != "answer_missing" || objects.calls != 1 {
		t.Fatal("previously recorded but redacted assistant answers must not be restored")
	}
	h.deps.Conversations = conversations

	addMessage(conversation.ID, jobID, domain.ConversationRoleAssistant, deckAnswer)
	if status, gotPrompt, answer, failure := readOK(base); status != domain.JobStatusSucceeded || gotPrompt != prompt || answer != deckAnswer || failure != "" {
		t.Fatalf("recovered response = %s, %q, %q, %q", status, gotPrompt, answer, failure)
	}
	job.Status = domain.JobStatusFailedTerminal
	job.ErrorCode = string(domain.ProviderErrTimeout)
	job.ErrorMessage = "private provider failure detail"
	if status, _, answer, failure := readOK(base); status != domain.JobStatusFailedTerminal || answer != "" || failure != "provider_unavailable" {
		t.Fatalf("failed response = %s, %q, %q", status, answer, failure)
	}
	if w := read(owner, base, "dev"); strings.Contains(w.Body.String(), job.ErrorMessage) {
		t.Fatal("provider details escaped the safe projection")
	}
	for _, request := range []struct {
		account           uuid.UUID
		location, release string
	}{
		{uuid.New(), base, "dev"},
		{owner, base, "prod"},
		{owner, "/web/v1/presentations/" + otherConversation.ID.String() + "?job_id=" + jobID.String(), "dev"},
		{owner, base + "?job_id=" + uuid.NewString(), "dev"},
	} {
		if w := read(request.account, request.location, request.release); w.Code != http.StatusNotFound {
			t.Fatalf("unrelated presentation returned %d %s", w.Code, w.Body)
		}
	}
	if w := read(owner, base+"?job_id=invalid", "dev"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid job ID returned %d", w.Code)
	}
	job.IdempotencyKey = "web-chat:" + owner.String() + ":" + uuid.NewString()
	if w := read(owner, base, "dev"); w.Code != http.StatusNotFound {
		t.Fatalf("ordinary chat returned %d %s", w.Code, w.Body)
	}
}

type redactedPresentationMessage struct {
	domain.ConversationRepository
	role domain.ConversationMessageRole
}

func (r redactedPresentationMessage) HasRecordedMessageByJobID(ctx context.Context, jobID uuid.UUID, role domain.ConversationMessageRole) (bool, error) {
	if role == r.role {
		return true, nil
	}
	return r.ConversationRepository.HasRecordedMessageByJobID(ctx, jobID, role)
}

type boundedPresentationStore struct {
	imageArtifactURLSignerStub
	data  []byte
	calls int
	limit int64
}

func (s *boundedPresentationStore) GetObjectBounded(_ context.Context, _, _ string, limit int64) ([]byte, error) {
	s.calls++
	s.limit = limit
	return s.data, nil
}
