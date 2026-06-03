package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ModerationResultRepository is the PostgreSQL implementation of
// domain.ModerationResultRepository.
type ModerationResultRepository struct {
	db Querier
}

// NewModerationResultRepository builds a ModerationResultRepository.
func NewModerationResultRepository(db Querier) *ModerationResultRepository {
	return &ModerationResultRepository{db: db}
}

var _ domain.ModerationResultRepository = (*ModerationResultRepository)(nil)

const moderationColumns = `id, job_id, artifact_id, stage, decision, categories, provider, created_at`

// Create inserts a moderation verdict.
func (r *ModerationResultRepository) Create(ctx context.Context, m *domain.ModerationResult) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	categories := m.Categories
	if categories == nil {
		categories = []string{}
	}
	const q = `
		INSERT INTO moderation_results (id, job_id, artifact_id, stage, decision, categories, provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + moderationColumns
	row := r.db.QueryRow(ctx, q, m.ID, m.JobID, m.ArtifactID, m.Stage, m.Decision, categories, m.Provider)
	return mapError(scanModeration(row, m))
}

// ListByJob returns moderation verdicts for a job, oldest first.
func (r *ModerationResultRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.ModerationResult, error) {
	const q = `SELECT ` + moderationColumns + ` FROM moderation_results WHERE job_id = $1 ORDER BY created_at`
	rows, err := r.db.Query(ctx, q, jobID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.ModerationResult
	for rows.Next() {
		var m domain.ModerationResult
		if err := scanModeration(rows, &m); err != nil {
			return nil, mapError(err)
		}
		out = append(out, &m)
	}
	return out, mapError(rows.Err())
}

func scanModeration(row rowScanner, m *domain.ModerationResult) error {
	return row.Scan(&m.ID, &m.JobID, &m.ArtifactID, &m.Stage, &m.Decision, &m.Categories, &m.Provider, &m.CreatedAt)
}
