package websession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/presentationintake"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/textgeneration"
)

const presentationConversationKind = "presentation"

// PresentationChatTurnReader includes accepted turns not yet present in history.
type PresentationChatTurnReader interface {
	GetLatestPresentationChatTurn(context.Context, uuid.UUID, uuid.UUID) (*domain.Job, error)
}

func publicConversationKind(conversation *domain.Conversation) string {
	if conversation.IsWebPresentationChat() {
		return presentationConversationKind
	}
	return ""
}

type safePresentationChatQuote struct {
	ModelID     string `json:"model_id"`
	ModelName   string `json:"model_name"`
	Credits     int64  `json:"credits"`
	TurnCredits int64  `json:"turn_credits"`
}

type presentationChatTurnRequest struct {
	Prompt                      string `json:"prompt"`
	ModelID                     string `json:"model_id"`
	ExpectedPresentationCredits int64  `json:"expected_presentation_credits"`
	ExpectedTurnCredits         int64  `json:"expected_turn_credits"`
}

type presentationChatGenerateRequest struct {
	AcceptUpdatedQuote          bool   `json:"accept_updated_quote,omitempty"`
	ModelID                     string `json:"model_id"`
	ExpectedPresentationCredits int64  `json:"expected_presentation_credits"`
}

type safePresentationChatDeck struct {
	ConversationID      uuid.UUID        `json:"conversation_id"`
	JobID               uuid.UUID        `json:"job_id"`
	Status              domain.JobStatus `json:"status"`
	ModelID             string           `json:"model_id"`
	PresentationCredits int64            `json:"presentation_credits"`
	Unavailable         bool             `json:"unavailable,omitempty"`
}

type safePresentationChatTurn struct {
	JobID               uuid.UUID                 `json:"job_id"`
	Status              domain.JobStatus          `json:"status"`
	ModelID             string                    `json:"model_id"`
	PresentationCredits int64                     `json:"presentation_credits"`
	TurnCredits         int64                     `json:"turn_credits"`
	Reply               string                    `json:"reply,omitempty"`
	Brief               *presentationintake.Brief `json:"brief,omitempty"`
	Missing             *[]string                 `json:"missing,omitempty"`
	Presentation        *safePresentationChatDeck `json:"presentation,omitempty"`
}

func (h *Handler) presentationChatQuote(r *http.Request) (modelcatalog.Model, pricingcatalog.PricingSnapshot, pricingcatalog.PricingSnapshot, bool) {
	for _, id := range presentationTextModels {
		if !h.releaseModelAllowed(r, id, "text") {
			continue
		}
		model, turn, err := textgeneration.Resolve(id, "presentation chat pricing", h.cfg.TextModels, h.deps.ImagePricing)
		if err != nil || turn.InternalCredits <= 0 {
			continue
		}
		deck, err := pricingcatalog.PresentationTextSnapshot(turn)
		if err == nil && deck.InternalCredits > 0 {
			return model, turn, deck, true
		}
	}
	return modelcatalog.Model{}, pricingcatalog.PricingSnapshot{}, pricingcatalog.PricingSnapshot{}, false
}

func (h *Handler) quotePresentationChat(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	model, turn, deck, ok := h.presentationChatQuote(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	writeJSON(w, http.StatusOK, safePresentationChatQuote{
		ModelID: model.ModelID, ModelName: model.ModelName, Credits: deck.InternalCredits, TurnCredits: turn.InternalCredits,
	})
}

func (h *Handler) createPresentationChat(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	owner, _ := PrincipalFromContext(r.Context())
	key, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || key == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid idempotency key")
		return
	}
	conversation, created, err := h.ensurePresentationConversation(r.Context(), owner.AccountID, domain.WebPresentationChatReference(key), "Epifera Презентации")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, newSafeConversation(*conversation))
}

func (h *Handler) ensurePresentationConversation(ctx context.Context, accountID uuid.UUID, externalID, title string) (*domain.Conversation, bool, error) {
	if h.deps.Conversations == nil {
		return nil, false, errors.New("presentations: no conversation repository")
	}
	conversation := &domain.Conversation{
		AccountID: accountID, Source: domain.ConversationSourceWeb, ExternalThreadID: externalID,
		Status: domain.ConversationActive, Title: title,
	}
	err := h.deps.Conversations.CreateConversation(ctx, conversation)
	created := err == nil
	if errors.Is(err, domain.ErrConflict) {
		conversation, err = h.deps.Conversations.GetActiveByReference(ctx, domain.ConversationRef{
			AccountID: accountID, Source: domain.ConversationSourceWeb, ExternalThreadID: externalID,
		})
	}
	if err != nil {
		return nil, false, err
	}
	if conversation == nil || conversation.ID == uuid.Nil || conversation.AccountID != accountID || conversation.Source != domain.ConversationSourceWeb ||
		conversation.Status != domain.ConversationActive || conversation.ExternalThreadID != externalID {
		return nil, false, domain.ErrNotFound
	}
	return conversation, created, nil
}

