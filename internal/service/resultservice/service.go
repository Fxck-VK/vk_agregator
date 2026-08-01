// Package resultservice exposes owner-scoped, product-safe job history and
// completed result metadata.
package resultservice

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// HistoryItem is the safe job projection returned by account history.
type HistoryItem struct {
	ID        uuid.UUID            `json:"id"`
	Operation domain.OperationType `json:"operation"`
	Modality  domain.Modality      `json:"modality"`
	Status    domain.JobStatus     `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// ArtifactMetadata is safe presentation metadata for one completed output.
// It deliberately contains no object-storage coordinate or directly readable
// URL.
type ArtifactMetadata struct {
	ID         uuid.UUID        `json:"id"`
	MediaType  domain.MediaType `json:"media_type"`
	MIMEType   string           `json:"mime_type"`
	SizeBytes  int64            `json:"size_bytes"`
	Width      int              `json:"width"`
	Height     int              `json:"height"`
	DurationMS int64            `json:"duration_ms"`
}

// Result is the safe projection of a completed job and all of its output
// artifact metadata.
type Result struct {
	ID        uuid.UUID            `json:"id"`
	Operation domain.OperationType `json:"operation"`
	Modality  domain.Modality      `json:"modality"`
	Status    domain.JobStatus     `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	Artifacts []ArtifactMetadata   `json:"artifacts"`
}

// Service owns the account-scoped result retrieval boundary.
type Service struct {
	jobs       domain.JobRepository
	artifacts  domain.ArtifactRepository
	moderation domain.ModerationResultRepository
}

// New constructs a result retrieval service from its read dependencies.
func New(
	jobs domain.JobRepository,
	artifacts domain.ArtifactRepository,
	moderation domain.ModerationResultRepository,
) *Service {
	return &Service{
		jobs:       jobs,
		artifacts:  artifacts,
		moderation: moderation,
	}
}

// ListHistory returns the repository's account-scoped page without reordering
// or adding artifact or private job fields.
func (s *Service) ListHistory(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]HistoryItem, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}

	jobs, err := s.jobs.ListByAccount(ctx, accountID, limit, offset)
	if err != nil {
		return nil, normalizeReadError(err)
	}

	history := make([]HistoryItem, len(jobs))
	for i, job := range jobs {
		if job == nil {
			return nil, domain.ErrNotFound
		}
		history[i] = historyItemFromJob(job)
	}
	return history, nil
}

// GetResult returns completed output metadata only when the exact account owns
// the job and every linked output is ready, supported, and safely moderated.
func (s *Service) GetResult(ctx context.Context, accountID, jobID uuid.UUID) (Result, error) {
	job, outputs, err := s.requireCompletionOutputs(ctx, accountID, jobID, domain.JobStatusSucceeded)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		ID:        job.ID,
		Operation: job.OperationType,
		Modality:  job.Modality,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
		Artifacts: make([]ArtifactMetadata, len(outputs)),
	}
	for i, artifact := range outputs {
		result.Artifacts[i] = ArtifactMetadata{
			ID:         artifact.ID,
			MediaType:  artifact.MediaType,
			MIMEType:   artifact.MimeType,
			SizeBytes:  artifact.SizeBytes,
			Width:      artifact.Width,
			Height:     artifact.Height,
			DurationMS: artifact.DurationMS,
		}
	}
	return result, nil
}

// RequireCompletionReady verifies that an exact account-owned job has every
// durable safe output required for capture. It accepts both pre-capture states
// used by an at-least-once finalizer; public result reads remain succeeded-only.
func (s *Service) RequireCompletionReady(ctx context.Context, accountID, jobID uuid.UUID) error {
	_, _, err := s.requireCompletionOutputs(
		ctx,
		accountID,
		jobID,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
	)
	return err
}

func (s *Service) requireCompletionOutputs(
	ctx context.Context,
	accountID, jobID uuid.UUID,
	requiredStatuses ...domain.JobStatus,
) (*domain.Job, []*domain.Artifact, error) {
	if accountID == uuid.Nil || jobID == uuid.Nil {
		return nil, nil, domain.ErrNotFound
	}
	if s.jobs == nil {
		return nil, nil, errors.New("resultservice: completion readiness dependencies are required")
	}

	job, err := s.jobs.GetByIDForAccount(ctx, accountID, jobID)
	if err != nil {
		return nil, nil, normalizeReadError(err)
	}
	statusAllowed := false
	if job != nil {
		for _, status := range requiredStatuses {
			if job.Status == status {
				statusAllowed = true
				break
			}
		}
	}
	if job == nil || job.ID != jobID || job.AccountID != accountID || !statusAllowed || len(job.OutputArtifactIDs) == 0 {
		return nil, nil, domain.ErrNotFound
	}
	if s.artifacts == nil || s.moderation == nil {
		return nil, nil, errors.New("resultservice: completion readiness dependencies are required")
	}

	outputs := make([]*domain.Artifact, len(job.OutputArtifactIDs))
	for i, artifactID := range job.OutputArtifactIDs {
		if artifactID == uuid.Nil {
			return nil, nil, domain.ErrNotFound
		}
		artifact, readErr := s.artifacts.GetByIDForAccount(ctx, accountID, artifactID)
		if readErr != nil {
			return nil, nil, normalizeReadError(readErr)
		}
		if artifact == nil ||
			artifact.ID != artifactID ||
			artifact.OwnerAccountID != accountID ||
			artifact.JobID == nil ||
			*artifact.JobID != job.ID ||
			artifact.Kind != domain.ArtifactKindOutput ||
			artifact.Status != domain.ArtifactStatusReady ||
			!artifact.MediaType.Valid() {
			return nil, nil, domain.ErrNotFound
		}
		outputs[i] = artifact
	}

	moderationResults, err := s.moderation.ListByJob(ctx, job.ID)
	if err != nil {
		return nil, nil, normalizeReadError(err)
	}
	for _, artifact := range outputs {
		if !artifactModerationAllowed(job.ID, artifact.ID, moderationResults) {
			return nil, nil, domain.ErrNotFound
		}
	}
	return job, outputs, nil
}

func historyItemFromJob(job *domain.Job) HistoryItem {
	return HistoryItem{
		ID:        job.ID,
		Operation: job.OperationType,
		Modality:  job.Modality,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}

func artifactModerationAllowed(jobID, artifactID uuid.UUID, results []*domain.ModerationResult) bool {
	matched := false
	for _, result := range results {
		if result == nil ||
			result.JobID != jobID ||
			result.Stage != domain.ModerationStageOutput ||
			result.ArtifactID == nil ||
			*result.ArtifactID != artifactID {
			continue
		}
		matched = true
		if !result.Decision.Allowed() {
			return false
		}
	}
	return matched
}

func normalizeReadError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}
