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

// ReferralStatus tracks the product funnel state for one accepted invitation.
type ReferralStatus string

const (
	// ReferralStatusRegistered means the referred user was linked to a code.
	ReferralStatusRegistered ReferralStatus = "registered"
	// ReferralStatusActivated means the referred user started using a VK surface.
	ReferralStatusActivated ReferralStatus = "activated"
	// ReferralStatusRewarded means configured ledger rewards were posted.
	ReferralStatusRewarded ReferralStatus = "rewarded"
)

// ReferralStats is the no-PII aggregate funnel view for a referrer's invites.
type ReferralStats struct {
	RegisteredCount int `json:"registered_count"`
	ActivatedCount  int `json:"activated_count"`
	RewardedCount   int `json:"rewarded_count"`
}

// Total returns all accepted invitations across the referral funnel.
func (s ReferralStats) Total() int {
	return s.RegisteredCount + s.ActivatedCount + s.RewardedCount
}

// ReferralCodeStats is a safe aggregate view keyed only by public referral code.
// It intentionally omits referrer/referred user identifiers and profile data.
type ReferralCodeStats struct {
	Code  string        `json:"code"`
	Stats ReferralStats `json:"stats"`
}

// ReferralSuspiciousFilter configures operator-only aggregate anomaly queries.
type ReferralSuspiciousFilter struct {
	MinRegistered int
	MinTotal      int
	Limit         int
}

// ReferralEventType describes future analytics events. MVP counters are derived
// from referrals.status; event rows are reserved for click/payment/generation
// analytics that do not expose invited-user PII.
type ReferralEventType string

const (
	ReferralEventLinkOpened      ReferralEventType = "link_opened"
	ReferralEventRegistered      ReferralEventType = "registered"
	ReferralEventActivated       ReferralEventType = "activated"
	ReferralEventRewarded        ReferralEventType = "rewarded"
	ReferralEventFirstGeneration ReferralEventType = "first_generation"
	ReferralEventFirstPayment    ReferralEventType = "first_payment"
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
	Status         ReferralStatus       `json:"status"`
	RewardStatus   ReferralRewardStatus `json:"reward_status"`
	FirstSeenAt    time.Time            `json:"first_seen_at"`
	ActivatedAt    *time.Time           `json:"activated_at,omitempty"`
	RewardedAt     *time.Time           `json:"rewarded_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
