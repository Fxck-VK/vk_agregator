package memory

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// PreparedWebImageExpiryRepository is the in-memory equivalent of the bounded
// PostgreSQL expiry claim. It shares JobRepo's mutex so competing local calls
// cannot double-transition the same prepared row.
type PreparedWebImageExpiryRepository struct {
	jobs *JobRepo
}

// NewPreparedWebImageExpiryRepository builds a repository over one shared job
// store.
func NewPreparedWebImageExpiryRepository(jobs *JobRepo) *PreparedWebImageExpiryRepository {
	return &PreparedWebImageExpiryRepository{jobs: jobs}
}

// ExpireDuePreparedWebImages updates at most limit due web image preparations
// in expiry/id order. A nil accountID is global; a non-nil id remains exact.
func (r *PreparedWebImageExpiryRepository) ExpireDuePreparedWebImages(_ context.Context, accountID *uuid.UUID, now time.Time, limit int) (int, bool, error) {
	if r == nil || r.jobs == nil {
		return 0, false, errors.New("memory: prepared web image expiry jobs are required")
	}
	if limit <= 0 {
		return 0, false, errors.New("memory: prepared web image expiry limit must be positive")
	}
	if accountID != nil && *accountID == uuid.Nil {
		return 0, false, nil
	}

	r.jobs.mu.Lock()
	defer r.jobs.mu.Unlock()
	candidates := make([]domain.Job, 0, limit+1)
	for _, job := range r.jobs.byID {
		if !duePreparedWebImage(job, accountID, now) {
			continue
		}
		candidates = append(candidates, job)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ExpiresAt.Equal(*candidates[j].ExpiresAt) {
			return candidates[i].ID.String() < candidates[j].ID.String()
		}
		return candidates[i].ExpiresAt.Before(*candidates[j].ExpiresAt)
	})
	hasMore := len(candidates) > limit
	if hasMore {
		candidates = candidates[:limit]
	}
	for _, candidate := range candidates {
		job := r.jobs.byID[candidate.ID]
		if !duePreparedWebImage(job, accountID, now) {
			continue
		}
		markPreparedWebImageExpired(&job, now)
		r.jobs.byID[job.ID] = job
	}
	return len(candidates), hasMore, nil
}

// ExpireDuePreparedWebImage updates one exact account-owned due job. It has no
// side effects when the job is foreign, no longer prepared, or not yet due.
func (r *PreparedWebImageExpiryRepository) ExpireDuePreparedWebImage(_ context.Context, accountID, jobID uuid.UUID, now time.Time) (bool, error) {
	if r == nil || r.jobs == nil {
		return false, errors.New("memory: prepared web image expiry jobs are required")
	}
	if accountID == uuid.Nil || jobID == uuid.Nil {
		return false, nil
	}
	r.jobs.mu.Lock()
	defer r.jobs.mu.Unlock()
	job, ok := r.jobs.byID[jobID]
	if !ok || !duePreparedWebImage(job, &accountID, now) {
		return false, nil
	}
	markPreparedWebImageExpired(&job, now)
	r.jobs.byID[job.ID] = job
	return true, nil
}

func duePreparedWebImage(job domain.Job, accountID *uuid.UUID, now time.Time) bool {
	return (accountID == nil || job.AccountID == *accountID) &&
		job.AccountID != uuid.Nil &&
		job.Source == "web" &&
		job.OperationType == domain.OperationImageGenerate &&
		job.Modality == domain.ModalityImage &&
		job.Status == domain.JobStatusPrepared &&
		job.ExpiresAt != nil &&
		!job.ExpiresAt.After(now)
}

func markPreparedWebImageExpired(job *domain.Job, now time.Time) {
	job.Status = domain.JobStatusExpired
	job.ErrorCode = domain.PreparedConfirmationExpiredCode
	job.ErrorMessage = domain.PreparedConfirmationExpiredMessage
	job.UpdatedAt = now
}
