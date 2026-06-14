package postgres

import (
	"context"
	"fmt"
	"strings"

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

const jobColumns = `id, user_id, source, vk_peer_id, command_id, operation_type, modality,
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
	job.Source = strings.TrimSpace(job.Source)
	if job.Source == "" {
		job.Source = "unknown"
	}
	// command_id is nullable; pass nil when the job has no associated command
	// (e.g. jobs created directly through the Mini App BFF).
	var commandID *uuid.UUID
	if job.CommandID != uuid.Nil {
		commandID = &job.CommandID
	}
	const q = `
		INSERT INTO jobs (
			id, user_id, source, vk_peer_id, command_id, operation_type, modality,
			provider_id, model_id, status, priority, idempotency_key, correlation_id,
			input_artifact_ids, output_artifact_ids, params, cost_estimate, cost_reserved,
			cost_captured, error_code, error_message, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, job.UserID, job.Source, job.VKPeerID, commandID, job.OperationType, job.Modality,
		job.ProviderID, job.ModelID, job.Status, job.Priority, job.IdempotencyKey, job.CorrelationID,
		uuidArray(job.InputArtifactIDs), uuidArray(job.OutputArtifactIDs), []byte(job.Params), job.CostEstimate, job.CostReserved,
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
	job.Source = strings.TrimSpace(job.Source)
	if job.Source == "" {
		job.Source = "unknown"
	}
	const q = `
		UPDATE jobs
		SET source = $2, provider_id = $3, model_id = $4, priority = $5, correlation_id = $6,
		    input_artifact_ids = $7, output_artifact_ids = $8, params = $9,
		    cost_estimate = $10, cost_reserved = $11, cost_captured = $12,
		    error_code = $13, error_message = $14, expires_at = $15, updated_at = now()
		WHERE id = $1
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, job.Source, job.ProviderID, job.ModelID, job.Priority, job.CorrelationID,
		uuidArray(job.InputArtifactIDs), uuidArray(job.OutputArtifactIDs), []byte(job.Params),
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

// List returns jobs matching the filter, newest first. The WHERE clause is
// built dynamically from the non-zero filter fields.
func (r *JobRepository) List(ctx context.Context, filter domain.JobFilter, limit, offset int) ([]*domain.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs`
	var (
		conds []string
		args  []any
	)
	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		conds = append(conds, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Operation != "" {
		args = append(args, filter.Operation)
		conds = append(conds, fmt.Sprintf("operation_type = $%d", len(args)))
	}
	if filter.Modality != "" {
		args = append(args, filter.Modality)
		conds = append(conds, fmt.Sprintf("modality = $%d", len(args)))
	}
	if filter.ErrorCode != "" {
		args = append(args, filter.ErrorCode)
		conds = append(conds, fmt.Sprintf("error_code = $%d", len(args)))
	}
	if filter.CorrelationID != "" {
		args = append(args, filter.CorrelationID)
		conds = append(conds, fmt.Sprintf("correlation_id = $%d", len(args)))
	}
	if filter.CreatedFrom != nil {
		args = append(args, *filter.CreatedFrom)
		conds = append(conds, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if filter.CreatedTo != nil {
		args = append(args, *filter.CreatedTo)
		conds = append(conds, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args))
	args = append(args, offset)
	q += fmt.Sprintf(" OFFSET $%d", len(args))

	rows, err := r.db.Query(ctx, q, args...)
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

func (r *JobRepository) CountActiveByUserOperation(ctx context.Context, userID uuid.UUID, operation domain.OperationType) (int, error) {
	statuses := domain.ActiveWorkJobStatuses()
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusValues = append(statusValues, string(status))
	}
	const q = `
		SELECT count(*)
		FROM jobs
		WHERE user_id = $1
		  AND operation_type = $2
		  AND status = ANY($3::text[])`
	var count int
	if err := r.db.QueryRow(ctx, q, userID, operation, statusValues).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

// CountSucceededByUser returns completed successful jobs for one user.
func (r *JobRepository) CountSucceededByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `
		SELECT count(*)
		FROM jobs
		WHERE user_id = $1 AND status = $2`
	var count int
	if err := r.db.QueryRow(ctx, q, userID, domain.JobStatusSucceeded).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func scanJob(row rowScanner, job *domain.Job) error {
	var commandID *uuid.UUID
	if err := row.Scan(
		&job.ID, &job.UserID, &job.Source, &job.VKPeerID, &commandID, &job.OperationType, &job.Modality,
		&job.ProviderID, &job.ModelID, &job.Status, &job.Priority, &job.IdempotencyKey, &job.CorrelationID,
		&job.InputArtifactIDs, &job.OutputArtifactIDs, &job.Params, &job.CostEstimate, &job.CostReserved,
		&job.CostCaptured, &job.ErrorCode, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.ExpiresAt,
	); err != nil {
		return err
	}
	if commandID != nil {
		job.CommandID = *commandID
	}
	return nil
}
