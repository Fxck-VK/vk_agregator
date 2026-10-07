package websession

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/resultservice"
)

type webVideoJobParams struct {
	Prompt          string `json:"prompt"`
	ModelID         string `json:"model_id"`
	ModelName       string `json:"model_name"`
	VideoRouteAlias string `json:"video_route_alias,omitempty"`
	DurationSec     int    `json:"duration_sec"`
	Resolution      string `json:"resolution"`
	AspectRatio     string `json:"aspect_ratio"`
}

type safeVideoJobList struct {
	Items      []safeVideoJob `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor *string        `json:"next_cursor"`
}

type safeVideoJob struct {
	ID           uuid.UUID        `json:"id"`
	ModelID      string           `json:"model_id"`
	ModelName    string           `json:"model_name"`
	Prompt       string           `json:"prompt"`
	DurationSec  int              `json:"duration_sec"`
	Resolution   string           `json:"resolution"`
	AspectRatio  string           `json:"aspect_ratio"`
	CostEstimate int64            `json:"cost_estimate"`
	Status       domain.JobStatus `json:"status"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type safeVideoJobResult struct {
	JobID     uuid.UUID                   `json:"job_id"`
	Status    domain.JobStatus            `json:"status"`
	Artifacts []safeVideoArtifactMetadata `json:"artifacts"`
}

type safeVideoArtifactMetadata struct {
	ID         uuid.UUID `json:"id"`
	MIMEType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	DurationMS int64     `json:"duration_ms"`
}

func (h *Handler) listVideoJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobHistory == nil {
		writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
		return
	}
	limitValues, limitProvided := r.URL.Query()["limit"]
	limit, err := imageJobHistoryLimit(limitValues, limitProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid video job limit")
		return
	}
	cursorValues, cursorProvided := r.URL.Query()["cursor"]
	after, err := imageJobHistoryCursor(cursorValues, cursorProvided)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid video job cursor")
		return
	}
	filter := domain.JobFilter{
		AccountID: &principal.AccountID,
		Source:    "web",
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
	}
	jobs, err := h.deps.ImageJobHistory.ListCursor(r.Context(), filter, limit+1, after)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	items := make([]safeVideoJob, 0, len(jobs))
	for _, job := range jobs {
		safeJob, ok := newSafeVideoJob(job)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
			return
		}
		items = append(items, safeJob)
	}
	var nextCursor *string
	if hasMore {
		cursor, ok := encodeImageJobHistoryCursor(domain.JobCursor{
			CreatedAt: jobs[len(jobs)-1].CreatedAt,
			ID:        jobs[len(jobs)-1].ID,
		})
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
			return
		}
		nextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, safeVideoJobList{Items: items, HasMore: hasMore, NextCursor: nextCursor})
}

func (h *Handler) getVideoJobResult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.ImageJobReader == nil || h.deps.ImageResults == nil {
		writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
		return
	}
	jobID, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid video job id")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) || job == nil {
		writeError(w, http.StatusNotFound, "video job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
		return
	}
	if _, ok := newSafeVideoJob(job); !ok {
		writeError(w, http.StatusNotFound, "video job not found")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), principal.AccountID, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "video result not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "video generation unavailable")
		return
	}
	safeResult, ok := newSafeVideoJobResult(result, jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "video result not found")
		return
	}
	writeJSON(w, http.StatusOK, safeResult)
}

func newSafeVideoJob(job *domain.Job) (safeVideoJob, bool) {
	if job == nil ||
		job.ID == uuid.Nil ||
		job.AccountID == uuid.Nil ||
		job.Source != "web" ||
		job.UserID != uuid.Nil ||
		job.VKPeerID != 0 ||
		job.CommandID != uuid.Nil ||
		job.ChannelContext == nil ||
		job.ChannelContext.Channel != domain.ChannelWeb ||
		job.ChannelContext.RecipientRef != "" ||
		job.ChannelContext.ThreadRef != "" ||
		job.DeliveryTarget != nil ||
		job.ResultMode != domain.ResultModeAccountHistory ||
		job.OperationType != domain.OperationVideoGenerate ||
		job.Modality != domain.ModalityVideo ||
		job.CostEstimate <= 0 ||
		job.ValidateResultContract() != nil {
		return safeVideoJob{}, false
	}
	var params webVideoJobParams
	if err := json.Unmarshal(job.Params, &params); err != nil {
		return safeVideoJob{}, false
	}
	params.Prompt = strings.TrimSpace(params.Prompt)
	params.ModelID = strings.TrimSpace(params.ModelID)
	params.ModelName = strings.TrimSpace(params.ModelName)
	params.VideoRouteAlias = strings.TrimSpace(params.VideoRouteAlias)
	params.Resolution = strings.TrimSpace(params.Resolution)
	params.AspectRatio = strings.TrimSpace(params.AspectRatio)
	if params.Prompt == "" ||
		params.ModelID == "" ||
		params.ModelName == "" ||
		params.DurationSec <= 0 ||
		params.Resolution == "" ||
		params.AspectRatio == "" ||
		(params.VideoRouteAlias != "" && params.VideoRouteAlias != params.ModelID) {
		return safeVideoJob{}, false
	}
	return safeVideoJob{
		ID:           job.ID,
		ModelID:      params.ModelID,
		ModelName:    params.ModelName,
		Prompt:       params.Prompt,
		DurationSec:  params.DurationSec,
		Resolution:   params.Resolution,
		AspectRatio:  params.AspectRatio,
		CostEstimate: job.CostEstimate,
		Status:       job.Status,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}, true
}

func newSafeVideoJobResult(result resultservice.Result, expectedJobID uuid.UUID) (safeVideoJobResult, bool) {
	if expectedJobID == uuid.Nil ||
		result.ID != expectedJobID ||
		result.Operation != domain.OperationVideoGenerate ||
		result.Modality != domain.ModalityVideo ||
		result.Status != domain.JobStatusSucceeded ||
		len(result.Artifacts) == 0 {
		return safeVideoJobResult{}, false
	}
	artifacts := make([]safeVideoArtifactMetadata, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		if artifact.ID == uuid.Nil ||
			artifact.MediaType != domain.MediaTypeVideo ||
			!strings.HasPrefix(artifact.MIMEType, "video/") ||
			artifact.SizeBytes < 1 ||
			artifact.Width < 1 ||
			artifact.Height < 1 ||
			artifact.DurationMS < 1 {
			return safeVideoJobResult{}, false
		}
		artifacts = append(artifacts, safeVideoArtifactMetadata{
			ID:         artifact.ID,
			MIMEType:   artifact.MIMEType,
			SizeBytes:  artifact.SizeBytes,
			Width:      artifact.Width,
			Height:     artifact.Height,
			DurationMS: artifact.DurationMS,
		})
	}
	return safeVideoJobResult{JobID: expectedJobID, Status: result.Status, Artifacts: artifacts}, true
}
