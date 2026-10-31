package domain

import (
	"context"
	"github.com/google/uuid"
	"time"
)

// SubscriptionPlanSnapshot is immutable purchase-time commercial data.
type SubscriptionPlanSnapshot struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Amount        int64  `json:"amount"`
	Stars         int64  `json:"stars"`
	PeriodDays    int    `json:"period_days"`
	Trial         bool   `json:"trial"`
	NextPlanCode  string `json:"next_plan_code"`
	PolicyVersion int    `json:"policy_version"`
}

// AccountSubscription is private durable state, never a public HTTP DTO.
// Revision is optimistic concurrency protection in addition to transaction locks.
type AccountSubscription struct {
	RetryPolicy     SubscriptionRetryPolicy  `json:"retry_policy"`
	RetryUntil      time.Time                `json:"retry_until"`
	RetryFailures   int                      `json:"retry_failures"`
	AccountID       uuid.UUID                `json:"account_id"`
	Revision        int64                    `json:"revision"`
	Plan            SubscriptionPlanSnapshot `json:"plan"`
	NextPlan        SubscriptionPlanSnapshot `json:"next_plan"`
	PeriodStart     time.Time                `json:"period_start"`
	PaidUntil       time.Time                `json:"paid_until"`
	AutoRenew       bool                     `json:"auto_renew"`
	TrialUsed       bool                     `json:"trial_used"`
	PaymentMethodID string                   `json:"payment_method_id"`
	PendingIntentID uuid.UUID                `json:"pending_intent_id"`
	ActiveIntentID  uuid.UUID                `json:"active_intent_id"`
	CancelledAt     *time.Time               `json:"cancelled_at,omitempty"`
	PastDue         bool                     `json:"past_due"`
	NextAttemptAt   time.Time                `json:"next_attempt_at"`
}

// SubscriptionRetryPolicy is immutable opt-in commercial data. The zero value
// preserves historical subscriptions' original no-retry behavior.
type SubscriptionRetryPolicy struct {
	Version       int `json:"version"`
	WindowDays    int `json:"window_days"`
	IntervalHours int `json:"interval_hours"`
	MaxAttempts   int `json:"max_attempts"`
}

func (p SubscriptionRetryPolicy) Enabled() bool {
	return p == (SubscriptionRetryPolicy{Version: 1, WindowDays: 14, IntervalHours: 24, MaxAttempts: 14})
}

// SubscriptionRepository must be transaction-bound during payment settlement.
// LockSubscription serializes one canonical owner's subscription transitions.
type SubscriptionRepository interface {
	GetSubscription(context.Context, uuid.UUID) (*AccountSubscription, error)
	LockSubscription(context.Context, uuid.UUID) (*AccountSubscription, error)
	SaveSubscription(context.Context, *AccountSubscription) error
	ListDueSubscriptions(context.Context, time.Time, int) ([]uuid.UUID, error)
}

// VerifiedPaymentRefund is fetched from the authenticated payment-provider API;
// an inbound notification is never proof of a refund or its payment binding.
type VerifiedPaymentRefund struct {
	ID        string
	PaymentID string
	Status    PaymentRefundStatus
	Amount    int64
	Currency  Currency
}
type PaymentRefundVerifier interface {
	GetRefund(context.Context, string) (VerifiedPaymentRefund, error)
}
