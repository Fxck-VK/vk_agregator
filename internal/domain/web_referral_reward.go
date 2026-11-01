package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WebReferralRewardPolicy is the published account-native referral policy.
type WebReferralRewardPolicy struct {
	Version    int   `json:"version"`
	Stars      int64 `json:"stars"`
	DelayHours int   `json:"delay_hours"`
	Expires    bool  `json:"expires"`
}

func CurrentWebReferralRewardPolicy() WebReferralRewardPolicy {
	return WebReferralRewardPolicy{Version: 1, Stars: 50, DelayHours: 24, Expires: false}
}

type WebReferralPaymentMode string

const (
	WebReferralPaymentLive    WebReferralPaymentMode = "live"
	WebReferralPaymentTest    WebReferralPaymentMode = "test"
	WebReferralPaymentUnknown WebReferralPaymentMode = "unknown"
)

// Evidence is supplied only after an authenticated provider GET and settlement.
// Unknown proof blocks treating a subsequent payment as a first payment.
type WebReferralPaymentEvidence struct {
	IntentID          uuid.UUID
	AccountID         uuid.UUID
	Provider          string
	ProviderPaymentID string
	Amount            int64
	Currency          string
	VerifiedAt        time.Time
	Mode              WebReferralPaymentMode
}
type WebReferralRefundEvidence struct {
	IntentID          uuid.UUID
	Provider          string
	ProviderPaymentID string
	ProviderRefundID  string
	Amount            int64
	Currency          string
	VerifiedAt        time.Time
}

// WebReferralEvidenceRepository must be bound to the settlement transaction.
type WebReferralEvidenceRepository interface {
	RecordWebReferralPayment(context.Context, WebReferralPaymentEvidence) error
	RecordWebReferralRefund(context.Context, WebReferralRefundEvidence) error
}

// WebReferralRewardProcessor is shared by webhook and worker maintenance.
type WebReferralRewardProcessor interface {
	ProcessWebReferralRewards(context.Context, time.Time, int) (int64, error)
}
