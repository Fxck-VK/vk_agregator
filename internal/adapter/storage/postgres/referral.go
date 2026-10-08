package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ReferralRepository is the PostgreSQL implementation of domain.ReferralRepository.
type ReferralRepository struct {
	db Querier
}

// NewReferralRepository builds a ReferralRepository over the given querier.
func NewReferralRepository(db Querier) *ReferralRepository {
	return &ReferralRepository{db: db}
}

var _ domain.ReferralRepository = (*ReferralRepository)(nil)

const referralCodeColumns = `id, user_id, account_id, code, created_at, updated_at`

const referralColumns = `id, referrer_user_id, referrer_account_id, referred_user_id, referred_account_id, referral_code,
	source, status, reward_status, first_seen_at, activated_at, rewarded_at, created_at, updated_at`

func (r *ReferralRepository) GetCodeByUserID(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, error) {
	const q = `
		SELECT rc.id, rc.user_id, rc.account_id, rc.code, rc.created_at, rc.updated_at
		FROM referral_codes rc
		LEFT JOIN users u ON u.id = $1
		WHERE rc.user_id = $1
		   OR rc.account_id = $1
		   OR (u.account_id IS NOT NULL AND rc.account_id = u.account_id)
		ORDER BY CASE
			WHEN rc.user_id = $1 THEN 0
			WHEN u.account_id IS NOT NULL AND rc.account_id = u.account_id THEN 1
			ELSE 2
		END
		LIMIT 1`
	var code domain.ReferralCode
	if err := mapError(scanReferralCode(r.db.QueryRow(ctx, q, userID), &code)); err != nil {
		return nil, err
	}
	return &code, nil
}

func (r *ReferralRepository) GetCode(ctx context.Context, codeValue string) (*domain.ReferralCode, error) {
	const q = `SELECT ` + referralCodeColumns + ` FROM referral_codes WHERE code = $1`
	var code domain.ReferralCode
	if err := mapError(scanReferralCode(r.db.QueryRow(ctx, q, codeValue), &code)); err != nil {
		return nil, err
	}
	return &code, nil
}

func (r *ReferralRepository) CreateCode(ctx context.Context, code *domain.ReferralCode) error {
	if code.ID == uuid.Nil {
		code.ID = uuid.New()
	}
	const q = `
		INSERT INTO referral_codes (id, user_id, account_id, code)
		VALUES ($1, $2, COALESCE($3::uuid, (SELECT account_id FROM users WHERE id = $2)), $4)
		RETURNING ` + referralCodeColumns
	return mapError(scanReferralCode(r.db.QueryRow(ctx, q, code.ID, code.UserID, nullableUUID(code.AccountID), code.Code), code))
}

func (r *ReferralRepository) CreateReferral(ctx context.Context, referral *domain.Referral) error {
	if referral.ID == uuid.Nil {
		referral.ID = uuid.New()
	}
	if referral.Source == "" {
		referral.Source = domain.ReferralSourceVKBot
	}
	if referral.RewardStatus == "" {
		referral.RewardStatus = domain.ReferralRewardPending
	}
	if referral.Status == "" {
		referral.Status = domain.ReferralStatusRegistered
	}
	const q = `
		INSERT INTO referrals (
			id, referrer_user_id, referrer_account_id, referred_user_id, referred_account_id,
			referral_code, source, status, reward_status
		) VALUES (
			$1,
			$2::uuid,
			COALESCE($3::uuid, (SELECT account_id FROM users WHERE id = $2)),
			$4::uuid,
			COALESCE($5::uuid, (SELECT account_id FROM users WHERE id = $4)),
			$6, $7, $8, $9
		)
		RETURNING ` + referralColumns
	return mapError(scanReferral(r.db.QueryRow(ctx, q,
		referral.ID,
		nullableUUID(referral.ReferrerUserID),
		nullableUUID(referral.ReferrerAccountID),
		nullableUUID(referral.ReferredUserID),
		nullableUUID(referral.ReferredAccountID),
		referral.ReferralCode,
		referral.Source,
		referral.Status,
		referral.RewardStatus,
	), referral))
}

