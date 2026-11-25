package websession

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

var presentationTextModels = []string{
	pricingcatalog.PresentationPreferredTextModelID,
	providermodels.PublicTextGemini37Flash,
	providermodels.PublicTextGemini36Flash,
}

const legacyPresentationQwenModelID = "qwen_3_7_flash"

func presentationTextModel(id string) bool {
	return slices.Contains(presentationTextModels, id)
}

func (h *Handler) requirePresentationPreview(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if h.releaseProfile(r) == "dev" {
		return true
	}
	writeError(w, http.StatusNotFound, "presentation unavailable")
	return false
}

func (h *Handler) quotePresentation(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	for _, id := range presentationTextModels {
		if !h.releaseModelAllowed(r, id, "text") {
			continue
		}
		model, quote, err := textgeneration.Resolve(id, "presentation pricing", h.cfg.TextModels, h.deps.ImagePricing)
		if err != nil {
			continue
		}
		quote, err = pricingcatalog.PresentationTextSnapshot(quote)
		if err != nil {
			continue
		}
		writeJSON(w, http.StatusOK, struct {
			ID      string `json:"model_id"`
			Name    string `json:"model_name"`
			Credits int64  `json:"credits"`
		}{model.ModelID, model.ModelName, quote.InternalCredits})
		return
	}
	writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
}

func (h *Handler) generatePresentation(w http.ResponseWriter, r *http.Request) {
	if h.requirePresentationPreview(w, r) {
		h.createTextOrMediaMessage(w, r, true)
	}
}

type safePresentation struct {
	JobID       uuid.UUID        `json:"job_id"`
	Status      domain.JobStatus `json:"status"`
	Prompt      string           `json:"prompt"`
	Answer      string           `json:"answer,omitempty"`
	ModelName   string           `json:"model_name,omitempty"`
	FailureCode string           `json:"failure_code,omitempty"`
}

func (h *Handler) getPresentation(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	conversationID, err := uuid.Parse(r.PathValue("conversationID"))
	if err != nil || conversationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid presentation id")
		return
	}
	jobID := uuid.Nil
	if ids, supplied := r.URL.Query()["job_id"]; supplied {
		if len(ids) == 1 {
			jobID, _ = uuid.Parse(ids[0])
		}
		if jobID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid presentation job id")
			return
		}
	}
	if h.deps.Conversations == nil || h.deps.ImageJobReader == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
		return
	}
	ctx := r.Context()
	conversation, err := h.deps.Conversations.GetByIDForAccount(ctx, principal.AccountID, conversationID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
		return
	}
	if err != nil || conversation == nil || conversation.ID != conversationID || conversation.AccountID != principal.AccountID || conversation.Source != domain.ConversationSourceWeb || conversation.Status != domain.ConversationActive {
		writeError(w, http.StatusNotFound, "presentation not found")
		return
	}

	var input *domain.ConversationMessage
	if jobID == uuid.Nil {
		input, err = h.deps.Conversations.GetFirstUserMessage(ctx, conversationID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
			return
		}
		if err != nil || input == nil || input.JobID == uuid.Nil || !presentationMessageMatches(input, conversationID, input.JobID, domain.ConversationRoleUser) {
			writeError(w, http.StatusNotFound, "presentation not found")
			return
		}
		jobID = input.JobID
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(ctx, principal.AccountID, jobID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
		return
	}
	if err != nil || !ownedConversationJob(job, principal.AccountID, conversationID) || job.ID != jobID || !job.IsWebPresentation() {
		writeError(w, http.StatusNotFound, "presentation not found")
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if input == nil {
		input, err = h.deps.Conversations.GetUserMessageByJobID(ctx, jobID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
			return
		}
		if err != nil {
			input = nil
		}
		if input != nil && !presentationMessageMatches(input, conversationID, jobID, domain.ConversationRoleUser) {
			writeError(w, http.StatusNotFound, "presentation not found")
			return
		}
	}
	var params webChatJobParams
	if !job.Status.Valid() || json.Unmarshal(job.Params, &params) != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
		return
	}
	out := safePresentation{JobID: job.ID, Status: job.Status, ModelName: params.ModelName, FailureCode: presentationFailureCode(job)}
	if input != nil {
		out.Prompt = input.Text
	} else {
		// The worker writes the original message before calling the provider. A
		// queued job can still report its input, but later reads respect retention.
		switch job.Status {
		case domain.JobStatusReceived, domain.JobStatusValidated, domain.JobStatusCreditsReserved, domain.JobStatusQueued, domain.JobStatusDispatchingProvider:
			recorded, err := h.deps.Conversations.HasRecordedMessageByJobID(ctx, jobID, domain.ConversationRoleUser)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
				return
			}
			if !recorded {
				out.Prompt = params.Prompt
			}
		}
	}
	if job.Status == domain.JobStatusSucceeded && input != nil {
		answer, err := h.deps.Conversations.GetAssistantMessageByJobID(ctx, jobID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
			return
		}
		if err == nil && presentationMessageMatches(answer, conversationID, jobID, domain.ConversationRoleAssistant) && strings.TrimSpace(answer.Text) != "" {
			out.Answer = answer.Text
		}
		if errors.Is(err, domain.ErrNotFound) {
			out.Answer, err = h.readPresentationArtifact(ctx, job)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
				return
			}
		}
	}
	if job.Status == domain.JobStatusSucceeded && out.Answer == "" {
		out.FailureCode = "answer_missing"
	}
	writeJSON(w, http.StatusOK, out)
}

func presentationMessageMatches(message *domain.ConversationMessage, conversationID, jobID uuid.UUID, role domain.ConversationMessageRole) bool {
	return message != nil && message.ConversationID == conversationID && message.JobID == jobID && message.Role == role
}

func presentationFailureCode(job *domain.Job) string {
	switch job.Status {
	case domain.JobStatusFailedTerminal, domain.JobStatusRejected, domain.JobStatusRefunded:
	default:
		return ""
	}
	switch domain.ProviderErrorClass(job.ErrorCode) {
	case domain.ProviderErrSubmitIndeterminate:
		return "submission_unconfirmed"
	case domain.ProviderErrOutputDownloadFailed:
		return "result_storage_failed"
	case domain.ProviderErrContentRejected:
		return "content_rejected"
	case domain.ProviderErrInvalidRequest, domain.ProviderErrUnsupportedCapab:
		return "invalid_request"
	case domain.ProviderErrRateLimited, domain.ProviderErrOverloaded, domain.ProviderErrTimeout, domain.ProviderErrModelUnavailable, domain.ProviderErrAuthFailed, domain.ProviderErrInsufficientBalance:
		return "provider_unavailable"
	default:
		return "generation_failed"
	}
}
