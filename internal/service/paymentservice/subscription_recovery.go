package paymentservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

// recoverSubscriptionPaymentBinding runs inside the webhook transaction, after
// authenticated GetPayment verification. It recovers a provider payment whose
// original response/binding write was lost; webhook metadata is never consulted.
func (p *WebhookProcessor) recoverSubscriptionPaymentBinding(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository, verified domain.ProviderPayment) (*domain.PaymentIntent, error) {
	if p.provider.Code() != domain.PaymentProviderYooKassa || verified.VerifiedIntentID == uuid.Nil || verified.VerifiedAccountID == uuid.Nil || strings.TrimSpace(verified.ProviderPaymentID) == "" {
		return nil, fmt.Errorf("%w: subscription recovery requires verified binding", ErrWebhookMismatch)
	}
	switch verified.Status {
	case domain.PaymentIntentSucceeded:
		if !verified.Paid || !verified.Captured {
			return nil, fmt.Errorf("%w: subscription recovery requires paid/captured success", ErrWebhookMismatch)
		}
	case domain.PaymentIntentCanceled, domain.PaymentIntentFailed:
		if verified.Paid || verified.Captured {
			return nil, fmt.Errorf("%w: subscription recovery terminal result is still paid/captured", ErrWebhookMismatch)
		}
	default:
		return nil, fmt.Errorf("%w: subscription recovery requires a final provider outcome", ErrWebhookMismatch)
	}
	store, ok := billing.(domain.SubscriptionRepository)
	if !ok {
		return nil, ErrSubscriptionUnavailable
	}
	// This locks accounts -> subscription before any intent binding or ledger
	// mutation, matching cancellation and ordinary subscription settlement.
	state, err := store.LockSubscription(ctx, verified.VerifiedAccountID)
	if err != nil {
		return nil, err
	}
	intent, err := payments.GetIntentByID(ctx, verified.VerifiedIntentID)
	if err != nil {
		return nil, err
	}
	if intent.Provider != p.provider.Code() || intent.AccountID != verified.VerifiedAccountID || intent.Amount <= 0 || intent.Credits <= 0 || intent.Amount != verified.Amount || intent.Currency != verified.Currency {
		return nil, fmt.Errorf("%w: subscription recovery provenance", ErrWebhookMismatch)
	}
	purchase, err := subscriptionPurchaseFrom(intent)
	if err != nil || purchase == nil {
		return nil, fmt.Errorf("%w: subscription recovery purchase snapshot", ErrWebhookMismatch)
	}
	started, submitted, err := submissionStartedAt(intent)
	if err != nil || !submitted || started.IsZero() {
		return nil, fmt.Errorf("%w: subscription recovery submit marker", ErrWebhookMismatch)
	}
	if intent.ProviderPaymentID != "" {
		if intent.ProviderPaymentID != verified.ProviderPaymentID {
			return nil, fmt.Errorf("%w: subscription recovery already bound", ErrWebhookMismatch)
		}
		// Another webhook transaction may have recovered this binding while we
		// waited for its owner lock. Normal apply handles its settled replay.
		return intent, nil
	}
	if state.PendingIntentID != intent.ID {
		return nil, fmt.Errorf("%w: subscription recovery pending intent", ErrWebhookMismatch)
	}
	switch intent.Status {
	case domain.PaymentIntentCreated, domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser:
	default:
		return nil, fmt.Errorf("%w: subscription recovery terminal intent", ErrWebhookMismatch)
	}
	// Bind as pending so the ordinary verified path atomically grants a paid
	// success or marks an unpaid terminal result replaceable by a fresh checkout.
	if err := payments.SetIntentProviderState(ctx, intent.ID, domain.PaymentIntentProviderPending, verified.ProviderPaymentID, ""); err != nil {
		return nil, err
	}
	return payments.GetIntentByID(ctx, intent.ID)
}
