package paymentservice

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

const subscriptionSubmitMarker = "subscription_submit_started_at"
const subscriptionSubmitAttempt = "subscription_submit_attempt_id"

type subscriptionSubmissionReceipt struct {
	ID           uuid.UUID
	FirstAttempt bool
}

func ownsSubmissionReceipt(intent *domain.PaymentIntent, receipt subscriptionSubmissionReceipt) bool {
	if receipt.ID == uuid.Nil {
		return false
	}
	var metadata map[string]json.RawMessage
	if json.Unmarshal(intent.Metadata, &metadata) != nil {
		return false
	}
	var value string
	return json.Unmarshal(metadata[subscriptionSubmitAttempt], &value) == nil && value == receipt.ID.String()
}

func submissionStartedAt(intent *domain.PaymentIntent) (time.Time, bool, error) {
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return time.Time{}, false, ErrInvalidInput
	}
	raw, ok := metadata[subscriptionSubmitMarker]
	if !ok {
		return time.Time{}, false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return time.Time{}, false, ErrInvalidInput
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false, ErrInvalidInput
	}
	return at, true, nil
}

// This receipt commits BEFORE the network transaction. A timeout or DB rollback
// after provider acceptance can no longer make the intent look never submitted.
func (s *Service) markSubscriptionSubmit(ctx context.Context, owner, intentID uuid.UUID, now time.Time) (subscriptionSubmissionReceipt, error) {
	var receipt subscriptionSubmissionReceipt
	err := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		receipt = subscriptionSubmissionReceipt{}
		store, ok := billing.(domain.SubscriptionRepository)
		if !ok {
			return ErrSubscriptionUnavailable
		}
		state, err := store.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		if state.PendingIntentID != intentID || !state.AutoRenew || state.CancelledAt != nil {
			return nil
		}
		intent, err := payments.GetIntentByID(ctx, intentID)
		if err != nil {
			return err
		}
		if intent.ProviderPaymentID != "" || intent.Status != domain.PaymentIntentCreated {
			return nil
		}
		_, exists, err := submissionStartedAt(intent)
		if err != nil {
			return err
		}
		var metadata map[string]json.RawMessage
		if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
			return err
		}
		if !exists {
			marker, _ := json.Marshal(now.UTC().Format(time.RFC3339Nano))
			metadata[subscriptionSubmitMarker] = marker
		}
		// A later invocation supersedes this receipt before it can call the
		// provider. Preserve the first timestamp and its idempotency deadline.
		receipt = subscriptionSubmissionReceipt{ID: uuid.New(), FirstAttempt: !exists}
		token, _ := json.Marshal(receipt.ID.String())
		metadata[subscriptionSubmitAttempt] = token
		raw, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		return payments.UpdateIntentMetadata(ctx, intentID, raw)
	})
	return receipt, err
}

// Retire only a provably unsubmitted renewal. Submitted/ambiguous intents keep
// their binding so authenticated late success can still grant its paid period.
func retireUnsubmittedRenewal(ctx context.Context, payments domain.PaymentRepository, state *domain.AccountSubscription) error {
	if state.PendingIntentID == uuid.Nil {
		return nil
	}
	intent, err := payments.GetIntentByID(ctx, state.PendingIntentID)
	if err != nil {
		return err
	}
	purchase, err := subscriptionPurchaseFrom(intent)
	if err != nil {
		return err
	}
	if purchase == nil || !purchase.Renewal || intent.Status != domain.PaymentIntentCreated || intent.ProviderPaymentID != "" {
		return nil
	}
	_, submitted, err := submissionStartedAt(intent)
	if err != nil {
		return err
	}
	if submitted {
		return nil
	}
	if err := payments.UpdateIntentStatus(ctx, intent.ID, intent.Status, domain.PaymentIntentCanceled); err != nil {
		return err
	}
	state.PendingIntentID = uuid.Nil
	return nil
}
