package postgres

import (
	"context"
	"fmt"
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
var _ domain.OutboxHealthRepository = (*OutboxRepository)(nil)

const outboxColumns = `id, aggregate_type, aggregate_id, event_type, payload, status,
	attempts, next_attempt_at, created_at, published_at, claim_token,
	COALESCE(claim_owner, ''), lease_until, last_error_code, failed_at`

const claimedOutboxColumns = `event.id, event.aggregate_type, event.aggregate_id,
	event.event_type, event.payload, event.status, event.attempts,
	event.next_attempt_at, event.created_at, event.published_at, event.claim_token,
	COALESCE(event.claim_owner, '') AS claim_owner, event.lease_until,
	event.last_error_code, event.failed_at`

const maxOutboxErrorCodeCharacters = 128

// Add inserts an outbox event.
func (r *OutboxRepository) Add(ctx context.Context, e *domain.OutboxEvent) error {
	prepareOutboxEvent(e)
	const q = `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type, payload, status,
			attempts, next_attempt_at, published_at, claim_token, claim_owner,
			lease_until, last_error_code, failed_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()), $9, $10,
			NULLIF($11, ''), $12, $13, $14
		)
		RETURNING ` + outboxColumns
	row := r.db.QueryRow(ctx, q,
		e.ID, e.AggregateType, e.AggregateID, e.EventType, []byte(e.Payload), e.Status,
		e.Attempts, nullableTime(e.NextAttemptAt), e.PublishedAt, e.ClaimToken, e.ClaimOwner,
		e.LeaseUntil, e.LastErrorCode, e.FailedAt,
	)
	return mapError(scanOutbox(row, e))
}

func (r *OutboxRepository) ExistsForAggregateEvent(ctx context.Context, aggregateType string, aggregateID uuid.UUID, eventType string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM outbox_events
			WHERE aggregate_type = $1
			  AND aggregate_id = $2
			  AND event_type = $3
		)`
	var exists bool
	err := r.db.QueryRow(ctx, q, aggregateType, aggregateID, eventType).Scan(&exists)
	return exists, mapError(err)
}

func (r *OutboxRepository) AddIfAbsentByID(ctx context.Context, e *domain.OutboxEvent) (bool, error) {
	prepareOutboxEvent(e)
	const q = `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type, payload, status,
			attempts, next_attempt_at, published_at, claim_token, claim_owner,
			lease_until, last_error_code, failed_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()), $9, $10,
			NULLIF($11, ''), $12, $13, $14
		)
		ON CONFLICT DO NOTHING`
	tag, err := r.db.Exec(ctx, q,
		e.ID, e.AggregateType, e.AggregateID, e.EventType, []byte(e.Payload), e.Status,
		e.Attempts, nullableTime(e.NextAttemptAt), e.PublishedAt, e.ClaimToken, e.ClaimOwner,
		e.LeaseUntil, e.LastErrorCode, e.FailedAt,
	)
	if err != nil {
		return false, mapError(err)
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}

	const confirmSemantic = `
		SELECT EXISTS (
			SELECT 1
			FROM outbox_events
			WHERE aggregate_type = $1
			  AND aggregate_id = $2
			  AND event_type = $3
		)`
	var exists bool
	if err := r.db.QueryRow(
		ctx,
		confirmSemantic,
		e.AggregateType,
		e.AggregateID,
		e.EventType,
	).Scan(&exists); err != nil {
		return false, mapError(err)
	}
	if exists {
		return false, nil
	}
	return false, fmt.Errorf("postgres: outbox conflict without matching semantic event: %w", domain.ErrConflict)
}

func prepareOutboxEvent(e *domain.OutboxEvent) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if len(e.Payload) == 0 {
		e.Payload = []byte("{}")
	}
	if e.Status == "" {
		e.Status = domain.OutboxPending
	}
	e.LastErrorCode = boundOutboxErrorCode(e.LastErrorCode)
}

// ClaimPending atomically leases ready pending or expired processing events.
func (r *OutboxRepository) ClaimPending(ctx context.Context, owner string, now, leaseUntil time.Time, limit int) ([]*domain.OutboxEvent, error) {
	if limit <= 0 {
		return []*domain.OutboxEvent{}, nil
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE (status = 'pending' AND next_attempt_at <= $2)
			   OR (status = 'processing' AND lease_until <= $2)
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT $4
			FOR UPDATE SKIP LOCKED
		),
		claimed AS (
			UPDATE outbox_events AS event
			SET status = 'processing',
				claim_token = gen_random_uuid(),
				claim_owner = NULLIF($1, ''),
				lease_until = $3
			FROM candidates
			WHERE event.id = candidates.id
			RETURNING ` + claimedOutboxColumns + `
		)
		SELECT ` + outboxColumns + `
		FROM claimed
		ORDER BY next_attempt_at ASC, id ASC`
	rows, err := r.db.Query(ctx, q, owner, now, leaseUntil, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	events := make([]*domain.OutboxEvent, 0, limit)
	for rows.Next() {
		var event domain.OutboxEvent
		if err := scanOutbox(rows, &event); err != nil {
			return nil, mapError(err)
		}
		events = append(events, &event)
	}
	return events, mapError(rows.Err())
}

