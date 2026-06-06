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

const referralCodeColumns = `id, user_id, code, created_at, updated_at`

const referralColumns = `id, referrer_user_id, referred_user_id, referral_code,
	source, reward_status, rewarded_at, created_at, updated_at`

func (r *ReferralRepository) GetCodeByUserID(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, error) {
	const q = `SELECT ` + referralCodeColumns + ` FROM referral_codes WHERE user_id = $1`
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
		INSERT INTO referral_codes (id, user_id, code)
		VALUES ($1, $2, $3)
		RETURNING ` + referralCodeColumns
	return mapError(scanReferralCode(r.db.QueryRow(ctx, q, code.ID, code.UserID, code.Code), code))
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
	const q = `
		INSERT INTO referrals (
			id, referrer_user_id, referred_user_id, referral_code, source, reward_status
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
		RETURNING ` + referralColumns
	return mapError(scanReferral(r.db.QueryRow(ctx, q,
		referral.ID,
		referral.ReferrerUserID,
		referral.ReferredUserID,
		referral.ReferralCode,
		referral.Source,
		referral.RewardStatus,
	), referral))
}

func (r *ReferralRepository) GetReferralByReferredUserID(ctx context.Context, userID uuid.UUID) (*domain.Referral, error) {
	const q = `SELECT ` + referralColumns + ` FROM referrals WHERE referred_user_id = $1`
	var referral domain.Referral
	if err := mapError(scanReferral(r.db.QueryRow(ctx, q, userID), &referral)); err != nil {
		return nil, err
	}
	return &referral, nil
}

func (r *ReferralRepository) CountByReferrer(ctx context.Context, referrerUserID uuid.UUID) (int, error) {
	const q = `SELECT count(*) FROM referrals WHERE referrer_user_id = $1`
	var count int
	if err := r.db.QueryRow(ctx, q, referrerUserID).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func (r *ReferralRepository) MarkRewardApplied(ctx context.Context, referralID uuid.UUID, rewardedAt time.Time) error {
	const q = `
		UPDATE referrals
		SET reward_status = $2, rewarded_at = $3, updated_at = now()
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, referralID, domain.ReferralRewardApplied, rewardedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanReferralCode(row rowScanner, code *domain.ReferralCode) error {
	return row.Scan(&code.ID, &code.UserID, &code.Code, &code.CreatedAt, &code.UpdatedAt)
}

func scanReferral(row rowScanner, referral *domain.Referral) error {
	return row.Scan(
		&referral.ID,
		&referral.ReferrerUserID,
		&referral.ReferredUserID,
		&referral.ReferralCode,
		&referral.Source,
		&referral.RewardStatus,
		&referral.RewardedAt,
		&referral.CreatedAt,
		&referral.UpdatedAt,
	)
}