func (h *Handler) ownedPresentationChat(w http.ResponseWriter, r *http.Request) *domain.Conversation {
	if !h.requirePresentationPreview(w, r) {
		return nil
	}
	principal, _ := PrincipalFromContext(r.Context())
	conversationID, err := uuid.Parse(r.PathValue("conversationID"))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid presentation chat id")
		return nil
	}
	if h.deps.Conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return nil
	}
	conversation, err := h.deps.Conversations.GetByIDForAccount(r.Context(), principal.AccountID, conversationID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return nil
	}
	if err != nil || !conversation.IsWebPresentationChat() || conversation.ID != conversationID || conversation.AccountID != principal.AccountID || conversation.Status != domain.ConversationActive {
		writeError(w, http.StatusNotFound, "presentation chat not found")
		return nil
	}
	return conversation
}

func (h *Handler) createPresentationChatTurn(w http.ResponseWriter, r *http.Request) {
	chat := h.ownedPresentationChat(w, r)
	if chat == nil {
		return
	}
	var req presentationChatTurnRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	key, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || key == uuid.Nil || !dialogcontext.ValidPresentationIntakePrompt(req.Prompt) ||
		req.ModelID == "" || req.ExpectedTurnCredits <= 0 || req.ExpectedPresentationCredits <= 0 {
		writeError(w, http.StatusBadRequest, "invalid presentation chat message")
		return
	}
	if h.deps.WebChatJobs == nil || h.deps.WebChatMessageLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	ctx := r.Context()
	idempotencyKey := domain.WebPresentationIntakeJobKeyPrefix(chat.AccountID) + key.String()
	if h.deps.ImageJobIdempotency != nil {
		prior, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(ctx, chat.AccountID, idempotencyKey)
		if err == nil {
			params, credits, valid := intakeTurnParams(chat, prior)
			if !valid || params.Prompt != req.Prompt || params.ModelID != req.ModelID || credits != req.ExpectedTurnCredits || params.PresentationCredits != req.ExpectedPresentationCredits {
				writeError(w, http.StatusConflict, "idempotency key belongs to another message")
				return
			}
			if prior.Status == domain.JobStatusAwaitingPayment {
				if !h.allowPresentationChatMessage(w, r, chat.AccountID) {
					return
				}
				prior, err = h.resumePresentationChatJob(ctx, chat.AccountID, prior.ID)
				if writePresentationChatJobError(w, err) {
					return
				}
			}
			writePersistedWebChatJob(w, prior)
			return
		}
		if !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
			return
		}
	}
	model, turn, deck, available := h.presentationChatQuote(r)
	if !available {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	if req.ModelID != model.ModelID || req.ExpectedTurnCredits != turn.InternalCredits || req.ExpectedPresentationCredits != deck.InternalCredits {
		writeError(w, http.StatusConflict, "presentation quote changed")
		return
	}
	if !h.allowPresentationChatMessage(w, r, chat.AccountID) {
		return
	}
	params := newPresentationChatJobParams(chat.ID, req.Prompt, model)
	params.PresentationCredits = deck.InternalCredits
	job, err := h.queuePresentationChatJob(ctx, chat.AccountID, idempotencyKey, params, turn)
	if !writePresentationChatJobError(w, err) {
		writePersistedWebChatJob(w, job)
	}
}

func (h *Handler) allowPresentationChatMessage(w http.ResponseWriter, r *http.Request, accountID uuid.UUID) bool {
	if h.deps.WebChatMessageLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return false
	}
	allowed, err := h.deps.WebChatMessageLimiter.Allow(r.Context(), "account:"+accountID.String())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return false
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "chat message rate limited")
	}
	return allowed
}

func newPresentationChatJobParams(conversationID uuid.UUID, prompt string, model modelcatalog.Model) webChatJobParams {
	return webChatJobParams{
		Prompt: prompt, ModelID: model.ModelID, ModelName: model.ModelName, Provider: model.Provider, ModelCode: model.ModelCode,
		ConversationID: conversationID.String(), ConversationSource: domain.ConversationSourceWeb,
	}
}

