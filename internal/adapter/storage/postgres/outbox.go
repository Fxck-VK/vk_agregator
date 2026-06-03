package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// OutboxRepository is the PostgreSQL implementation of domain.OutboxRepository.
type OutboxRepository struct {
	db Querier
}

// NewOutboxRepository builds an OutboxRepository over the given querier. It is
// typically constructed over a transaction so events are written atomically
// with the state change that produced them.
func NewOutboxRepository(db Querier) *OutboxRepository {
	return &OutboxRepository{db: db}
}

var _ domain.OutboxRepository = (*OutboxRepository)(nil)

const outboxColumns = `id, aggregate_type, aggregate_id, event_type, payload, status,
	attempts, next_attempt_at, created_at, published_at`

// Add inserts an outbox event.
func (r *OutboxRepository) Add(ctx context.Context, e *domain.OutboxEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if len(e.Payload) == 0 {
		e.Payload = []byte("{}")
	}
	if e.Status == "" {
		e.Status = domain.OutboxPending
	}
	const q = `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, attempts, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()))
		RETURNING ` + outboxColumns
	row := r.db.QueryRow(ctx, q,
		e.ID, e.AggregateType, e.AggregateID, e.EventType, []byte(e.Payload), e.Status,
		e.Attempts, nullableTime(e.NextAttemptAt),
	)
	return mapError(scanOutbox(row, e))
}

// FetchPending returns up to limit events ready for publication, skipping rows
// locked by other publishers so multiple workers can drain concurrently.
func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	const q = `
		SELECT ` + outboxColumns + `
		FROM outbox_events
		WHERE status = $1 AND next_attempt_at <= now()
		ORDER BY next_attempt_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`
	rows, err := r.db.Query(ctx, q, domain.OutboxPending, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := scanOutbox(rows, &e); err != nil {
			return nil, mapError(err)
		}
		events = append(events, &e)
	}
	return events, mapError(rows.Err())
}

// MarkPublished marks an event as successfully published.
func (r *OutboxRepository) MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error {
	const q = `
		UPDATE outbox_events
		SET status = $2, published_at = $3
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, domain.OutboxPublished, publishedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkFailed records a failed publication and schedules the next attempt.
func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error {
	const q = `
		UPDATE outbox_events
		SET attempts = attempts + 1, next_attempt_at = $2
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, nextAttemptAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanOutbox(row rowScanner, e *domain.OutboxEvent) error {
	return row.Scan(
		&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload, &e.Status,
		&e.Attempts, &e.NextAttemptAt, &e.CreatedAt, &e.PublishedAt,
	)
}
