package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// InboundEventRepository is the PostgreSQL implementation of
// domain.InboundEventRepository.
type InboundEventRepository struct {
	db Querier
}

// NewInboundEventRepository builds an InboundEventRepository over the querier.
func NewInboundEventRepository(db Querier) *InboundEventRepository {
	return &InboundEventRepository{db: db}
}

var _ domain.InboundEventRepository = (*InboundEventRepository)(nil)

const inboundColumns = `id, source, event_type, group_id, vk_event_id, peer_id, vk_user_id,
	payload, status, idempotency_key, created_at, updated_at`

// Create inserts a new inbound event.
func (r *InboundEventRepository) Create(ctx context.Context, e *domain.InboundEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if len(e.Payload) == 0 {
		e.Payload = []byte("{}")
	}
	if e.Status == "" {
		e.Status = domain.InboundReceived
	}
	const q = `
		INSERT INTO inbound_events (
			id, source, event_type, group_id, vk_event_id, peer_id, vk_user_id,
			payload, status, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + inboundColumns
	row := r.db.QueryRow(ctx, q,
		e.ID, e.Source, e.EventType, e.GroupID, e.VKEventID, e.PeerID, e.VKUserID,
		[]byte(e.Payload), e.Status, e.IdempotencyKey,
	)
	return mapError(scanInbound(row, e))
}

// GetByIdempotencyKey fetches an event by idempotency key.
func (r *InboundEventRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.InboundEvent, error) {
	const q = `SELECT ` + inboundColumns + ` FROM inbound_events WHERE idempotency_key = $1`
	var e domain.InboundEvent
	if err := mapError(scanInbound(r.db.QueryRow(ctx, q, key), &e)); err != nil {
		return nil, err
	}
	return &e, nil
}

// SetStatus updates the processing status of an event.
func (r *InboundEventRepository) SetStatus(ctx context.Context, id uuid.UUID, status domain.InboundEventStatus) error {
	const q = `UPDATE inbound_events SET status = $2, updated_at = now() WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanInbound(row rowScanner, e *domain.InboundEvent) error {
	return row.Scan(
		&e.ID, &e.Source, &e.EventType, &e.GroupID, &e.VKEventID, &e.PeerID, &e.VKUserID,
		&e.Payload, &e.Status, &e.IdempotencyKey, &e.CreatedAt, &e.UpdatedAt,
	)
}
