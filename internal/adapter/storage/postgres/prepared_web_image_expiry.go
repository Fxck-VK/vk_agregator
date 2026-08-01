package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// PreparedWebImageExpiryRepository atomically claims and expires due browser
// image preparations. It is intentionally separate from JobRepository because
// expiry is a narrow maintenance operation, not a generally available browser
// job mutation.
type PreparedWebImageExpiryRepository struct {
	db Querier
}

// NewPreparedWebImageExpiryRepository builds an expiry repository over a pool
// or transaction-compatible querier.
func NewPreparedWebImageExpiryRepository(db Querier) *PreparedWebImageExpiryRepository {
	return &PreparedWebImageExpiryRepository{db: db}
}

// ExpireDuePreparedWebImages claims at most one bounded page with
// FOR UPDATE SKIP LOCKED, then transitions only those still-due rows. The
// single SQL statement is safe across independently running API instances and
// never emits queue, provider, delivery, billing, or outbox side effects.
func (r *PreparedWebImageExpiryRepository) ExpireDuePreparedWebImages(
	ctx context.Context,
	accountID *uuid.UUID,
	now time.Time,
	limit int,
) (int, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, errors.New("postgres: prepared web image expiry repository is required")
	}
	if limit <= 0 {
		return 0, false, errors.New("postgres: prepared web image expiry limit must be positive")
	}
	if accountID != nil && *accountID == uuid.Nil {
		return 0, false, nil
	}

	claimLimit := limit + 1
	var (
		query string
		args  []any
	)
	if accountID == nil {
		query = globalPreparedWebImageExpiryQuery
		args = []any{now, claimLimit, limit, domain.PreparedConfirmationExpiredCode, domain.PreparedConfirmationExpiredMessage}
	} else {
		query = accountPreparedWebImageExpiryQuery
		args = []any{*accountID, now, claimLimit, limit, domain.PreparedConfirmationExpiredCode, domain.PreparedConfirmationExpiredMessage}
	}
	return scanPreparedWebImageExpiryRows(ctx, r.db, query, args...)
}

// ExpireDuePreparedWebImage atomically transitions one exact account-owned
// row. It supports the browser read-after-page race without a global scan.
func (r *PreparedWebImageExpiryRepository) ExpireDuePreparedWebImage(
	ctx context.Context,
	accountID, jobID uuid.UUID,
	now time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("postgres: prepared web image expiry repository is required")
	}
	if accountID == uuid.Nil || jobID == uuid.Nil {
		return false, nil
	}
	rows, err := r.db.Query(ctx, exactPreparedWebImageExpiryQuery,
		jobID,
		accountID,
		now,
		domain.PreparedConfirmationExpiredCode,
		domain.PreparedConfirmationExpiredMessage,
	)
	if err != nil {
		return false, mapError(err)
	}
	defer rows.Close()
	changed := rows.Next()
	if changed {
		var returnedID uuid.UUID
		if err := rows.Scan(&returnedID); err != nil {
			return false, mapError(err)
		}
		if returnedID != jobID {
			return false, fmt.Errorf("postgres: prepared web image expiry returned unexpected job")
		}
	}
	if err := rows.Err(); err != nil {
		return false, mapError(err)
	}
	return changed, nil
}

func scanPreparedWebImageExpiryRows(ctx context.Context, db Querier, query string, args ...any) (int, bool, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return 0, false, mapError(err)
	}
	defer rows.Close()
	var (
		expired int
		hasMore bool
	)
	for rows.Next() {
		var (
			jobID *uuid.UUID
			more  bool
		)
		if err := rows.Scan(&jobID, &more); err != nil {
			return 0, false, mapError(err)
		}
		if more {
			hasMore = true
			continue
		}
		if jobID == nil || *jobID == uuid.Nil {
			return 0, false, errors.New("postgres: prepared web image expiry returned invalid job")
		}
		expired++
	}
	if err := rows.Err(); err != nil {
		return 0, false, mapError(err)
	}
	return expired, hasMore, nil
}

const globalPreparedWebImageExpiryQuery = `
	WITH candidates AS MATERIALIZED (
		SELECT id, expires_at
		FROM jobs
		WHERE account_id IS NOT NULL
		  AND source = 'web'
		  AND operation_type = 'image_generate'
		  AND modality = 'image'
		  AND status = 'prepared'
		  AND expires_at IS NOT NULL
		  AND expires_at <= $1
		ORDER BY expires_at ASC, id ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	), selected AS (
		SELECT id
		FROM candidates
		ORDER BY expires_at ASC, id ASC
		LIMIT $3
	), updated AS (
		UPDATE jobs AS j
		SET status = 'expired',
		    error_code = $4,
		    error_message = $5,
		    updated_at = now()
		FROM selected
		WHERE j.id = selected.id
		  AND j.status = 'prepared'
		  AND j.expires_at IS NOT NULL
		  AND j.expires_at <= $1
		RETURNING j.id
	)
	SELECT id, false AS has_more FROM updated
	UNION ALL
	SELECT NULL::uuid AS id, true AS has_more
	WHERE (SELECT count(*) FROM candidates) > $3`

const accountPreparedWebImageExpiryQuery = `
	WITH candidates AS MATERIALIZED (
		SELECT id, expires_at
		FROM jobs
		WHERE account_id = $1
		  AND source = 'web'
		  AND operation_type = 'image_generate'
		  AND modality = 'image'
		  AND status = 'prepared'
		  AND expires_at IS NOT NULL
		  AND expires_at <= $2
		ORDER BY expires_at ASC, id ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	), selected AS (
		SELECT id
		FROM candidates
		ORDER BY expires_at ASC, id ASC
		LIMIT $4
	), updated AS (
		UPDATE jobs AS j
		SET status = 'expired',
		    error_code = $5,
		    error_message = $6,
		    updated_at = now()
		FROM selected
		WHERE j.id = selected.id
		  AND j.status = 'prepared'
		  AND j.expires_at IS NOT NULL
		  AND j.expires_at <= $2
		RETURNING j.id
	)
	SELECT id, false AS has_more FROM updated
	UNION ALL
	SELECT NULL::uuid AS id, true AS has_more
	WHERE (SELECT count(*) FROM candidates) > $4`

const exactPreparedWebImageExpiryQuery = `
	UPDATE jobs
	SET status = 'expired',
	    error_code = $4,
	    error_message = $5,
	    updated_at = now()
	WHERE id = $1
	  AND account_id = $2
	  AND source = 'web'
	  AND operation_type = 'image_generate'
	  AND modality = 'image'
	  AND status = 'prepared'
	  AND expires_at IS NOT NULL
	  AND expires_at <= $3
	RETURNING id`
