package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// PaymentProviderCode identifies a money/payment provider. Keep this separate
// from AI ProviderName: payment adapters have different security and retry
// rules than generation providers.
type PaymentProviderCode string

const (
	// PaymentProviderMock is the local/test payment provider.
	PaymentProviderMock PaymentProviderCode = "mock"
	// PaymentProviderYooKassa is the YooKassa payment provider.
	PaymentProviderYooKassa PaymentProviderCode = "yookassa"
)

// Valid reports whether the payment provider is one of the known providers.
func (p PaymentProviderCode) Valid() bool {
	switch p {
	case PaymentProviderMock, PaymentProviderYooKassa:
		return true
	default:
		return false
	}
}

// PaymentIntentStatus is the explicit lifecycle state for a top-up payment.
type PaymentIntentStatus string

const (
	// PaymentIntentCreated means the intent exists locally but no provider
	// payment has been confirmed yet.
	PaymentIntentCreated PaymentIntentStatus = "created"
	// PaymentIntentProviderPending means provider creation/sync is in progress.
	PaymentIntentProviderPending PaymentIntentStatus = "provider_pending"
	// PaymentIntentWaitingForUser means the provider is waiting for user action.
	PaymentIntentWaitingForUser PaymentIntentStatus = "waiting_for_user"
	// PaymentIntentSucceeded means the provider confirmed payment success.
	PaymentIntentSucceeded PaymentIntentStatus = "succeeded"
	// PaymentIntentCanceled means the provider/user canceled the payment.
	PaymentIntentCanceled PaymentIntentStatus = "canceled"
	// PaymentIntentExpired means the intent passed its allowed payment window.
	PaymentIntentExpired PaymentIntentStatus = "expired"
	// PaymentIntentFailed means the payment failed permanently.
	PaymentIntentFailed PaymentIntentStatus = "failed"
	// PaymentIntentRefunded means the whole successful payment was refunded.
	PaymentIntentRefunded PaymentIntentStatus = "refunded"
	// PaymentIntentPartiallyRefunded means only part of the payment was refunded.
	PaymentIntentPartiallyRefunded PaymentIntentStatus = "partially_refunded"
)

var paymentIntentTransitions = map[PaymentIntentStatus][]PaymentIntentStatus{
	PaymentIntentCreated:           {PaymentIntentProviderPending, PaymentIntentWaitingForUser, PaymentIntentCanceled, PaymentIntentExpired, PaymentIntentFailed},
	PaymentIntentProviderPending:   {PaymentIntentWaitingForUser, PaymentIntentSucceeded, PaymentIntentCanceled, PaymentIntentExpired, PaymentIntentFailed},
	PaymentIntentWaitingForUser:    {PaymentIntentSucceeded, PaymentIntentCanceled, PaymentIntentExpired, PaymentIntentFailed},
	PaymentIntentSucceeded:         {PaymentIntentPartiallyRefunded, PaymentIntentRefunded},
	PaymentIntentCanceled:          {},
	PaymentIntentExpired:           {},
	PaymentIntentFailed:            {},
	PaymentIntentRefunded:          {},
	PaymentIntentPartiallyRefunded: {PaymentIntentPartiallyRefunded, PaymentIntentRefunded},
}

// Valid reports whether the status is one of the known payment intent states.
func (s PaymentIntentStatus) Valid() bool {
	_, ok := paymentIntentTransitions[s]
	return ok
}

