package postgres

import (
	"context"
	"time"

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

// HealthSnapshot returns safe provider-level aggregates for operator screens.
func (r *ProviderTaskRepository) HealthSnapshot(ctx context.Context, since time.Time) ([]domain.ProviderTaskHealth, error) {
	const q = `
		SELECT
			provider,
			count(*)::bigint,
			count(*) FILTER (WHERE status = $2)::bigint,
			count(*) FILTER (WHERE error_class = $3)::bigint,
			count(*) FILTER (WHERE status IN ($4, $5))::bigint,
			COALESCE((
				percentile_disc(0.95) WITHIN GROUP (
					ORDER BY EXTRACT(EPOCH FROM (completed_at - COALESCE(submitted_at, created_at))) * 1000
				) FILTER (
					WHERE completed_at IS NOT NULL
					  AND completed_at >= COALESCE(submitted_at, created_at)
				)
			)::bigint, 0)::bigint
		FROM provider_tasks
		WHERE created_at >= $1
		GROUP BY provider`
	rows, err := r.db.Query(ctx, q,
		since,
		domain.ProviderTaskFailed,
		domain.ProviderErrRateLimited,
		domain.ProviderTaskPending,
		domain.ProviderTaskProcessing,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := map[domain.ProviderName]domain.ProviderTaskHealth{}
	var order []domain.ProviderName
	for rows.Next() {
		var item domain.ProviderTaskHealth
		if err := rows.Scan(
			&item.Provider,
			&item.TotalCount,
			&item.FailedCount,
			&item.RateLimitedCount,
			&item.InFlightCount,
			&item.LatencyP95Ms,
		); err != nil {
			return nil, mapError(err)
		}
		items[item.Provider] = item
		order = append(order, item.Provider)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}

	const latestQ = `
		SELECT DISTINCT ON (provider) provider, error_class, updated_at
		FROM provider_tasks
		WHERE created_at >= $1 AND error_class <> ''
		ORDER BY provider, updated_at DESC`
	latestRows, err := r.db.Query(ctx, latestQ, since)
	if err != nil {
		return nil, mapError(err)
	}
	defer latestRows.Close()
	for latestRows.Next() {
		var provider domain.ProviderName
		var class domain.ProviderErrorClass
		var at time.Time
		if err := latestRows.Scan(&provider, &class, &at); err != nil {
			return nil, mapError(err)
		}
		item := items[provider]
		item.LatestErrorClass = class
		item.LatestErrorAt = &at
		items[provider] = item
	}
	if err := latestRows.Err(); err != nil {
		return nil, mapError(err)
	}

	out := make([]domain.ProviderTaskHealth, 0, len(order))
	for _, provider := range order {
		out = append(out, items[provider])
	}
	return out, nil
}

func scanProviderTask(row rowScanner, task *domain.ProviderTask) error {
	return row.Scan(
		&task.ID, &task.JobID, &task.Provider, &task.ModelCode, &task.ExternalID, &task.AttemptNo,
		&task.Status, &task.Request, &task.Result, &task.ErrorClass, &task.IdempotencyKey,
		&task.SubmittedAt, &task.CompletedAt, &task.CreatedAt, &task.UpdatedAt,
	)
}
