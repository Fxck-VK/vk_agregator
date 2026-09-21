package websession

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"reflect"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/speechgeneration"
)

type safeSpeechJob struct {
	ID           uuid.UUID        `json:"id"`
	ModelID      string           `json:"model_id"`
	Status       domain.JobStatus `json:"status"`
	CostEstimate int64            `json:"cost_estimate"`
	CreatedAt    time.Time        `json:"created_at"`
}

func speechJobDTO(job *domain.Job, account uuid.UUID) (safeSpeechJob, bool) {
	if job == nil || job.AccountID != account || job.Source != "web" || job.UserID != uuid.Nil || job.VKPeerID != 0 || job.ResultMode != domain.ResultModeAccountHistory || job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" || job.DeliveryTarget != nil {
		return safeSpeechJob{}, false
	}
	p, err := speechgeneration.DecodeJob(job)
	if err != nil {
		return safeSpeechJob{}, false
	}
	return safeSpeechJob{job.ID, p.ModelID, job.Status, job.CostEstimate, job.CreatedAt}, true
}
func (h *Handler) prepareSpeechJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	var request speechgeneration.Request
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Validate() != nil {
		writeError(w, 400, "invalid speech request")
		return
	}
	key, err := uuid.Parse(r.Header.Get("X-Idempotency-Key"))
	if err != nil || key == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	if h.deps.ImageJobs == nil || h.deps.ImageBalance == nil || h.deps.ImageJobIdempotency == nil || h.deps.ImageJobPrepareLimiter == nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	if job, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, key.String()); err == nil {
		if !speechReplayMatches(job, principal.AccountID, key.String(), request) {
			writeError(w, 409, "speech preparation conflict")
			return
		}
		h.writeSpeechPreparation(w, r, job, principal.AccountID)
		return
	} else if !errors.Is(err, domain.ErrNotFound) {
		writeError(w, 503, "speech unavailable")
		return
	}
	params, price, err := speechgeneration.Resolve(request, providermodels.StaticRegistry(), h.deps.ImagePricing)
	if err != nil {
		writeError(w, 503, "speech awaits verification and pricing")
		return
	}
	if !h.validateSpeechInput(r, principal.AccountID, request) {
		writeError(w, 400, "invalid speech input")
		return
	}
	allowed, err := h.deps.ImageJobPrepareLimiter.Allow(r.Context(), "speech:"+principal.AccountID.String())
	if err != nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	if !allowed {
		writeError(w, 429, "speech preparation rate limited")
		return
	}
	raw, err := json.Marshal(params)
	if err != nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	inputs := []uuid.UUID{}
	if request.AudioArtifactID != uuid.Nil {
		inputs = append(inputs, request.AudioArtifactID)
	}
	job, err := h.deps.ImageJobs.PrepareAccountJob(r.Context(), joborchestrator.PrepareAccountJobInput{AccountID: principal.AccountID, Operation: request.Operation(), Modality: request.Modality(), IdempotencyKey: key.String(), CorrelationID: "web-speech:" + key.String(), Params: raw, PricingSnapshot: price, CostEstimateCredits: price.InternalCredits, InputArtifactIDs: inputs})
	if errors.Is(err, domain.ErrConflict) {
		job, err = h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, key.String())
		if err != nil || !speechReplayMatches(job, principal.AccountID, key.String(), request) {
			writeError(w, 409, "speech preparation conflict")
			return
		}
	}
	if errors.Is(err, domain.ErrPreparedJobLimitExceeded) {
		writeError(w, 429, "speech preparation rate limited")
		return
	}
	if err != nil || !speechReplayMatches(job, principal.AccountID, key.String(), request) {
		writeError(w, 503, "speech unavailable")
		return
	}
	h.writeSpeechPreparation(w, r, job, principal.AccountID)
}
func speechReplayMatches(job *domain.Job, account uuid.UUID, key string, request speechgeneration.Request) bool {
	if _, ok := speechJobDTO(job, account); !ok || job.IdempotencyKey != key {
		return false
	}
	p, err := speechgeneration.DecodeJob(job)
	return err == nil && reflect.DeepEqual(p.Request, request)
}
func (h *Handler) writeSpeechPreparation(w http.ResponseWriter, r *http.Request, job *domain.Job, account uuid.UUID) {
	dto, ok := speechJobDTO(job, account)
	if !ok {
		writeError(w, 503, "speech unavailable")
		return
	}
	balance, err := h.deps.ImageBalance.BalanceForEstimate(r.Context(), account)
	if err != nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	writeJSON(w, 201, struct {
		Job       safeSpeechJob `json:"job"`
		Balance   int64         `json:"balance"`
		CanAfford bool          `json:"can_afford"`
	}{dto, balance, balance >= dto.CostEstimate})
}
func (h *Handler) validateSpeechInput(r *http.Request, account uuid.UUID, request speechgeneration.Request) bool {
	if request.AudioArtifactID == uuid.Nil {
		return true
	}
	if h.deps.ImageArtifacts == nil {
		return false
	}
	a, err := h.deps.ImageArtifacts.GetByIDForAccount(r.Context(), account, request.AudioArtifactID)
	return err == nil && mediaprobe.ValidateMusicInputArtifact(a, account) == nil && (a.MimeType == "audio/mpeg" || a.MimeType == "audio/wav" || a.MimeType == "audio/x-wav")
}
func (h *Handler) loadSpeechJob(w http.ResponseWriter, r *http.Request) (*domain.Job, uuid.UUID, bool) {
	w.Header().Set("Cache-Control", "no-store")
	p, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || id == uuid.Nil {
		writeError(w, 400, "invalid speech job")
		return nil, p.AccountID, false
	}
	if h.deps.ImageJobReader == nil {
		writeError(w, 503, "speech unavailable")
		return nil, p.AccountID, false
	}
	if h.deps.ImageJobExpiry != nil {
		if _, err := h.deps.ImageJobExpiry.ReconcileJob(r.Context(), p.AccountID, id); err != nil {
			writeError(w, 503, "speech unavailable")
			return nil, p.AccountID, false
		}
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), p.AccountID, id)
	if err != nil {
		writeError(w, 404, "speech job not found")
		return nil, p.AccountID, false
	}
	if _, ok := speechJobDTO(job, p.AccountID); !ok {
		writeError(w, 404, "speech job not found")
		return nil, p.AccountID, false
	}
	return job, p.AccountID, true
}
func (h *Handler) getSpeechJob(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadSpeechJob(w, r)
	if !ok {
		return
	}
	dto, _ := speechJobDTO(job, account)
	writeJSON(w, 200, struct {
		Job safeSpeechJob `json:"job"`
	}{dto})
}
func (h *Handler) activateSpeechJob(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadSpeechJob(w, r)
	if !ok {
		return
	}
	params, _ := speechgeneration.DecodeJob(job)
	if h.deps.ImageJobs == nil || !providermodels.StaticRegistry().MediaCandidateAdmitted(params.ModelID, params.Action()) {
		writeError(w, 503, "speech awaits verification")
		return
	}
	if !h.validateSpeechInput(r, account, params.Request) {
		writeError(w, 400, "invalid speech input")
		return
	}
	job, err := h.deps.ImageJobs.ActivatePreparedAccountJob(r.Context(), account, job.ID)
	if errors.Is(err, domain.ErrInsufficientCredits) {
		writeError(w, 402, "insufficient credits")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		writeError(w, 409, "speech activation conflict")
		return
	}
	if err != nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	dto, ok := speechJobDTO(job, account)
	if !ok {
		writeError(w, 503, "speech unavailable")
		return
	}
	writeJSON(w, 200, struct {
		Job safeSpeechJob `json:"job"`
	}{dto})
}
func (h *Handler) getSpeechJobResult(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadSpeechJob(w, r)
	if !ok {
		return
	}
	if h.deps.ImageResults == nil {
		writeError(w, 503, "speech unavailable")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), account, job.ID)
	if err != nil || result.ID != job.ID {
		writeError(w, 404, "speech result not found")
		return
	}
	artifacts := []safeMusicArtifact{}
	for _, a := range result.Artifacts {
		artifacts = append(artifacts, safeMusicArtifact{ID: a.ID, Kind: string(a.MediaType), URL: "/web/v1/speech-artifacts/" + a.ID.String()})
	}
	writeJSON(w, 200, struct {
		JobID     uuid.UUID           `json:"job_id"`
		Artifacts []safeMusicArtifact `json:"artifacts"`
	}{job.ID, artifacts})
}
func (h *Handler) getSpeechArtifact(w http.ResponseWriter, r *http.Request) {
	h.serveGeneratedArtifact(w, r, func(job *domain.Job, owner uuid.UUID) bool { _, ok := speechJobDTO(job, owner); return ok })
}
