package websession

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/audiodownload"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

type audioDownloadJobParams struct {
	SourceURL string `json:"source_url"`
}

type safeAudioDownloadJob struct {
	ID         uuid.UUID        `json:"id"`
	Status     domain.JobStatus `json:"status"`
	ArtifactID *uuid.UUID       `json:"artifact_id,omitempty"`
	ErrorCode  string           `json:"error_code,omitempty"`
}

func (h *Handler) createAudioDownloadJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	var input struct {
		URL string `json:"url"`
	}
	if !decodeJSONWithLimit(w, r, &input, 8<<10) {
		return
	}
	sourceURL, err := audiodownload.NormalizeURL(input.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_download_url")
		return
	}
	key, err := uuid.Parse(r.Header.Get("X-Idempotency-Key"))
	if err != nil || key == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid idempotency key")
		return
	}
	if h.deps.WebChatJobs == nil || h.deps.ImageJobIdempotency == nil || h.deps.ImageJobPrepareLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}

	ctx := r.Context()
	accountID := principal.AccountID
	accountKey := "audio-download:" + accountID.String()
	jobKey := accountKey + ":" + key.String()
	job, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(ctx, accountID, jobKey)
	if err == nil {
		h.writeAudioDownloadReplay(w, r, job, accountID, jobKey, sourceURL)
		return
	}
	if !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}
	allowed, err := h.deps.ImageJobPrepareLimiter.Allow(ctx, accountKey)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "audio download rate limited")
		return
	}

	params, _ := json.Marshal(audioDownloadJobParams{SourceURL: sourceURL})
	job, err = h.deps.WebChatJobs.CreateJob(ctx, joborchestrator.CreateJobInput{
		AccountID:      accountID,
		Source:         "web",
		ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:     domain.ResultModeAccountHistory,
		Operation:      domain.OperationAudioDownload,
		Modality:       domain.ModalityAudio,
		IdempotencyKey: jobKey,
		CorrelationID:  "web-audio-download:" + key.String(),
		Params:         params,
	})
	switch {
	case errors.Is(err, domain.ErrConflict):
		job, err = h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(ctx, accountID, jobKey)
		if err != nil {
			writeError(w, http.StatusConflict, "audio download request conflict")
			return
		}
		h.writeAudioDownloadReplay(w, r, job, accountID, jobKey, sourceURL)
		return
	case errors.Is(err, domain.ErrActiveJobLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "audio download rate limited")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}
	if !sameAudioDownloadRequest(job, accountID, jobKey, sourceURL) {
		writeError(w, http.StatusConflict, "audio download request conflict")
		return
	}
	h.writeAudioDownloadJob(w, r, http.StatusCreated, job)
}

func (h *Handler) writeAudioDownloadReplay(w http.ResponseWriter, r *http.Request, job *domain.Job, owner uuid.UUID, jobKey, sourceURL string) {
	if !sameAudioDownloadRequest(job, owner, jobKey, sourceURL) {
		writeError(w, http.StatusConflict, "audio download request conflict")
		return
	}
	h.writeAudioDownloadJob(w, r, http.StatusOK, job)
}

func sameAudioDownloadRequest(job *domain.Job, owner uuid.UUID, jobKey, sourceURL string) bool {
	if !ownedAudioDownloadJob(job, owner) || job.IdempotencyKey != jobKey {
		return false
	}
	var params audioDownloadJobParams
	return json.Unmarshal(job.Params, &params) == nil && params.SourceURL == sourceURL
}

func ownedAudioDownloadJob(job *domain.Job, owner uuid.UUID) bool {
	return job != nil && job.ID != uuid.Nil && job.AccountID == owner && job.Source == "web" &&
		job.UserID == uuid.Nil && job.VKPeerID == 0 && job.CommandID == uuid.Nil && job.DeliveryTarget == nil &&
		job.ChannelContext != nil && job.ChannelContext.Channel == domain.ChannelWeb &&
		job.ChannelContext.RecipientRef == "" && job.ChannelContext.ThreadRef == "" &&
		job.ResultMode == domain.ResultModeAccountHistory && job.OperationType == domain.OperationAudioDownload &&
		job.Modality == domain.ModalityAudio && job.CostEstimate == 0 && job.CostReserved == 0 && job.CostCaptured == 0
}

func (h *Handler) getAudioDownloadJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || id == uuid.Nil {
		writeError(w, http.StatusNotFound, "audio download not found")
		return
	}
	if h.deps.ImageJobReader == nil {
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
		return
	}
	if err != nil || !ownedAudioDownloadJob(job, principal.AccountID) || job.ID != id {
		writeError(w, http.StatusNotFound, "audio download not found")
		return
	}
	h.writeAudioDownloadJob(w, r, http.StatusOK, job)
}

func (h *Handler) writeAudioDownloadJob(w http.ResponseWriter, r *http.Request, status int, job *domain.Job) {
	out := safeAudioDownloadJob{ID: job.ID, Status: job.Status}
	switch job.Status {
	case domain.JobStatusSucceeded:
		if h.deps.ImageResults == nil {
			writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
			return
		}
		result, err := h.deps.ImageResults.GetResult(r.Context(), job.AccountID, job.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "audio download unavailable")
			return
		}
		if err == nil && result.ID == job.ID && len(result.Artifacts) == 1 && len(job.OutputArtifactIDs) == 1 {
			artifact := result.Artifacts[0]
			if !artifact.Deleted && artifact.ID == job.OutputArtifactIDs[0] && artifact.ID != uuid.Nil &&
				artifact.MediaType == domain.MediaTypeAudio && artifact.MIMEType == "audio/mpeg" {
				out.ArtifactID = &artifact.ID
			}
		}
		if out.ArtifactID == nil {
			out.ErrorCode = "artifact_unavailable"
		}
	case domain.JobStatusFailedTerminal, domain.JobStatusRejected:
		switch job.ErrorCode {
		case "download_failed", "download_unavailable", "download_too_large", "download_timeout", "content_rejected":
			out.ErrorCode = job.ErrorCode
		default:
			out.ErrorCode = "download_failed"
		}
	}
	writeJSON(w, status, out)
}

func (h *Handler) getAudioDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	h.serveGeneratedArtifact(w, r, func(job *domain.Job, owner uuid.UUID) bool {
		return ownedAudioDownloadJob(job, owner) && job.Status == domain.JobStatusSucceeded && len(job.OutputArtifactIDs) == 1
	}, func(a *domain.Artifact) bool {
		return a.MediaType == domain.MediaTypeAudio && a.MimeType == "audio/mpeg"
	}, "attachment")
}
