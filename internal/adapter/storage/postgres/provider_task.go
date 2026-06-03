package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ProviderTaskRepository is the PostgreSQL implementation of
// domain.ProviderTaskRepository.
type ProviderTaskRepository struct {
	db Querier
}

// NewProviderTaskRepository builds a ProviderTaskRepository over the querier.
func NewProviderTaskRepository(db Querier) *ProviderTaskRepository {
	return &ProviderTaskRepository{db: db}
}

var _ domain.ProviderTaskRepository = (*ProviderTaskRepository)(nil)

const providerTaskColumns = `id, job_id, provider, model_code, external_id, attempt_no,
	status, request, result, error_class, idempotency_key, submitted_at, completed_at,
	created_at, updated_at`

// Create inserts a provider task for a job.
func (r *ProviderTaskRepository) Create(ctx context.Context, task *domain.ProviderTask) error {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if len(task.Request) == 0 {
		task.Request = []byte("{}")
	}
	if task.AttemptNo == 0 {
		task.AttemptNo = 1
	}
	const q = `
		INSERT INTO provider_tasks (
			id, job_id, provider, model_code, external_id, attempt_no,
			status, request, result, error_class, idempotency_key, submitted_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING ` + providerTaskColumns
	row := r.db.QueryRow(ctx, q,
		task.ID, task.JobID, task.Provider, task.ModelCode, task.ExternalID, task.AttemptNo,
		task.Status, []byte(task.Request), rawOrNil(task.Result), task.ErrorClass,
		task.IdempotencyKey, task.SubmittedAt, task.CompletedAt,
	)
	return mapError(scanProviderTask(row, task))
}

// Update persists changes to a provider task.
func (r *ProviderTaskRepository) Update(ctx context.Context, task *domain.ProviderTask) error {
	const q = `
		UPDATE provider_tasks
		SET external_id = $2, attempt_no = $3, status = $4, result = $5,
		    error_class = $6, submitted_at = $7, completed_at = $8, updated_at = now()
		WHERE id = $1
		RETURNING ` + providerTaskColumns
	row := r.db.QueryRow(ctx, q,
		task.ID, task.ExternalID, task.AttemptNo, task.Status, rawOrNil(task.Result),
		task.ErrorClass, task.SubmittedAt, task.CompletedAt,
	)
	return mapError(scanProviderTask(row, task))
}

// GetByID fetches a provider task by id.
func (r *ProviderTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTask, error) {
	const q = `SELECT ` + providerTaskColumns + ` FROM provider_tasks WHERE id = $1`
	var task domain.ProviderTask
	if err := mapError(scanProviderTask(r.db.QueryRow(ctx, q, id), &task)); err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByExternalID fetches a task by provider and external id.
func (r *ProviderTaskRepository) GetByExternalID(ctx context.Context, provider domain.ProviderName, externalID string) (*domain.ProviderTask, error) {
	const q = `SELECT ` + providerTaskColumns + `
		FROM provider_tasks WHERE provider = $1 AND external_id = $2`
	var task domain.ProviderTask
	if err := mapError(scanProviderTask(r.db.QueryRow(ctx, q, provider, externalID), &task)); err != nil {
		return nil, err
	}
	return &task, nil
}

// ListByJob returns all provider tasks for a job, oldest attempt first.
func (r *ProviderTaskRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.ProviderTask, error) {
	const q = `SELECT ` + providerTaskColumns + `
		FROM provider_tasks WHERE job_id = $1
		ORDER BY attempt_no ASC, created_at ASC`
	rows, err := r.db.Query(ctx, q, jobID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var tasks []*domain.ProviderTask
	for rows.Next() {
		var task domain.ProviderTask
		if err := scanProviderTask(rows, &task); err != nil {
			return nil, mapError(err)
		}
		tasks = append(tasks, &task)
	}
	return tasks, mapError(rows.Err())
}

func scanProviderTask(row rowScanner, task *domain.ProviderTask) error {
	return row.Scan(
		&task.ID, &task.JobID, &task.Provider, &task.ModelCode, &task.ExternalID, &task.AttemptNo,
		&task.Status, &task.Request, &task.Result, &task.ErrorClass, &task.IdempotencyKey,
		&task.SubmittedAt, &task.CompletedAt, &task.CreatedAt, &task.UpdatedAt,
	)
}
