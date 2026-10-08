package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/webreferralservice"
)

type webReferralBeginQuerier interface {
	Querier
	Begin(context.Context) (pgx.Tx, error)
}

// WebReferralRepository stores account-native web referral codes and visits.
type WebReferralRepository struct {
	db webReferralBeginQuerier
}

// NewWebReferralRepository builds a WebReferralRepository over a pool or
// transaction-capable pgx connection.
func NewWebReferralRepository(db webReferralBeginQuerier) *WebReferralRepository {
	return &WebReferralRepository{db: db}
}

var _ webreferralservice.Repository = (*WebReferralRepository)(nil)

// ErrWebReferralCleanupRequiresTransactions is returned when maintenance is
// wired with a querier that cannot open the cleanup transaction.
var ErrWebReferralCleanupRequiresTransactions = errors.New("postgres: web referral cleanup requires transaction-capable database")

const webReferralCodeColumns = `id, account_id, code, created_at, updated_at`

type expiredWebReferralVisitCleaner interface {
	CleanupExpiredWebReferralVisits(context.Context, time.Time, int) (int64, error)
}

var _ expiredWebReferralVisitCleaner = (*MaintenanceRepository)(nil)

func (r *WebReferralRepository) CodeByAccountID(ctx context.Context, accountID uuid.UUID) (*domain.WebReferralCode, error) {
	const q = `
		SELECT rc.id, rc.account_id, rc.code, rc.created_at, rc.updated_at
		FROM referral_codes rc
		INNER JOIN accounts a ON a.id = rc.account_id
		WHERE rc.account_id = $1
		  AND a.status = 'active'`
	var code domain.WebReferralCode
	if err := mapError(scanWebReferralCode(r.db.QueryRow(ctx, q, accountID), &code)); err != nil {
		return nil, err
	}
	return &code, nil
}

func (r *WebReferralRepository) CreateCode(ctx context.Context, code *domain.WebReferralCode) error {
	if code.ID == uuid.Nil {
		code.ID = uuid.New()
	}
	const q = `
		INSERT INTO referral_codes (id, user_id, account_id, code)
		SELECT $1, NULL, a.id, $3
		FROM accounts a
		WHERE a.id = $2
		  AND a.status = 'active'
		RETURNING ` + webReferralCodeColumns
	return mapError(scanWebReferralCode(r.db.QueryRow(ctx, q, code.ID, code.AccountID, code.Code), code))
}

func (r *WebReferralRepository) Summary(ctx context.Context, accountID uuid.UUID) (webreferralservice.Summary, error) {
	const q = `
		WITH referral_counts AS (
			SELECT
				COUNT(*)::int AS registered,
				COALESCE(SUM(CASE
					WHEN status IN ('activated', 'rewarded') OR reward_status = 'applied'
					THEN 1 ELSE 0
				END), 0)::int AS activated,
				COALESCE(SUM(CASE
					WHEN status = 'rewarded' OR reward_status = 'applied'
					THEN 1 ELSE 0
				END), 0)::int AS rewarded
			FROM referrals
			WHERE referrer_account_id = $1
			  AND source = 'web'
		), visit_counts AS (
			SELECT COUNT(*)::int AS visits
			FROM web_referral_visits
			WHERE referrer_account_id = $1
		), retained_visit_counts AS (
			SELECT COALESCE(visits_total, 0)::int AS visits
			FROM web_referral_visit_aggregates
			WHERE referrer_account_id = $1
		)
		SELECT
			visit_counts.visits + COALESCE(retained_visit_counts.visits, 0),
			referral_counts.registered,
			referral_counts.activated,
			referral_counts.rewarded
		FROM referral_counts
		CROSS JOIN visit_counts
		LEFT JOIN retained_visit_counts ON true`
	var summary webreferralservice.Summary
	if err := r.db.QueryRow(ctx, q, accountID).Scan(&summary.Visits, &summary.Registered, &summary.Activated, &summary.Rewarded); err != nil {
		return webreferralservice.Summary{}, mapError(err)
	}
	return summary, nil
}

