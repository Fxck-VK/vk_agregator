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

const deliveryColumns = `id, job_id, user_id, vk_peer_id, artifact_id, type, status,
	vk_random_id, vk_message_id, attachment, text, attempt_no, idempotency_key,
	error_code, error_message, created_at, updated_at`

// Create inserts a new delivery attempt.
func (r *DeliveryRepository) Create(ctx context.Context, d *domain.Delivery) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.AttemptNo == 0 {
		d.AttemptNo = 1
	}
	const q = `
		INSERT INTO deliveries (
			id, job_id, user_id, vk_peer_id, artifact_id, type, status,
			vk_random_id, vk_message_id, attachment, text, attempt_no, idempotency_key,
			error_code, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING ` + deliveryColumns
	row := r.db.QueryRow(ctx, q,
		d.ID, d.JobID, d.UserID, d.VKPeerID, d.ArtifactID, d.Type, d.Status,
		d.VKRandomID, d.VKMessageID, d.Attachment, d.Text, d.AttemptNo, d.IdempotencyKey,
		d.ErrorCode, d.ErrorMessage,
	)
	return mapError(scanDelivery(row, d))
}

// Update persists changes to a delivery attempt.
func (r *DeliveryRepository) Update(ctx context.Context, d *domain.Delivery) error {
	const q = `
		UPDATE deliveries
		SET status = $2, vk_message_id = $3, attachment = $4, text = $5, attempt_no = $6,
		    error_code = $7, error_message = $8, updated_at = now()
		WHERE id = $1
		RETURNING ` + deliveryColumns
	row := r.db.QueryRow(ctx, q,
		d.ID, d.Status, d.VKMessageID, d.Attachment, d.Text, d.AttemptNo,
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
	return row.Scan(
		&d.ID, &d.JobID, &d.UserID, &d.VKPeerID, &d.ArtifactID, &d.Type, &d.Status,
		&d.VKRandomID, &d.VKMessageID, &d.Attachment, &d.Text, &d.AttemptNo, &d.IdempotencyKey,
		&d.ErrorCode, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
}