func (h *Handler) queuePresentationChatJob(ctx context.Context, accountID uuid.UUID, key string, params webChatJobParams, price pricingcatalog.PricingSnapshot) (*domain.Job, error) {
	if h.deps.WebChatJobs == nil {
		return nil, errors.New("presentations: no job creator")
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	job, err := h.deps.WebChatJobs.CreateJob(ctx, joborchestrator.CreateJobInput{
		AccountID: accountID, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode: domain.ResultModeAccountHistory, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText,
		IdempotencyKey: key, CorrelationID: key, Params: raw, PricingSnapshot: price,
	})
	if errors.Is(err, domain.ErrConflict) && h.deps.ImageJobIdempotency != nil {
		// A concurrent turn or finalize can pass the first lookup before the
		// winning request commits its unique key. Return only an exact replay.
		prior, lookupErr := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(ctx, accountID, key)
		if lookupErr == nil && validPersistedWebChatJob(prior, accountID, key, params) {
			if credits, ok := prior.PricingSnapshotCredits(); ok && credits == price.InternalCredits {
				return prior, nil
			}
		} else if lookupErr != nil && !errors.Is(lookupErr, domain.ErrNotFound) {
			return nil, lookupErr
		}
	}
	if err != nil {
		return nil, err
	}
	if !validPersistedWebChatJob(job, accountID, key, params) {
		return nil, errors.New("presentations: unexpected persisted job")
	}
	return job, nil
}

func (h *Handler) resumePresentationChatJob(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	if h.deps.ImageJobs == nil {
		return nil, errors.New("presentations: no job activator")
	}
	job, err := h.deps.ImageJobs.ActivatePreparedAccountJob(ctx, accountID, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil || job.ID != jobID || job.AccountID != accountID || !job.Status.Valid() || !(job.IsWebPresentation() || job.IsWebPresentationIntake()) {
		return nil, domain.ErrInvalidInput
	}
	return job, nil
}

func writePresentationChatJobError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "idempotency key belongs to another message")
	case errors.Is(err, domain.ErrActiveJobLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "chat message rate limited")
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeError(w, http.StatusPaymentRequired, "insufficient credits")
	case errors.Is(err, domain.ErrCostCapExceeded):
		writeError(w, http.StatusBadRequest, "invalid presentation chat message")
	default:
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
	}
	return true
}

func intakeTurnParams(chat *domain.Conversation, job *domain.Job) (webChatJobParams, int64, bool) {
	var params webChatJobParams
	if !job.IsWebPresentationIntake() || !ownedConversationJob(job, chat.AccountID, chat.ID) || !job.Status.Valid() ||
		json.Unmarshal(job.Params, &params) != nil || !validPersistedWebChatJob(job, chat.AccountID, job.IdempotencyKey, params) ||
		params.PresentationCredits <= 0 || !presentationTextModel(params.ModelID) {
		return webChatJobParams{}, 0, false
	}
	credits, ok := job.PricingSnapshotCredits()
	return params, credits, ok && credits > 0
}

func (h *Handler) getPresentationChat(w http.ResponseWriter, r *http.Request) {
	chat := h.ownedPresentationChat(w, r)
	if chat == nil {
		return
	}
	if h.deps.PresentationChatTurns == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	job, err := h.deps.PresentationChatTurns.GetLatestPresentationChatTurn(r.Context(), chat.AccountID, chat.ID)
	if errors.Is(err, domain.ErrNotFound) {
		writeJSON(w, http.StatusOK, struct{}{})
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	h.writePresentationChatTurn(w, r, chat, job)
}

func (h *Handler) requestedPresentationChatTurn(w http.ResponseWriter, r *http.Request, chat *domain.Conversation) *domain.Job {
	jobID, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid presentation chat turn id")
		return nil
	}
	if h.deps.ImageJobReader == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return nil
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), chat.AccountID, jobID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return nil
	}
	if !job.IsWebPresentationIntake() || job.ID != jobID || !ownedConversationJob(job, chat.AccountID, chat.ID) {
		writeError(w, http.StatusNotFound, "presentation chat turn not found")
		return nil
	}
	return job
}

func (h *Handler) getPresentationChatTurn(w http.ResponseWriter, r *http.Request) {
	chat := h.ownedPresentationChat(w, r)
	if chat != nil {
		if job := h.requestedPresentationChatTurn(w, r, chat); job != nil {
			h.writePresentationChatTurn(w, r, chat, job)
		}
	}
}

