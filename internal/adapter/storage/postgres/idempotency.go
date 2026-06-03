package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
)

// IdempotencyRepository is the PostgreSQL implementation of
// domain.IdempotencyRepository.
type IdempotencyRepository struct {
	db Querier
}

// NewIdempotencyRepository builds an IdempotencyRepository over the querier.
func NewIdempotencyRepository(db Querier) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

var _ domain.IdempotencyRepository = (*IdempotencyRepository)(nil)

const idempotencyColumns = `key, scope, resource_type, resource_id, status, created_at, expires_at`

// GetOrCreate atomically inserts a record in the started state, or returns the
// existing record when the key already exists. created reports whether the row
// was newly inserted.
func (r *IdempotencyRepository) GetOrCreate(ctx context.Context, rec *domain.IdempotencyRecord) (*domain.IdempotencyRecord, bool, error) {
	if rec.Status == "" {
		rec.Status = domain.IdempotencyStarted
	}
	if rec.ExpiresAt.IsZero() {
		rec.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	const insert = `
		INSERT INTO idempotency_keys (key, scope, resource_type, resource_id, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key) DO NOTHING
		RETURNING ` + idempotencyColumns
	var created domain.IdempotencyRecord
	err := scanIdempotency(r.db.QueryRow(ctx, insert,
		rec.Key, rec.Scope, rec.ResourceType, rec.ResourceID, rec.Status, rec.ExpiresAt,
	), &created)
	switch {
	case err == nil:
		*rec = created
		return rec, true, nil
	case errors.Is(err, pgx.ErrNoRows):
		// Key already existed: the insert affected no rows, so load the record.
		existing, getErr := r.Get(ctx, rec.Key)
		if getErr != nil {
			return nil, false, getErr
		}
		return existing, false, nil
	default:
		return nil, false, mapError(err)
	}
}

// MarkCompleted records successful completion and the resource produced.
func (r *IdempotencyRepository) MarkCompleted(ctx context.Context, key string, resourceID uuid.UUID) error {
	const q = `
		UPDATE idempotency_keys
		SET status = $2, resource_id = $3
		WHERE key = $1`
	tag, err := r.db.Exec(ctx, q, key, domain.IdempotencyCompleted, resourceID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkFailed records a failed attempt so the operation may be retried.
func (r *IdempotencyRepository) MarkFailed(ctx context.Context, key string) error {
	const q = `UPDATE idempotency_keys SET status = $2 WHERE key = $1`
	tag, err := r.db.Exec(ctx, q, key, domain.IdempotencyFailed)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get fetches a record by key.
func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	const q = `SELECT ` + idempotencyColumns + ` FROM idempotency_keys WHERE key = $1`
	var rec domain.IdempotencyRecord
	if err := mapError(scanIdempotency(r.db.QueryRow(ctx, q, key), &rec)); err != nil {
		return nil, err
	}
	return &rec, nil
}

func scanIdempotency(row rowScanner, rec *domain.IdempotencyRecord) error {
	return row.Scan(
		&rec.Key, &rec.Scope, &rec.ResourceType, &rec.ResourceID, &rec.Status,
		&rec.CreatedAt, &rec.ExpiresAt,
	)
}
