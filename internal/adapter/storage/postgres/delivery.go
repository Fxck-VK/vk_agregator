package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// DeliveryRepository is the PostgreSQL implementation of
// domain.DeliveryRepository.
type DeliveryRepository struct {
	db Querier
}

// NewDeliveryRepository builds a DeliveryRepository over the given querier.
func NewDeliveryRepository(db Querier) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

var _ domain.DeliveryRepository = (*DeliveryRepository)(nil)

const deliveryColumns = `id, job_id, user_id, account_id, vk_peer_id, artifact_id,
	channel, recipient_ref, thread_ref, type, status, vk_random_id, vk_message_id, attachment, text, attempt_no, idempotency_key,
	error_code, error_message, created_at, updated_at`

// Create inserts a new delivery attempt.
func (r *DeliveryRepository) Create(ctx context.Context, d *domain.Delivery) error {
	if err := d.ValidateTarget(); err != nil {
		return err
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.AttemptNo == 0 {
		d.AttemptNo = 1
	}
	targetChannel, targetRecipientRef, targetThreadRef := deliveryTargetValues(d.Target)
	const q = `
		INSERT INTO deliveries (
			id, job_id, user_id, account_id, vk_peer_id, artifact_id,
			channel, recipient_ref, thread_ref, type, status, vk_random_id, vk_message_id, attachment, text, attempt_no, idempotency_key,
			error_code, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING ` + deliveryColumns
	row := r.db.QueryRow(ctx, q,
		d.ID, d.JobID, nullableUUID(d.UserID), nullableUUID(d.AccountID), nullableInt64(d.VKPeerID), d.ArtifactID,
		targetChannel, targetRecipientRef, targetThreadRef, d.Type, d.Status, nullableInt64(d.VKRandomID), d.VKMessageID, d.Attachment, d.Text, d.AttemptNo, d.IdempotencyKey,
		d.ErrorCode, d.ErrorMessage,
	)
	return mapError(scanDelivery(row, d))
}

// Update persists changes to a delivery attempt.
func (r *DeliveryRepository) Update(ctx context.Context, d *domain.Delivery) error {
	if err := d.ValidateTarget(); err != nil {
		return err
	}
	targetChannel, targetRecipientRef, targetThreadRef := deliveryTargetValues(d.Target)
	const q = `
		UPDATE deliveries
		SET account_id = COALESCE(account_id, $2),
		    status = $3, vk_message_id = $4, attachment = $5, text = $6, attempt_no = $7,
		    channel = $8, recipient_ref = $9, thread_ref = $10,
		    error_code = $11, error_message = $12, updated_at = now()
		WHERE id = $1
		RETURNING ` + deliveryColumns
	row := r.db.QueryRow(ctx, q,
		d.ID, nullableUUID(d.AccountID), d.Status, d.VKMessageID, d.Attachment, d.Text, d.AttemptNo,
		targetChannel, targetRecipientRef, targetThreadRef,
		d.ErrorCode, d.ErrorMessage,
	)
	return mapError(scanDelivery(row, d))
}

// GetByID fetches a delivery by id.
func (r *DeliveryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Delivery, error) {
	const q = `SELECT ` + deliveryColumns + ` FROM deliveries WHERE id = $1`
	var d domain.Delivery
	if err := mapError(scanDelivery(r.db.QueryRow(ctx, q, id), &d)); err != nil {
		return nil, err
	}
	return &d, nil
}

// GetByIdempotencyKey fetches a delivery by idempotency key for dedup.
func (r *DeliveryRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Delivery, error) {
	const q = `SELECT ` + deliveryColumns + ` FROM deliveries WHERE idempotency_key = $1`
	var d domain.Delivery
	if err := mapError(scanDelivery(r.db.QueryRow(ctx, q, key), &d)); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListByJob returns all delivery attempts for a job, oldest first.
func (r *DeliveryRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.Delivery, error) {
	const q = `SELECT ` + deliveryColumns + `
		FROM deliveries WHERE job_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, jobID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var deliveries []*domain.Delivery
	for rows.Next() {
		var d domain.Delivery
		if err := scanDelivery(rows, &d); err != nil {
			return nil, mapError(err)
		}
		deliveries = append(deliveries, &d)
	}
	return deliveries, mapError(rows.Err())
}

// HealthSnapshot returns safe delivery aggregates for operator screens.
func (r *DeliveryRepository) HealthSnapshot(ctx context.Context, since time.Time) (domain.DeliveryHealth, error) {
	const q = `
		SELECT
			count(*)::bigint,
			count(*) FILTER (WHERE status = $2)::bigint,
			count(*) FILTER (WHERE status = $3)::bigint,
			COALESCE((
				percentile_disc(0.95) WITHIN GROUP (
					ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000
				) FILTER (
					WHERE status IN ($4, $2) AND updated_at >= created_at
				)
			)::bigint, 0)::bigint
		FROM deliveries
		WHERE created_at >= $1`
	var health domain.DeliveryHealth
	if err := r.db.QueryRow(ctx, q,
		since,
		domain.DeliveryStatusFailed,
		domain.DeliveryStatusRetrying,
		domain.DeliveryStatusSent,
	).Scan(
		&health.TotalCount,
		&health.FailedCount,
		&health.RetryingCount,
		&health.LatencyP95Ms,
	); err != nil {
		return domain.DeliveryHealth{}, mapError(err)
	}

	const latestQ = `
		SELECT error_code, updated_at
		FROM deliveries
		WHERE created_at >= $1 AND error_code <> ''
		ORDER BY updated_at DESC
		LIMIT 1`
	var errorCode sql.NullString
	var errorAt sql.NullTime
	err := r.db.QueryRow(ctx, latestQ, since).Scan(&errorCode, &errorAt)
	if mapped := mapError(err); mapped != nil && !errors.Is(mapped, domain.ErrNotFound) {
		return domain.DeliveryHealth{}, mapped
	}
	if errorCode.Valid {
		health.LatestErrorCode = errorCode.String
	}
	if errorAt.Valid {
		at := errorAt.Time
		health.LatestErrorAt = &at
	}
	return health, nil
}

func scanDelivery(row rowScanner, d *domain.Delivery) error {
	var legacyUserID, accountID *uuid.UUID
	var legacyPeerID, legacyRandomID *int64
	var channel, recipientRef, threadRef *string
	if err := row.Scan(
		&d.ID, &d.JobID, &legacyUserID, &accountID, &legacyPeerID, &d.ArtifactID,
		&channel, &recipientRef, &threadRef, &d.Type, &d.Status, &legacyRandomID, &d.VKMessageID, &d.Attachment, &d.Text, &d.AttemptNo, &d.IdempotencyKey,
		&d.ErrorCode, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return err
	}
	d.UserID = uuid.Nil
	if legacyUserID != nil {
		d.UserID = *legacyUserID
	}
	d.AccountID = uuid.Nil
	if accountID != nil {
		d.AccountID = *accountID
	}
	d.VKPeerID = 0
	if legacyPeerID != nil {
		d.VKPeerID = *legacyPeerID
	}
	d.VKRandomID = 0
	if legacyRandomID != nil {
		d.VKRandomID = *legacyRandomID
	}
	d.Target = deliveryTargetFromColumns(channel, recipientRef, threadRef)
	return nil
}
