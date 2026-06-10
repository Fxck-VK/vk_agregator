// Package mock provides an in-memory payment provider adapter for tests and
// local development.
package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// Provider is an in-memory implementation of domain.PaymentProvider.
type Provider struct {
	mu                sync.Mutex
	payments          map[string]domain.ProviderPayment
	paymentResults    map[string]domain.CreatePaymentResult
	paymentIdempotent map[string]string
	refunds           map[string]domain.RefundResult
	refundIdempotent  map[string]string
}

var _ domain.PaymentProvider = (*Provider)(nil)

// New returns a ready-to-use mock payment provider.
func New() *Provider {
	return &Provider{
		payments:          map[string]domain.ProviderPayment{},
		paymentResults:    map[string]domain.CreatePaymentResult{},
		paymentIdempotent: map[string]string{},
		refunds:           map[string]domain.RefundResult{},
		refundIdempotent:  map[string]string{},
	}
}

// Code returns the payment provider code.
func (p *Provider) Code() domain.PaymentProviderCode {
	return domain.PaymentProviderMock
}

// CreatePayment creates or returns an idempotently existing mock payment.
func (p *Provider) CreatePayment(_ context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	if in.Amount <= 0 {
		return domain.CreatePaymentResult{}, errors.New("mock payment: amount must be positive")
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return domain.CreatePaymentResult{}, errors.New("mock payment: idempotency key is required")
	}
	if in.Currency == "" {
		in.Currency = domain.CurrencyRUB
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existingID := p.paymentIdempotent[in.IdempotencyKey]; existingID != "" {
		return p.paymentResults[existingID], nil
	}

	intentID := in.IntentID
	if intentID == uuid.Nil {
		intentID = uuid.New()
	}
	providerPaymentID := "mock-pay-" + intentID.String()
	result := domain.CreatePaymentResult{
		ProviderPaymentID: providerPaymentID,
		ConfirmationURL:   "https://mock.payments.local/confirm/" + providerPaymentID,
		Status:            domain.PaymentIntentWaitingForUser,
		Raw:               mustJSON(map[string]any{"provider": "mock", "id": providerPaymentID}),
	}
	p.paymentResults[providerPaymentID] = result
	p.paymentIdempotent[in.IdempotencyKey] = providerPaymentID
	p.payments[providerPaymentID] = domain.ProviderPayment{
		ProviderPaymentID: providerPaymentID,
		Status:            domain.PaymentIntentWaitingForUser,
		Amount:            in.Amount,
		Currency:          in.Currency,
		Paid:              false,
		Captured:          false,
		Refundable:        false,
		Raw:               result.Raw,
	}
	return result, nil
}

// GetPayment returns the current normalized mock payment state.
func (p *Provider) GetPayment(_ context.Context, providerPaymentID string) (domain.ProviderPayment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	payment, ok := p.payments[providerPaymentID]
	if !ok {
		return domain.ProviderPayment{}, domain.ErrNotFound
	}
	return payment, nil
}

// CancelPayment marks a non-final mock payment as canceled.
func (p *Provider) CancelPayment(_ context.Context, providerPaymentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	payment, ok := p.payments[providerPaymentID]
	if !ok {
		return domain.ErrNotFound
	}
	if payment.Status == domain.PaymentIntentSucceeded ||
		payment.Status == domain.PaymentIntentRefunded ||
		payment.Status == domain.PaymentIntentPartiallyRefunded {
		return domain.ErrConflict
	}
	payment.Status = domain.PaymentIntentCanceled
	payment.Raw = mustJSON(map[string]any{"provider": "mock", "id": providerPaymentID, "status": payment.Status})
	p.payments[providerPaymentID] = payment
	return nil
}

// CreateRefund creates or returns an idempotently existing mock refund.
func (p *Provider) CreateRefund(_ context.Context, in domain.CreateRefundInput) (domain.RefundResult, error) {
	if in.Amount <= 0 {
		return domain.RefundResult{}, errors.New("mock payment: refund amount must be positive")
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return domain.RefundResult{}, errors.New("mock payment: refund idempotency key is required")
	}
	if in.Currency == "" {
		in.Currency = domain.CurrencyRUB
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existingID := p.refundIdempotent[in.IdempotencyKey]; existingID != "" {
		return p.refunds[existingID], nil
	}
	payment, ok := p.payments[in.ProviderPaymentID]
	if !ok {
		return domain.RefundResult{}, domain.ErrNotFound
	}
	if payment.Status != domain.PaymentIntentSucceeded && payment.Status != domain.PaymentIntentPartiallyRefunded {
		return domain.RefundResult{}, domain.ErrConflict
	}

	refundID := in.RefundID
	if refundID == uuid.Nil {
		refundID = uuid.New()
	}
	providerRefundID := "mock-refund-" + refundID.String()
	result := domain.RefundResult{
		ProviderRefundID: providerRefundID,
		Status:           domain.PaymentRefundSucceeded,
		Amount:           in.Amount,
		Currency:         in.Currency,
		Raw:              mustJSON(map[string]any{"provider": "mock", "id": providerRefundID, "payment_id": in.ProviderPaymentID}),
	}
	p.refunds[providerRefundID] = result
	p.refundIdempotent[in.IdempotencyKey] = providerRefundID

	if in.Amount >= payment.Amount {
		payment.Status = domain.PaymentIntentRefunded
		payment.Refundable = false
	} else {
		payment.Status = domain.PaymentIntentPartiallyRefunded
		payment.Refundable = true
	}
	payment.Raw = mustJSON(map[string]any{
		"provider": "mock",
		"id":       in.ProviderPaymentID,
		"status":   payment.Status,
	})
	p.payments[in.ProviderPaymentID] = payment
	return result, nil
}

// ParseWebhook normalizes a mock webhook JSON payload. Headers are accepted to
// match the provider port but are not used by the mock implementation.
func (p *Provider) ParseWebhook(_ context.Context, raw []byte, _ http.Header) (domain.WebhookEvent, error) {
	var payload struct {
		EventType         string `json:"event_type"`
		Type              string `json:"type"`
		ProviderPaymentID string `json:"provider_payment_id"`
		PaymentID         string `json:"payment_id"`
		ProviderRefundID  string `json:"provider_refund_id"`
		RefundID          string `json:"refund_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.WebhookEvent{}, fmt.Errorf("mock payment: parse webhook: %w", err)
	}
	eventType := firstNonEmpty(payload.EventType, payload.Type)
	paymentID := firstNonEmpty(payload.ProviderPaymentID, payload.PaymentID)
	refundID := firstNonEmpty(payload.ProviderRefundID, payload.RefundID)
	if eventType == "" {
		return domain.WebhookEvent{}, errors.New("mock payment: webhook event_type is required")
	}
	if paymentID == "" && refundID == "" {
		return domain.WebhookEvent{}, errors.New("mock payment: webhook payment or refund id is required")
	}
	naturalID := paymentID
	if refundID != "" {
		naturalID = refundID
	}
	return domain.WebhookEvent{
		Provider:          domain.PaymentProviderMock,
		EventType:         eventType,
		ProviderPaymentID: paymentID,
		ProviderRefundID:  refundID,
		DedupKey:          "webhook:mock:" + eventType + ":" + naturalID,
		Payload:           append(json.RawMessage(nil), raw...),
	}, nil
}

// SetPaymentStatus updates a mock payment state for tests and local smoke
// flows that need to simulate provider-side completion.
func (p *Provider) SetPaymentStatus(providerPaymentID string, status domain.PaymentIntentStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	payment, ok := p.payments[providerPaymentID]
	if !ok {
		return domain.ErrNotFound
	}
	payment.Status = status
	payment.Paid = status == domain.PaymentIntentSucceeded ||
		status == domain.PaymentIntentPartiallyRefunded ||
		status == domain.PaymentIntentRefunded
	payment.Captured = payment.Paid
	payment.Refundable = status == domain.PaymentIntentSucceeded ||
		status == domain.PaymentIntentPartiallyRefunded
	payment.Raw = mustJSON(map[string]any{"provider": "mock", "id": providerPaymentID, "status": status})
	p.payments[providerPaymentID] = payment
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
