package postgres

import (
	"context"
	"errors"
	"fmt"

	"vk-ai-aggregator/internal/domain"
)

const resultReadyCandidateEventType = "event.job.result_ready"

// ResultReadyCandidateRepository is the PostgreSQL bounded anti-join adapter
// for historical result-ready reconciliation.
type ResultReadyCandidateRepository struct {
	db Querier
}

// NewResultReadyCandidateRepository builds a candidate repository over a pool
// or transaction-compatible querier.
func NewResultReadyCandidateRepository(db Querier) *ResultReadyCandidateRepository {
	return &ResultReadyCandidateRepository{db: db}
}

var _ domain.ResultReadyCandidateRepository = (*ResultReadyCandidateRepository)(nil)

// ListMissingCanonicalResultReady returns at most limit jobs and fetches one
// extra row to derive hasMore without an exact global count.
func (r *ResultReadyCandidateRepository) ListMissingCanonicalResultReady(
	ctx context.Context,
	limit int,
	after *domain.JobCursor,
) ([]*domain.Job, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("postgres: result-ready candidate repository is required")
	}
	if limit <= 0 {
		return nil, false, errors.New("postgres: result-ready candidate limit must be positive")
	}

	args := []any{
		domain.JobStatusResultReady,
		domain.ResultModeAccountHistory,
		domain.ResultModeExternalPush,
		resultReadyCandidateEventType,
	}
	q := `SELECT ` + jobColumns + `
		FROM jobs
		WHERE jobs.status = $1
		  AND jobs.account_id IS NOT NULL
		  AND jobs.result_mode IN ($2, $3)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM outbox_events AS event
			  WHERE event.aggregate_type = 'job'
			    AND event.aggregate_id = jobs.id
			    AND event.event_type = $4
		  )`
	if after != nil {
		args = append(args, after.CreatedAt, after.ID)
		q += fmt.Sprintf("\n\t\t  AND (jobs.created_at, jobs.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, limit+1)
	q += fmt.Sprintf("\n\t\tORDER BY jobs.created_at DESC, jobs.id DESC LIMIT $%d", len(args))

	jobs, err := NewJobRepository(r.db).queryJobs(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	return jobs, hasMore, nil
}