func (h *Handler) resumePresentationChatTurn(w http.ResponseWriter, r *http.Request) {
	chat := h.ownedPresentationChat(w, r)
	if chat == nil {
		return
	}
	job := h.requestedPresentationChatTurn(w, r, chat)
	if job == nil {
		return
	}
	if _, _, ok := intakeTurnParams(chat, job); !ok {
		writeError(w, http.StatusConflict, "presentation chat turn unavailable")
		return
	}
	if job.Status == domain.JobStatusAwaitingPayment {
		if h.deps.PresentationChatTurns == nil {
			writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
			return
		}
		latest, err := h.deps.PresentationChatTurns.GetLatestPresentationChatTurn(r.Context(), chat.AccountID, chat.ID)
		if err != nil || latest == nil || latest.ID != job.ID {
			writeError(w, http.StatusConflict, "presentation brief changed")
			return
		}
		if !h.allowPresentationChatMessage(w, r, chat.AccountID) {
			return
		}
		job, err = h.resumePresentationChatJob(r.Context(), chat.AccountID, job.ID)
		if writePresentationChatJobError(w, err) {
			return
		}
	}
	writePersistedWebChatJob(w, job)
}

func (h *Handler) buildVerifiedIntakeResult(ctx context.Context, chat *domain.Conversation, job *domain.Job) (presentationintake.Result, error) {
	if job.Status != domain.JobStatusSucceeded {
		return presentationintake.Result{}, domain.ErrNotFound
	}
	assistant, err := h.deps.Conversations.GetAssistantMessageByJobID(ctx, job.ID)
	if err != nil {
		return presentationintake.Result{}, err
	}
	if !presentationMessageMatches(assistant, chat.ID, job.ID, domain.ConversationRoleAssistant) {
		return presentationintake.Result{}, domain.ErrNotFound
	}
	evidence, _, err := dialogcontext.PresentationIntakeEvidence(ctx, h.deps.Conversations, chat.ID, job.ID)
	if err != nil {
		return presentationintake.Result{}, err
	}
	raw, err := h.readModeratedTextArtifact(ctx, job, false)
	if err != nil {
		return presentationintake.Result{}, err
	}
	if raw == "" {
		return presentationintake.Result{}, domain.ErrNotFound
	}
	result, err := presentationintake.Parse(raw, evidence)
	if err != nil || strings.TrimSpace(assistant.Text) == "" {
		return presentationintake.Result{}, domain.ErrInvalidInput
	}
	result.Reply = assistant.Text
	return result, nil
}