func (r *ReferralRepository) GetReferralByReferredUserID(ctx context.Context, userID uuid.UUID) (*domain.Referral, error) {
	const q = `
		SELECT r.id, r.referrer_user_id, r.referrer_account_id, r.referred_user_id, r.referred_account_id,
			r.referral_code, r.source, r.status, r.reward_status, r.first_seen_at,
			r.activated_at, r.rewarded_at, r.created_at, r.updated_at
		FROM referrals r
		LEFT JOIN users u ON u.id = $1
		WHERE r.referred_user_id = $1
		   OR r.referred_account_id = $1
		   OR (u.account_id IS NOT NULL AND r.referred_account_id = u.account_id)
		LIMIT 1`
	var referral domain.Referral
	if err := mapError(scanReferral(r.db.QueryRow(ctx, q, userID), &referral)); err != nil {
		return nil, err
	}
	return &referral, nil
}

func (r *ReferralRepository) CountByReferrer(ctx context.Context, referrerUserID uuid.UUID) (int, error) {
	const q = `
		SELECT count(DISTINCT r.id)
		FROM referrals r
		LEFT JOIN users u ON u.id = $1
		WHERE r.referrer_user_id = $1
		   OR r.referrer_account_id = $1
		   OR (u.account_id IS NOT NULL AND r.referrer_account_id = u.account_id)`
	var count int
	if err := r.db.QueryRow(ctx, q, referrerUserID).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func (r *ReferralRepository) CountByReferrerStatus(ctx context.Context, referrerUserID uuid.UUID) (domain.ReferralStats, error) {
	const q = `
		SELECT status, reward_status, count(*)
		FROM (
			SELECT DISTINCT r.id, r.status, r.reward_status
			FROM referrals r
			LEFT JOIN users u ON u.id = $1
			WHERE r.referrer_user_id = $1
			   OR r.referrer_account_id = $1
			   OR (u.account_id IS NOT NULL AND r.referrer_account_id = u.account_id)
		) matched
		GROUP BY status, reward_status`
	rows, err := r.db.Query(ctx, q, referrerUserID)
	if err != nil {
		return domain.ReferralStats{}, mapError(err)
	}
	defer rows.Close()

	var stats domain.ReferralStats
	for rows.Next() {
		var status domain.ReferralStatus
		var rewardStatus domain.ReferralRewardStatus
		var count int
		if err := rows.Scan(&status, &rewardStatus, &count); err != nil {
			return domain.ReferralStats{}, mapError(err)
		}
		switch status {
		case domain.ReferralStatusRewarded:
			stats.RewardedCount += count
		case domain.ReferralStatusActivated:
			stats.ActivatedCount += count
		default:
			if rewardStatus == domain.ReferralRewardApplied {
				stats.RewardedCount += count
			} else {
				stats.RegisteredCount += count
			}
		}
	}
	if err := rows.Err(); err != nil {
		return domain.ReferralStats{}, mapError(err)
	}
	return stats, nil
}

func (r *ReferralRepository) StatsByReferralCode(ctx context.Context, code string) (domain.ReferralCodeStats, error) {
	if _, err := r.GetCode(ctx, code); err != nil {
		return domain.ReferralCodeStats{}, err
	}
	const q = `
		SELECT
			COALESCE(SUM(CASE
				WHEN reward_status <> 'applied'
				 AND (status IS NULL OR status NOT IN ('activated', 'rewarded'))
				THEN 1 ELSE 0
			END), 0)::int AS registered_count,
			COALESCE(SUM(CASE
				WHEN status = 'activated' AND reward_status <> 'applied'
				THEN 1 ELSE 0
			END), 0)::int AS activated_count,
			COALESCE(SUM(CASE
				WHEN status = 'rewarded' OR reward_status = 'applied'
				THEN 1 ELSE 0
			END), 0)::int AS rewarded_count
		FROM referrals
		WHERE referral_code = $1`
	var stats domain.ReferralStats
	if err := r.db.QueryRow(ctx, q, code).Scan(&stats.RegisteredCount, &stats.ActivatedCount, &stats.RewardedCount); err != nil {
		return domain.ReferralCodeStats{}, mapError(err)
	}
	return domain.ReferralCodeStats{Code: code, Stats: stats}, nil
}

func (r *ReferralRepository) ListSuspiciousReferralCodes(ctx context.Context, filter domain.ReferralSuspiciousFilter) ([]domain.ReferralCodeStats, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	const q = `
		WITH aggregated AS (
			SELECT
				referral_code,
				COALESCE(SUM(CASE
					WHEN reward_status <> 'applied'
					 AND (status IS NULL OR status NOT IN ('activated', 'rewarded'))
					THEN 1 ELSE 0
				END), 0)::int AS registered_count,
				COALESCE(SUM(CASE
					WHEN status = 'activated' AND reward_status <> 'applied'
					THEN 1 ELSE 0
				END), 0)::int AS activated_count,
				COALESCE(SUM(CASE
					WHEN status = 'rewarded' OR reward_status = 'applied'
					THEN 1 ELSE 0
				END), 0)::int AS rewarded_count
			FROM referrals
			GROUP BY referral_code
		)
		SELECT referral_code, registered_count, activated_count, rewarded_count
		FROM aggregated
		WHERE registered_count >= $1
		   OR (registered_count + activated_count + rewarded_count) >= $2
		ORDER BY (registered_count + activated_count + rewarded_count) DESC,
			registered_count DESC,
			referral_code ASC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, filter.MinRegistered, filter.MinTotal, filter.Limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []domain.ReferralCodeStats
	for rows.Next() {
		var item domain.ReferralCodeStats
		if err := rows.Scan(&item.Code, &item.Stats.RegisteredCount, &item.Stats.ActivatedCount, &item.Stats.RewardedCount); err != nil {
			return nil, mapError(err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (r *ReferralRepository) ReferralStatusDistribution(ctx context.Context) (domain.ReferralStats, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE
				WHEN reward_status <> 'applied'
				 AND (status IS NULL OR status NOT IN ('activated', 'rewarded'))
				THEN 1 ELSE 0
			END), 0)::int AS registered_count,
			COALESCE(SUM(CASE
				WHEN status = 'activated' AND reward_status <> 'applied'
				THEN 1 ELSE 0
			END), 0)::int AS activated_count,
			COALESCE(SUM(CASE
				WHEN status = 'rewarded' OR reward_status = 'applied'
				THEN 1 ELSE 0
			END), 0)::int AS rewarded_count
		FROM referrals`
	var stats domain.ReferralStats
	if err := r.db.QueryRow(ctx, q).Scan(&stats.RegisteredCount, &stats.ActivatedCount, &stats.RewardedCount); err != nil {
		return domain.ReferralStats{}, mapError(err)
	}
	return stats, nil
}

func (r *ReferralRepository) MarkActivated(ctx context.Context, referralID uuid.UUID, activatedAt time.Time) error {
	const q = `
		UPDATE referrals
		SET status = CASE
				WHEN status = $2 THEN $3
				ELSE status
			END,
			activated_at = COALESCE(activated_at, $4),
			updated_at = now()
		WHERE id = $1
		  AND status IN ($2, $3)`
	tag, err := r.db.Exec(ctx, q,
		referralID,
		domain.ReferralStatusRegistered,
		domain.ReferralStatusActivated,
		activatedAt,
	)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ReferralRepository) MarkRewardApplied(ctx context.Context, referralID uuid.UUID, rewardedAt time.Time) error {
	const q = `
		UPDATE referrals
		SET status = $2,
			reward_status = $3,
			activated_at = COALESCE(activated_at, $4),
			rewarded_at = $4,
			updated_at = now()
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, q,
		referralID,
		domain.ReferralStatusRewarded,
		domain.ReferralRewardApplied,
		rewardedAt,
	)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanReferralCode(row rowScanner, code *domain.ReferralCode) error {
	var userID, accountID *uuid.UUID
	if err := row.Scan(&code.ID, &userID, &accountID, &code.Code, &code.CreatedAt, &code.UpdatedAt); err != nil {
		return err
	}
	if userID != nil {
		code.UserID = *userID
	}
	if accountID != nil {
		code.AccountID = *accountID
	}
	return nil
}

func scanReferral(row rowScanner, referral *domain.Referral) error {
	var referrerUserID, referrerAccountID, referredUserID, referredAccountID *uuid.UUID
	if err := row.Scan(
		&referral.ID,
		&referrerUserID,
		&referrerAccountID,
		&referredUserID,
		&referredAccountID,
		&referral.ReferralCode,
		&referral.Source,
		&referral.Status,
		&referral.RewardStatus,
		&referral.FirstSeenAt,
		&referral.ActivatedAt,
		&referral.RewardedAt,
		&referral.CreatedAt,
		&referral.UpdatedAt,
	); err != nil {
		return err
	}
	if referrerUserID != nil {
		referral.ReferrerUserID = *referrerUserID
	}
	if referrerAccountID != nil {
		referral.ReferrerAccountID = *referrerAccountID
	}
	if referredUserID != nil {
		referral.ReferredUserID = *referredUserID
	}
	if referredAccountID != nil {
		referral.ReferredAccountID = *referredAccountID
	}
	return nil
}
