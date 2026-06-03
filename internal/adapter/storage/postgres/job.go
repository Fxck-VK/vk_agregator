package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// JobRepository is the PostgreSQL implementation of domain.JobRepository.
type JobRepository struct {
	db Querier
}

// NewJobRepository builds a JobRepository over the given querier.
func NewJobRepository(db Querier) *JobRepository {
	return &JobRepository{db: db}
}

var _ domain.JobRepository = (*JobRepository)(nil)

const jobColumns = `id, user_id, vk_peer_id, command_id, operation_type, modality,
	provider_id, model_id, status, priority, idempotency_key, correlation_id,
	input_artifact_ids, output_artifact_ids, params, cost_estimate, cost_reserved,
	cost_captured, error_code, error_message, created_at, updated_at, expires_at`

// Create inserts a new job.
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if len(job.Params) == 0 {
		job.Params = []byte("{}")
	}
	const q = `
		INSERT INTO jobs (
			id, user_id, vk_peer_id, command_id, operation_type, modality,
			provider_id, model_id, status, priority, idempotency_key, correlation_id,
			input_artifact_ids, output_artifact_ids, params, cost_estimate, cost_reserved,
			cost_captured, error_code, error_message, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21
		)
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, job.UserID, job.VKPeerID, job.CommandID, job.OperationType, job.Modality,
		job.ProviderID, job.ModelID, job.Status, job.Priority, job.IdempotencyKey, job.CorrelationID,
		job.InputArtifactIDs, job.OutputArtifactIDs, []byte(job.Params), job.CostEstimate, job.CostReserved,
		job.CostCaptured, job.ErrorCode, job.ErrorMessage, job.ExpiresAt,
	)
	return mapError(scanJob(row, job))
}

// GetByID fetches a job by id.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, id), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIdempotencyKey fetches a job by its idempotency key.
func (r *JobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE idempotency_key = $1`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, key), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateStatus applies an explicit state-machine transition. The WHERE clause
// pins the previous status so a concurrent transition cannot be lost; a missing
// match is reported as ErrConflict.
func (r *JobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domain.JobStatus, errCode, errMessage string) error {
	const q = `
		UPDATE jobs
		SET status = $3, error_code = $4, error_message = $5, updated_at = now()
		WHERE id = $1 AND status = $2`
	tag, err := r.db.Exec(ctx, q, id, from, to, errCode, errMessage)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

// Update persists mutable, non-status fields of a job.
func (r *JobRepository) Update(ctx context.Context, job *domain.Job) error {
	if len(job.Params) == 0 {
		job.Params = []byte("{}")
	}
	const q = `
		UPDATE jobs
		SET provider_id = $2, model_id = $3, priority = $4, correlation_id = $5,
		    input_artifact_ids = $6, output_artifact_ids = $7, params = $8,
		    cost_estimate = $9, cost_reserved = $10, cost_captured = $11,
		    error_code = $12, error_message = $13, expires_at = $14, updated_at = now()
		WHERE id = $1
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, job.ProviderID, job.ModelID, job.Priority, job.CorrelationID,
		job.InputArtifactIDs, job.OutputArtifactIDs, []byte(job.Params),
		job.CostEstimate, job.CostReserved, job.CostCaptured,
		job.ErrorCode, job.ErrorMessage, job.ExpiresAt,
	)
	return mapError(scanJob(row, job))
}

// ListByUser returns the most recent jobs for a user, newest first.
func (r *JobRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	const q = `SELECT ` + jobColumns + `
		FROM jobs WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		var job domain.Job
		if err := scanJob(rows, &job); err != nil {
			return nil, mapError(err)
		}
		jobs = append(jobs, &job)
	}
	return jobs, mapError(rows.Err())
}

func scanJob(row rowScanner, job *domain.Job) error {
	return row.Scan(
		&job.ID, &job.UserID, &job.VKPeerID, &job.CommandID, &job.OperationType, &job.Modality,
		&job.ProviderID, &job.ModelID, &job.Status, &job.Priority, &job.IdempotencyKey, &job.CorrelationID,
		&job.InputArtifactIDs, &job.OutputArtifactIDs, &job.Params, &job.CostEstimate, &job.CostReserved,
		&job.CostCaptured, &job.ErrorCode, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.ExpiresAt,
	)
}