// OutboxHealthSnapshot returns a read-only count/timestamp snapshot suitable
// for private metrics. Scalar subqueries keep each predicate indexable and no
// payload, aggregate identity, claim owner, or raw error leaves PostgreSQL.
func (r *OutboxRepository) OutboxHealthSnapshot(ctx context.Context, now time.Time) (domain.OutboxHealth, error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM outbox_events WHERE status = 'pending'),
			(SELECT COUNT(*) FROM outbox_events WHERE status = 'processing'),
			(SELECT COUNT(*) FROM outbox_events WHERE status = 'failed'),
			(SELECT MIN(created_at) FROM outbox_events WHERE status = 'pending'),
			(SELECT COUNT(*) FROM outbox_events WHERE status = 'processing' AND lease_until <= $1)`
	var snapshot domain.OutboxHealth
	err := r.db.QueryRow(ctx, q, now).Scan(
		&snapshot.Pending,
		&snapshot.Processing,
		&snapshot.Failed,
		&snapshot.OldestPendingCreatedAt,
		&snapshot.ExpiredLeases,
	)
	return snapshot, mapError(err)
}

// MarkPublishedClaimed publishes an event only for its current claim token.
func (r *OutboxRepository) MarkPublishedClaimed(ctx context.Context, id, claimToken uuid.UUID, publishedAt time.Time) (bool, error) {
	const q = `
		UPDATE outbox_events
		SET status = 'published',
			published_at = $3,
			claim_token = NULL,
			claim_owner = NULL,
			lease_until = NULL,
			last_error_code = '',
			failed_at = NULL
		WHERE id = $1 AND status = 'processing' AND claim_token = $2`
	tag, err := r.db.Exec(ctx, q, id, claimToken, publishedAt)
	if err != nil {
		return false, mapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// RetryClaimed returns an event to pending only for its current claim token.
func (r *OutboxRepository) RetryClaimed(ctx context.Context, id, claimToken uuid.UUID, nextAttemptAt time.Time, errorCode string) (bool, error) {
	const q = `
		UPDATE outbox_events
		SET status = 'pending',
			attempts = attempts + 1,
			next_attempt_at = $3,
			claim_token = NULL,
			claim_owner = NULL,
			lease_until = NULL,
			last_error_code = LEFT($4, 128),
			failed_at = NULL
		WHERE id = $1 AND status = 'processing' AND claim_token = $2`
	tag, err := r.db.Exec(ctx, q, id, claimToken, nextAttemptAt, errorCode)
	if err != nil {
		return false, mapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// FailClaimed quarantines an event only for its current claim token.
func (r *OutboxRepository) FailClaimed(ctx context.Context, id, claimToken uuid.UUID, failedAt time.Time, errorCode string) (bool, error) {
	const q = `
		UPDATE outbox_events
		SET status = 'failed',
			attempts = attempts + 1,
			claim_token = NULL,
			claim_owner = NULL,
			lease_until = NULL,
			last_error_code = LEFT($4, 128),
			failed_at = $3
		WHERE id = $1 AND status = 'processing' AND claim_token = $2`
	tag, err := r.db.Exec(ctx, q, id, claimToken, failedAt, errorCode)
	if err != nil {
		return false, mapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

func boundOutboxErrorCode(errorCode string) string {
	runes := []rune(errorCode)
	if len(runes) <= maxOutboxErrorCodeCharacters {
		return errorCode
	}
	return string(runes[:maxOutboxErrorCodeCharacters])
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
		&e.ClaimToken, &e.ClaimOwner, &e.LeaseUntil, &e.LastErrorCode, &e.FailedAt,
	)
}
