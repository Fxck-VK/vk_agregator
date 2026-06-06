package domain

import (
	"time"

	"github.com/google/uuid"
)

// ReferralSource identifies which VK surface accepted a referral code.
type ReferralSource string

const (
	// ReferralSourceVKBot is a referral applied through the VK bot dialog.
	ReferralSourceVKBot ReferralSource = "vk_bot"
	// ReferralSourceVKMiniApp is reserved for the VK Mini App BFF.
	ReferralSourceVKMiniApp ReferralSource = "vk_miniapp"
)

// ReferralRewardStatus tracks whether signup referral rewards have been posted
// to the append-only billing ledger.
type ReferralRewardStatus string

const (
	// ReferralRewardPending means the relation exists but reward ledger entries
	// have not been confirmed yet.
	ReferralRewardPending ReferralRewardStatus = "pending"
	// ReferralRewardApplied means all configured signup reward ledger entries
	// have been created idempotently.
	ReferralRewardApplied ReferralRewardStatus = "applied"
)

// ReferralCode is the single stable invitation code owned by one user.
type ReferralCode struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Referral records that one user joined through another user's referral code.
type Referral struct {
	ID             uuid.UUID            `json:"id"`
	ReferrerUserID uuid.UUID            `json:"referrer_user_id"`
	ReferredUserID uuid.UUID            `json:"referred_user_id"`
	ReferralCode   string               `json:"referral_code"`
	Source         ReferralSource       `json:"source"`
	RewardStatus   ReferralRewardStatus `json:"reward_status"`
	RewardedAt     *time.Time           `json:"rewarded_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