func (h *Handler) writePresentationChatTurn(w http.ResponseWriter, r *http.Request, chat *domain.Conversation, job *domain.Job) {
	params, credits, ok := intakeTurnParams(chat, job)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	out := safePresentationChatTurn{
		JobID: job.ID, Status: job.Status, ModelID: params.ModelID, PresentationCredits: params.PresentationCredits, TurnCredits: credits,
	}
	deck, err := h.findPresentationFromChat(r.Context(), chat)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	out.Presentation = deck
	if job.Status == domain.JobStatusSucceeded {
		result, err := h.buildVerifiedIntakeResult(r.Context(), chat, job)
		if err != nil && deck == nil {
			writeError(w, http.StatusServiceUnavailable, "presentation chat result unavailable")
			return
		}
		if err == nil {
			out.Reply, out.Brief = result.Reply, &result.Brief
			missing := append([]string{}, result.Brief.Missing()...)
			out.Missing = &missing
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func childPresentationKeys(chatID uuid.UUID) (string, uuid.UUID) {
	conversation := uuid.NewSHA1(uuid.NameSpaceURL, []byte("epifera:chat-presentation:conversation:"+chatID.String()))
	job := uuid.NewSHA1(uuid.NameSpaceURL, []byte("epifera:chat-presentation:job:"+chatID.String()))
	return "presentation-from-intake:" + conversation.String(), job
}

func (h *Handler) findPresentationFromChat(ctx context.Context, chat *domain.Conversation) (*safePresentationChatDeck, error) {
	if h.deps.ImageJobIdempotency == nil {
		return nil, errors.New("presentations: no idempotency reader")
	}
	conversationRef, jobKey := childPresentationKeys(chat.ID)
	job, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(ctx, chat.AccountID, domain.WebPresentationJobKeyPrefix(chat.AccountID)+jobKey.String())
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var params webChatJobParams
	if !job.IsWebPresentation() || !job.Status.Valid() || json.Unmarshal(job.Params, &params) != nil ||
		!validPersistedWebChatJob(job, chat.AccountID, job.IdempotencyKey, params) {
		return nil, domain.ErrInvalidInput
	}
	conversationID, err := uuid.Parse(params.ConversationID)
	credits, priced := job.PricingSnapshotCredits()
	if err != nil || conversationID == uuid.Nil || !priced || credits <= 0 || !ownedConversationJob(job, chat.AccountID, conversationID) {
		return nil, domain.ErrInvalidInput
	}
	result := &safePresentationChatDeck{ConversationID: conversationID, JobID: job.ID, Status: job.Status, ModelID: params.ModelID, PresentationCredits: credits}
	conversation, err := h.deps.Conversations.GetByIDForAccount(ctx, chat.AccountID, conversationID)
	if errors.Is(err, domain.ErrNotFound) {
		result.Unavailable = true
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.ID != conversationID || conversation.AccountID != chat.AccountID ||
		conversation.Source != domain.ConversationSourceWeb || conversation.ExternalThreadID != conversationRef {
		return nil, domain.ErrInvalidInput
	}
	result.Unavailable = conversation.Status != domain.ConversationActive
	return result, nil
}

func (h *Handler) generateFromPresentationChat(w http.ResponseWriter, r *http.Request) {
	chat := h.ownedPresentationChat(w, r)
	if chat == nil {
		return
	}
	var req presentationChatGenerateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resultJob := h.requestedPresentationChatTurn(w, r, chat)
	if resultJob == nil {
		return
	}
	params, _, valid := intakeTurnParams(chat, resultJob)
	frozenQuote := params.ModelID == req.ModelID && params.PresentationCredits == req.ExpectedPresentationCredits
	if !valid || resultJob.Status != domain.JobStatusSucceeded || req.ModelID == "" || req.ExpectedPresentationCredits <= 0 || !frozenQuote && !req.AcceptUpdatedQuote {
		writeError(w, http.StatusConflict, "presentation brief or quote changed")
		return
	}
	ctx := r.Context()
	existing, err := h.findPresentationFromChat(ctx, chat)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	if existing != nil {
		if existing.Status == domain.JobStatusAwaitingPayment && !existing.Unavailable {
			if existing.ModelID != req.ModelID || existing.PresentationCredits != req.ExpectedPresentationCredits {
				writeError(w, http.StatusConflict, "presentation quote changed")
				return
			}
			if !h.allowPresentationChatMessage(w, r, chat.AccountID) {
				return
			}
			resumed, err := h.resumePresentationChatJob(ctx, chat.AccountID, existing.JobID)
			if writePresentationChatJobError(w, err) {
				return
			}
			existing.Status = resumed.Status
		}
		writeJSON(w, http.StatusOK, existing)
		return
	}
	if h.deps.PresentationChatTurns == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	latest, err := h.deps.PresentationChatTurns.GetLatestPresentationChatTurn(ctx, chat.AccountID, chat.ID)
	if err != nil || latest == nil || latest.ID != resultJob.ID {
		writeError(w, http.StatusConflict, "presentation brief changed")
		return
	}
	result, err := h.buildVerifiedIntakeResult(ctx, chat, resultJob)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat result unavailable")
		return
	}
	prompt := result.Brief.PresentationPrompt()
	if prompt == "" || len(result.Brief.Missing()) != 0 {
		writeError(w, http.StatusConflict, "presentation brief incomplete")
		return
	}
	model, _, deckPrice, available := h.presentationChatQuote(r)
	if !available {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	if model.ModelID != req.ModelID || deckPrice.InternalCredits != req.ExpectedPresentationCredits {
		writeError(w, http.StatusConflict, "presentation quote changed")
		return
	}
	if _, _, err := textgeneration.Resolve(model.ModelID, prompt, h.cfg.TextModels, h.deps.ImagePricing); err != nil {
		writeError(w, http.StatusBadRequest, "invalid presentation brief")
		return
	}
	if !h.allowPresentationChatMessage(w, r, chat.AccountID) {
		return
	}
	conversationRef, jobKey := childPresentationKeys(chat.ID)
	conversation, _, err := h.ensurePresentationConversation(ctx, chat.AccountID, conversationRef, "")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation chat unavailable")
		return
	}
	job, err := h.queuePresentationChatJob(ctx, chat.AccountID, domain.WebPresentationJobKeyPrefix(chat.AccountID)+jobKey.String(),
		newPresentationChatJobParams(conversation.ID, prompt, model), deckPrice)
	if writePresentationChatJobError(w, err) {
		return
	}
	if job.Status == domain.JobStatusAwaitingPayment {
		writeError(w, http.StatusPaymentRequired, "insufficient credits")
		return
	}
	writeJSON(w, http.StatusCreated, safePresentationChatDeck{
		ConversationID: conversation.ID, JobID: job.ID, Status: job.Status,
		ModelID: model.ModelID, PresentationCredits: deckPrice.InternalCredits,
	})
}
