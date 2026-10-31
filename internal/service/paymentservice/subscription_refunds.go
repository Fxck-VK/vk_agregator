package paymentservice

import (
	"context"
	"github.com/google/uuid"
	"strings"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type subscriptionLotRevoker interface {
	RevokeCreditLotForGrant(context.Context, uuid.UUID, string, time.Time) error
}

func (p *WebhookProcessor) processSubscriptionRefund(ctx context.Context, event *domain.PaymentEvent) error {
	verifier, ok := p.provider.(domain.PaymentRefundVerifier)
	if !ok {
		if p.provider.Code() == domain.PaymentProviderYooKassa {
			return ErrWebhookUnsupported
		}
		return p.ackInboxEvent(ctx, event.ID)
	}
	if strings.TrimSpace(event.ProviderRefundID) == "" {
		return ErrWebhookInvalid
	}
	verified, err := verifier.GetRefund(ctx, event.ProviderRefundID)
	if err != nil {
		return err
	}
	if verified.ID != event.ProviderRefundID || verified.PaymentID == "" || verified.Status != domain.PaymentRefundSucceeded || (event.ProviderPaymentID != "" && verified.PaymentID != event.ProviderPaymentID) {
		return ErrWebhookUnverified
	}
	return p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		currentEvent, err := payments.GetEventByID(ctx, event.ID)
		if err != nil {
			return err
		}
		if currentEvent.ProcessedAt != nil {
			return nil
		}
		intent, err := payments.GetIntentByProviderPaymentID(ctx, event.Provider, verified.PaymentID)
		if err != nil {
			return err
		}
		purchase, err := subscriptionPurchaseFrom(intent)
		if err != nil {
			return err
		}
		if purchase == nil {
			return payments.MarkEventProcessed(ctx, event.ID, p.now())
		}
		if verified.Amount != intent.Amount || verified.Currency != intent.Currency {
			return ErrWebhookUnsupported
		}
		store, ok := billing.(domain.SubscriptionRepository)
		if !ok {
			return ErrSubscriptionUnavailable
		}
		state, err := store.LockSubscription(ctx, intent.AccountID)
		if err != nil {
			return err
		}
		if intent.Status != domain.PaymentIntentRefunded {
			if intent.Status != domain.PaymentIntentSucceeded {
				return ErrWebhookUnverified
			}
			revoker, ok := billing.(subscriptionLotRevoker)
			if !ok {
				return ErrSubscriptionUnavailable
			}
			if err := revoker.RevokeCreditLotForGrant(ctx, intent.AccountID, topUpLedgerKey(intent.Provider, intent.ProviderPaymentID), p.now()); err != nil {
				return err
			}
			if err := payments.UpdateIntentStatus(ctx, intent.ID, intent.Status, domain.PaymentIntentRefunded); err != nil {
				return err
			}
		}
		if state.ActiveIntentID == intent.ID {
			if err := retireUnsubmittedRenewal(ctx, payments, state); err != nil {
				return err
			}
			now := p.now().UTC()
			state.PaidUntil = now
			state.AutoRenew = false
			state.PaymentMethodID = ""
			state.CancelledAt = &now
			state.PastDue = false
			resetSubscriptionRetry(state)
			if err := store.SaveSubscription(ctx, state); err != nil {
				return err
			}
		}
		return payments.MarkEventProcessed(ctx, event.ID, p.now())
	})
}
