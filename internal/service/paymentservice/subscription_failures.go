package paymentservice

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

// stopFailedRenewal runs in the verified payment/inbox transaction under the
// canonical owner lock. A stale failure cannot disable a newer paid period or
// checkout, and provider failure must not overwrite an explicit cancellation.
func (p *WebhookProcessor) stopFailedRenewal(ctx context.Context, billing domain.BillingRepository, intent *domain.PaymentIntent, verified domain.ProviderPayment, purchase *subscriptionPurchase) error {
	if !purchase.Renewal {
		return nil
	}
	if verified.Amount != intent.Amount || verified.Currency != intent.Currency || verified.Paid || verified.Captured {
		return fmt.Errorf("%w: terminal subscription renewal", ErrWebhookMismatch)
	}
	store, ok := billing.(domain.SubscriptionRepository)
	if !ok {
		return ErrSubscriptionUnavailable
	}
	state, err := store.LockSubscription(ctx, intent.AccountID)
	if err != nil {
		return err
	}
	if purchase.PreviousIntentID == uuid.Nil || state.PendingIntentID != intent.ID || state.ActiveIntentID != purchase.PreviousIntentID || state.CancelledAt != nil {
		return nil
	}
	if state.PastDue && !state.AutoRenew && state.PaymentMethodID == "" {
		return nil
	}
	state.PastDue = true
	now := p.now().UTC()
	if state.AutoRenew && state.PaymentMethodID != "" && purchase.RetryPolicy.Enabled() && state.RetryPolicy == purchase.RetryPolicy && verified.Status == domain.PaymentIntentCanceled && verified.CancellationReason == "insufficient_funds" {
		if state.RetryUntil.IsZero() {
			state.RetryUntil = now.Add(time.Duration(state.RetryPolicy.WindowDays) * 24 * time.Hour)
		}
		state.RetryFailures++
		next := now.Add(time.Duration(state.RetryPolicy.IntervalHours) * time.Hour)
		if state.RetryFailures < state.RetryPolicy.MaxAttempts && next.Before(state.RetryUntil) {
			state.PendingIntentID = uuid.Nil
			state.NextAttemptAt = next
			return store.SaveSubscription(ctx, state)
		}
	}
	disableSubscriptionRenewal(state)
	return store.SaveSubscription(ctx, state)
}

func retryWindowExhausted(state *domain.AccountSubscription, now time.Time) bool {
	return !state.RetryUntil.IsZero() && (!now.Before(state.RetryUntil) || state.RetryFailures >= state.RetryPolicy.MaxAttempts)
}

func disableSubscriptionRenewal(state *domain.AccountSubscription) {
	state.AutoRenew = false
	state.PaymentMethodID = ""
	state.NextAttemptAt = time.Time{}
}

func resetSubscriptionRetry(state *domain.AccountSubscription) {
	state.RetryUntil = time.Time{}
	state.RetryFailures = 0
	state.NextAttemptAt = time.Time{}
}

// Batch selection time cannot authorize a submit delayed behind an owner lock.
// Keep an explicitly later test/batch time while always observing actual time.
func currentRenewalTime(batch time.Time) time.Time {
	now := time.Now().UTC()
	if now.After(batch) {
		return now
	}
	return batch
}

// Only the owner of a new first submission receipt can prove it has not called
// the provider. Retried/ambiguous intents retain their binding for late payment.
func expireOwnedSubscriptionRetry(ctx context.Context, payments domain.PaymentRepository, store domain.SubscriptionRepository, state *domain.AccountSubscription, intent *domain.PaymentIntent, receipt subscriptionSubmissionReceipt) error {
	disableSubscriptionRenewal(state)
	if receipt.FirstAttempt && ownsSubmissionReceipt(intent, receipt) && intent.Status == domain.PaymentIntentCreated && intent.ProviderPaymentID == "" {
		if err := payments.UpdateIntentStatus(ctx, intent.ID, intent.Status, domain.PaymentIntentCanceled); err != nil {
			return err
		}
		state.PendingIntentID = uuid.Nil
	}
	return store.SaveSubscription(ctx, state)
}