// CanTransitionTo reports whether moving from the receiver status to target is
// allowed. It prevents late provider webhooks from rolling a paid intent back
// into canceled/failed states.
func (s PaymentIntentStatus) CanTransitionTo(target PaymentIntentStatus) bool {
	for _, allowed := range paymentIntentTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

// IsTerminal reports whether no further state transitions are allowed.
func (s PaymentIntentStatus) IsTerminal() bool {
	next, ok := paymentIntentTransitions[s]
	return ok && len(next) == 0
}

// PaymentRefundStatus tracks a provider refund lifecycle.
type PaymentRefundStatus string

const (
	PaymentRefundCreated         PaymentRefundStatus = "created"
	PaymentRefundProviderPending PaymentRefundStatus = "provider_pending"
	PaymentRefundSucceeded       PaymentRefundStatus = "succeeded"
	PaymentRefundFailed          PaymentRefundStatus = "failed"
	PaymentRefundCanceled        PaymentRefundStatus = "canceled"
)

// Valid reports whether the refund status is one of the known states.
func (s PaymentRefundStatus) Valid() bool {
	switch s {
	case PaymentRefundCreated,
		PaymentRefundProviderPending,
		PaymentRefundSucceeded,
		PaymentRefundFailed,
		PaymentRefundCanceled:
		return true
	default:
		return false
	}
}

// PaymentProduct is a top-up catalog item. Amount is stored in minor currency
// units (kopecks for RUB); credits is the internal ledger amount granted after
// trusted provider confirmation.
type PaymentProduct struct {
	ID             uuid.UUID `json:"id"`
	Code           string    `json:"code"`
	Title          string    `json:"title"`
	Amount         int64     `json:"amount"`
	Currency       Currency  `json:"currency"`
	Credits        int64     `json:"credits"`
	PriceVersion   int       `json:"price_version"`
	VATCode        *int16    `json:"vat_code,omitempty"`
	PaymentSubject string    `json:"payment_subject,omitempty"`
	PaymentMode    string    `json:"payment_mode,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TopUpPackage is a product alias used by payment UX docs.
type TopUpPackage = PaymentProduct

// PaymentIntent is an idempotent attempt to purchase credits. Amount, credits
// and price version are snapshots from PaymentProduct at creation time.
type PaymentIntent struct {
	ID                uuid.UUID           `json:"id"`
	UserID            uuid.UUID           `json:"user_id"`
	ProductID         *uuid.UUID          `json:"product_id,omitempty"`
	Status            PaymentIntentStatus `json:"status"`
	Amount            int64               `json:"amount"`
	Currency          Currency            `json:"currency"`
	Credits           int64               `json:"credits"`
	PriceVersion      int                 `json:"price_version"`
	Provider          PaymentProviderCode `json:"provider"`
	ProviderPaymentID string              `json:"provider_payment_id,omitempty"`
	ConfirmationURL   string              `json:"confirmation_url,omitempty"`
	IdempotencyKey    string              `json:"idempotency_key"`
	ReceiptEmail      string              `json:"receipt_email,omitempty"`
	ReceiptPhone      string              `json:"receipt_phone,omitempty"`
	Metadata          json.RawMessage     `json:"metadata,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	ExpiresAt         *time.Time          `json:"expires_at,omitempty"`
}

// PaymentEvent is the provider webhook inbox row. It stores raw provider data
// separately from the later, transactional processing that updates intent and
// ledger state.
type PaymentEvent struct {
	ID                uuid.UUID           `json:"id"`
	Provider          PaymentProviderCode `json:"provider"`
	EventType         string              `json:"event_type"`
	ProviderPaymentID string              `json:"provider_payment_id,omitempty"`
	ProviderRefundID  string              `json:"provider_refund_id,omitempty"`
	DedupKey          string              `json:"dedup_key"`
	Payload           json.RawMessage     `json:"payload"`
	ProcessedAt       *time.Time          `json:"processed_at,omitempty"`
	ReceivedAt        time.Time           `json:"received_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// PaymentRefund records a manual/provider refund for a successful payment
// intent. Its idempotency key is internal and may be longer than provider HTTP
// idempotency headers.
type PaymentRefund struct {
	ID               uuid.UUID           `json:"id"`
	IntentID         uuid.UUID           `json:"intent_id"`
	ProviderRefundID string              `json:"provider_refund_id,omitempty"`
	Amount           int64               `json:"amount"`
	Status           PaymentRefundStatus `json:"status"`
	IdempotencyKey   string              `json:"idempotency_key"`
	Reason           string              `json:"reason,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// CreatePaymentInput is the provider-agnostic request to create a payment on a
// money provider. Amount is in minor currency units (kopecks for RUB).
type CreatePaymentInput struct {
	IntentID       uuid.UUID       `json:"intent_id"`
	UserID         uuid.UUID       `json:"user_id"`
	Amount         int64           `json:"amount"`
	Currency       Currency        `json:"currency"`
	Credits        int64           `json:"credits"`
	Description    string          `json:"description,omitempty"`
	ReturnURL      string          `json:"return_url,omitempty"`
	ReceiptEmail   string          `json:"receipt_email,omitempty"`
	ReceiptPhone   string          `json:"receipt_phone,omitempty"`
	VATCode        *int16          `json:"vat_code,omitempty"`
	PaymentSubject string          `json:"payment_subject,omitempty"`
	PaymentMode    string          `json:"payment_mode,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
}

// CreatePaymentResult is a normalized provider response for a newly created
// payment.
type CreatePaymentResult struct {
	ProviderPaymentID string              `json:"provider_payment_id"`
	ConfirmationURL   string              `json:"confirmation_url,omitempty"`
	Status            PaymentIntentStatus `json:"status"`
	Raw               json.RawMessage     `json:"raw,omitempty"`
}

// ProviderPayment is the normalized state fetched from a payment provider.
type ProviderPayment struct {
	ProviderPaymentID string              `json:"provider_payment_id"`
	Status            PaymentIntentStatus `json:"status"`
	Amount            int64               `json:"amount"`
	Currency          Currency            `json:"currency"`
	Paid              bool                `json:"paid"`
	Captured          bool                `json:"captured"`
	Refundable        bool                `json:"refundable"`
	Raw               json.RawMessage     `json:"raw,omitempty"`
}

// CreateRefundInput is the provider-agnostic request to create a refund.
type CreateRefundInput struct {
	RefundID          uuid.UUID       `json:"refund_id"`
	IntentID          uuid.UUID       `json:"intent_id"`
	ProviderPaymentID string          `json:"provider_payment_id"`
	Amount            int64           `json:"amount"`
	Currency          Currency        `json:"currency"`
	Reason            string          `json:"reason,omitempty"`
	ReceiptEmail      string          `json:"receipt_email,omitempty"`
	ReceiptPhone      string          `json:"receipt_phone,omitempty"`
	VATCode           *int16          `json:"vat_code,omitempty"`
	PaymentSubject    string          `json:"payment_subject,omitempty"`
	PaymentMode       string          `json:"payment_mode,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

// RefundResult is a normalized provider response for a refund.
type RefundResult struct {
	ProviderRefundID string              `json:"provider_refund_id"`
	Status           PaymentRefundStatus `json:"status"`
	Amount           int64               `json:"amount"`
	Currency         Currency            `json:"currency"`
	Raw              json.RawMessage     `json:"raw,omitempty"`
}

// WebhookEvent is the normalized representation of a raw provider webhook
// event before it is stored in the payment_events inbox.
type WebhookEvent struct {
	Provider          PaymentProviderCode `json:"provider"`
	EventType         string              `json:"event_type"`
	ProviderPaymentID string              `json:"provider_payment_id,omitempty"`
	ProviderRefundID  string              `json:"provider_refund_id,omitempty"`
	DedupKey          string              `json:"dedup_key"`
	Payload           json.RawMessage     `json:"payload"`
}

// PaymentProvider is the port implemented by payment provider adapters. It
// hides provider-native HTTP/auth/idempotency shapes from payment services.
type PaymentProvider interface {
	Code() PaymentProviderCode
	CreatePayment(ctx context.Context, in CreatePaymentInput) (CreatePaymentResult, error)
	GetPayment(ctx context.Context, providerPaymentID string) (ProviderPayment, error)
	CancelPayment(ctx context.Context, providerPaymentID string) error
	CreateRefund(ctx context.Context, in CreateRefundInput) (RefundResult, error)
	ParseWebhook(ctx context.Context, raw []byte, headers http.Header) (WebhookEvent, error)
}