func (r *WebReferralRepository) Capture(ctx context.Context, visit domain.WebReferralVisit) error {
	var existingExpiresAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT expires_at
		FROM web_referral_visits
		WHERE token_hash = $1`, visit.TokenHash).Scan(&existingExpiresAt)
	if err == nil {
		if !visit.CreatedAt.Before(existingExpiresAt) {
			return domain.ErrExpired
		}
		return nil
	}
	if !errors.Is(mapError(err), domain.ErrNotFound) {
		return mapError(err)
	}

	var referrerAccountID uuid.UUID
	if err := r.db.QueryRow(ctx, `
		SELECT rc.account_id
		FROM referral_codes rc
		INNER JOIN accounts a ON a.id = rc.account_id
		WHERE rc.code = $1
		  AND a.status = 'active'`, visit.Code).Scan(&referrerAccountID); err != nil {
		return mapError(err)
	}
	tag, err := r.db.Exec(ctx, `
		INSERT INTO web_referral_visits (token_hash, code, referrer_account_id, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (token_hash) DO NOTHING`,
		visit.TokenHash,
		visit.Code,
		referrerAccountID,
		visit.CreatedAt,
		visit.ExpiresAt,
	)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		err := r.db.QueryRow(ctx, `
			SELECT expires_at
			FROM web_referral_visits
			WHERE token_hash = $1`, visit.TokenHash).Scan(&existingExpiresAt)
		if err != nil {
			return mapError(err)
		}
		if !visit.CreatedAt.Before(existingExpiresAt) {
			return domain.ErrExpired
		}
	}
	return nil
}

func (r *WebReferralRepository) Accept(ctx context.Context, tokenHash string, accountID uuid.UUID, now time.Time) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	visit, err := lockWebReferralVisit(ctx, tx, tokenHash)
	if err != nil {
		return err
	}
	if !now.Before(visit.ExpiresAt) {
		return domain.ErrExpired
	}
	if visit.AcceptedAccountID != uuid.Nil {
		if visit.AcceptedAccountID == accountID {
			return mapError(tx.Commit(ctx))
		}
		return domain.ErrConflict
	}

	var accountCreatedAt time.Time
	if err := tx.QueryRow(ctx, `
		SELECT created_at
		FROM accounts
		WHERE id = $1
		  AND status = 'active'
		FOR UPDATE`, accountID).Scan(&accountCreatedAt); err != nil {
		return mapError(err)
	}
	if accountID == visit.ReferrerAccountID || accountCreatedAt.Before(visit.CreatedAt) {
		return domain.ErrForbidden
	}

	existingReferrer, err := existingReferralReferrer(ctx, tx, accountID)
	if err == nil {
		if existingReferrer == visit.ReferrerAccountID {
			if err := markWebReferralVisitAccepted(ctx, tx, tokenHash, accountID); err != nil {
				return err
			}
			return mapError(tx.Commit(ctx))
		}
		return domain.ErrConflict
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	var insertedID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO referrals (
			id, referrer_user_id, referrer_account_id, referred_user_id, referred_account_id,
			referral_code, source, status, reward_status, first_seen_at, created_at, updated_at
		) VALUES (
			$1, NULL, $2, NULL, $3,
			$4, $5, $6, $7, $8, $9, $9
		)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		uuid.New(),
		visit.ReferrerAccountID,
		accountID,
		visit.Code,
		domain.ReferralSourceWeb,
		domain.ReferralStatusRegistered,
		domain.ReferralRewardPending,
		visit.CreatedAt,
		now,
	).Scan(&insertedID)
	if errors.Is(err, pgx.ErrNoRows) {
		existingReferrer, err := existingReferralReferrer(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if existingReferrer != visit.ReferrerAccountID {
			return domain.ErrConflict
		}
	} else if err != nil {
		return mapError(err)
	}

	if err := markWebReferralVisitAccepted(ctx, tx, tokenHash, accountID); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

// CleanupExpiredWebReferralVisits deletes expired opaque visit token hashes in a
// bounded batch while retaining cumulative per-referrer visit totals.
func (r *WebReferralRepository) CleanupExpiredWebReferralVisits(ctx context.Context, now time.Time, limit int) (deleted int64, err error) {
	if limit <= 0 {
		limit = 1000
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, mapError(err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	const q = `
		WITH expired AS (
			SELECT token_hash, referrer_account_id
			FROM web_referral_visits
			WHERE expires_at <= $1
			ORDER BY expires_at, token_hash
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		),
		deleted AS (
			DELETE FROM web_referral_visits v
			USING expired e
			WHERE v.token_hash = e.token_hash
			RETURNING e.referrer_account_id
		),
		aggregated AS (
			SELECT referrer_account_id, COUNT(*)::bigint AS visits
			FROM deleted
			GROUP BY referrer_account_id
		),
		retained AS (
			INSERT INTO web_referral_visit_aggregates (referrer_account_id, visits_total, updated_at)
			SELECT referrer_account_id, visits, $1
			FROM aggregated
			ON CONFLICT (referrer_account_id) DO UPDATE
			SET visits_total = web_referral_visit_aggregates.visits_total + EXCLUDED.visits_total,
				updated_at = EXCLUDED.updated_at
			RETURNING visits_total
		)
		SELECT COALESCE(SUM(visits), 0)::bigint
		FROM aggregated`
	if err := tx.QueryRow(ctx, q, now, limit).Scan(&deleted); err != nil {
		return 0, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, mapError(err)
	}
	return deleted, nil
}

// CleanupExpiredWebReferralVisits exposes web referral visit cleanup through
// the production maintenance repository used by the worker maintenance loop.
func (r *MaintenanceRepository) CleanupExpiredWebReferralVisits(ctx context.Context, now time.Time, limit int) (int64, error) {
	db, ok := r.db.(webReferralBeginQuerier)
	if !ok {
		return 0, ErrWebReferralCleanupRequiresTransactions
	}
	return NewWebReferralRepository(db).CleanupExpiredWebReferralVisits(ctx, now, limit)
}

type webReferralVisitRow struct {
	TokenHash         string
	Code              string
	ReferrerAccountID uuid.UUID
	CreatedAt         time.Time
	ExpiresAt         time.Time
	AcceptedAccountID uuid.UUID
}

func lockWebReferralVisit(ctx context.Context, tx pgx.Tx, tokenHash string) (webReferralVisitRow, error) {
	const q = `
		SELECT token_hash, code, referrer_account_id, created_at, expires_at, accepted_account_id
		FROM web_referral_visits
		WHERE token_hash = $1
		FOR UPDATE`
	var row webReferralVisitRow
	var acceptedAccountID *uuid.UUID
	err := tx.QueryRow(ctx, q, tokenHash).Scan(
		&row.TokenHash,
		&row.Code,
		&row.ReferrerAccountID,
		&row.CreatedAt,
		&row.ExpiresAt,
		&acceptedAccountID,
	)
	if err != nil {
		return webReferralVisitRow{}, mapError(err)
	}
	if acceptedAccountID != nil {
		row.AcceptedAccountID = *acceptedAccountID
	}
	return row, nil
}

func existingReferralReferrer(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (uuid.UUID, error) {
	var referrerAccountID *uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT referrer_account_id
		FROM referrals
		WHERE referred_account_id = $1
		FOR UPDATE`, accountID).Scan(&referrerAccountID)
	if err != nil {
		return uuid.Nil, mapError(err)
	}
	if referrerAccountID == nil {
		return uuid.Nil, domain.ErrConflict
	}
	return *referrerAccountID, nil
}

func markWebReferralVisitAccepted(ctx context.Context, tx pgx.Tx, tokenHash string, accountID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE web_referral_visits
		SET accepted_account_id = $2
		WHERE token_hash = $1
		  AND (accepted_account_id IS NULL OR accepted_account_id = $2)`, tokenHash, accountID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	return nil
}

func scanWebReferralCode(row rowScanner, code *domain.WebReferralCode) error {
	return row.Scan(&code.ID, &code.AccountID, &code.Code, &code.CreatedAt, &code.UpdatedAt)
}
