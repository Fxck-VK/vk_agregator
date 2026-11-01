package paymentservice

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

// Only the authenticated provider response establishes live/test mode. Unknown
// mode is retained as evidence so a later payment cannot become the first one.
func webReferralPaymentMode(raw json.RawMessage) domain.WebReferralPaymentMode {
	var proof struct {
		Test *bool `json:"test"`
	}
	if err := json.Unmarshal(raw, &proof); err != nil || proof.Test == nil {
		return domain.WebReferralPaymentUnknown
	}
	if *proof.Test {
		return domain.WebReferralPaymentTest
	}
	return domain.WebReferralPaymentLive
}

func (p *WebhookProcessor) recordWebReferralPayment(ctx context.Context, payments domain.PaymentRepository, intent *domain.PaymentIntent, verified domain.ProviderPayment) error {
	store, ok := payments.(domain.WebReferralEvidenceRepository)
	if !ok || intent.AccountID == uuid.Nil || intent.Provider != domain.PaymentProviderYooKassa || p.provider.Code() != domain.PaymentProviderYooKassa || intent.Amount <= 0 {
		return nil
	}
	return store.RecordWebReferralPayment(ctx, domain.WebReferralPaymentEvidence{
		IntentID: intent.ID, AccountID: intent.AccountID, Provider: string(intent.Provider),
		ProviderPaymentID: intent.ProviderPaymentID, Amount: intent.Amount, Currency: string(intent.Currency),
		VerifiedAt: p.now().UTC(), Mode: webReferralPaymentMode(verified.Raw),
	})
}

func (p *WebhookProcessor) recordWebReferralRefund(ctx context.Context, payments domain.PaymentRepository, intent *domain.PaymentIntent, verified domain.VerifiedPaymentRefund) error {
	store, ok := payments.(domain.WebReferralEvidenceRepository)
	if !ok || intent.AccountID == uuid.Nil || intent.Provider != domain.PaymentProviderYooKassa || p.provider.Code() != domain.PaymentProviderYooKassa {
		return nil
	}
	if strings.TrimSpace(verified.ID) == "" || verified.PaymentID != intent.ProviderPaymentID || verified.Status != domain.PaymentRefundSucceeded || verified.Amount <= 0 || verified.Amount > intent.Amount || verified.Currency != intent.Currency {
		return ErrWebhookUnverified
	}
	return store.RecordWebReferralRefund(ctx, domain.WebReferralRefundEvidence{
		IntentID: intent.ID, Provider: string(intent.Provider), ProviderPaymentID: verified.PaymentID,
		ProviderRefundID: verified.ID, Amount: verified.Amount, Currency: string(verified.Currency), VerifiedAt: p.now().UTC(),
	})
}

// A refund must cancel reward eligibility even when the existing principal
// refund path cannot process partial subscriptions or externally refunded
// top-ups. Commit the verified fact before entering that separate path.
func (p *WebhookProcessor) persistWebReferralRefund(ctx context.Context, event *domain.PaymentEvent, verified domain.VerifiedPaymentRefund) error {
	return p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, _ domain.BillingRepository) error {
		intent, err := payments.GetIntentByProviderPaymentID(ctx, event.Provider, verified.PaymentID)
		if err != nil {
			return err
		}
		return p.recordWebReferralRefund(ctx, payments, intent, verified)
	})
}
