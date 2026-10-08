package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrForbidden is returned when the caller is authenticated but the requested
// operation is not allowed for that account.
var ErrForbidden = errors.New("domain: forbidden")

// ErrExpired is returned when a time-limited token or proof is no longer valid.
var ErrExpired = errors.New("domain: expired")

// ErrInvalidInput is returned when caller-supplied input fails validation before
// an ownership check can be evaluated.
var ErrInvalidInput = errors.New("domain: invalid input")

// ReferralSourceWeb records account-native referrals accepted from the web app.
const ReferralSourceWeb ReferralSource = "web"

// WebReferralCode is the account-owned public invitation code used by web
// referrals. Legacy user_id may be absent for account-native rows.
type WebReferralCode struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebReferralVisit stores an opaque referral cookie token hash. It intentionally
// carries no raw token, IP address, user agent or other visitor PII.
type WebReferralVisit struct {
	TokenHash         string    `json:"token_hash"`
	Code              string    `json:"code"`
	ReferrerAccountID uuid.UUID `json:"referrer_account_id"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	AcceptedAccountID uuid.UUID `json:"accepted_account_id,omitempty"`
}
