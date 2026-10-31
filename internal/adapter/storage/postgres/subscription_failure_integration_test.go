package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// A provider-confirmed terminal renewal must update subscription state in the
// same transaction as the payment and inbox acknowledgement, not a minute later.
func pendingFailedRenewal(t *testing.T, status domain.PaymentIntentStatus, versions ...int) (*subscriptionPgFixture, *domain.PaymentIntent, *domain.PaymentEvent, *domain.AccountSubscription) {
	t.Helper()
	ctx := context.Background()
	f := newSubscriptionPgFixture(t, ctx)
	version := 0
	if len(versions) > 0 {
		version = versions[0]
	}
	initial, e := f.service.CreateSubscription(ctx, paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "lite", AutoRenew: true, RetryPolicyVersion: version, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.provider.SetPaymentStatus(initial.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); e != nil {
		t.Fatal(e)
	}
	if _, e = f.processor.SyncIntent(ctx, initial.Intent.ID); e != nil {
		t.Fatal(e)
	}
	if e = f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, b domain.BillingRepository) error {
		store := b.(domain.SubscriptionRepository)
		s, e := store.LockSubscription(ctx, f.owner)
		if e != nil {
			return e
		}
		s.PaidUntil = time.Now().UTC().Add(-time.Minute)
		return store.SaveSubscription(ctx, s)
	}); e != nil {
		t.Fatal(e)
	}
	if n, e := f.service.ProcessSubscriptionRenewals(ctx, 10); e != nil || n != 1 {
		t.Fatalf("prepare renewal: n=%d err=%v", n, e)
	}
	before, e := f.service.Subscription(ctx, f.owner)
	if e != nil {
		t.Fatal(e)
	}
	renewal, e := f.payments.GetIntentByID(ctx, before.PendingIntentID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.provider.SetPaymentStatus(renewal.ProviderPaymentID, status); e != nil {
		t.Fatal(e)
	}
	payload, e := json.Marshal(map[string]any{"type": "notification", "event": "payment.canceled", "object": map[string]string{"id": renewal.ProviderPaymentID}})
	if e != nil {
		t.Fatal(e)
	}
	event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "payment." + string(status), ProviderPaymentID: renewal.ProviderPaymentID, DedupKey: uuid.NewString(), Payload: payload}
	if _, e = f.payments.CreateEvent(ctx, event); e != nil {
		t.Fatal(e)
	}
	return f, renewal, event, before
}

func TestSubscriptionPostgresFailedRenewalStopsAtVerifiedWebhook(t *testing.T) {
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentCanceled, domain.PaymentIntentFailed, domain.PaymentIntentExpired} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			f, in, event, before := pendingFailedRenewal(t, status)
			if e := f.processor.ProcessEvent(ctx, event); e != nil {
				t.Fatal(e)
			}
			s, e := f.service.Subscription(ctx, f.owner)
			if e != nil {
				t.Fatal(e)
			}
			if s.AutoRenew || !s.PastDue || s.PaymentMethodID != "" {
				t.Fatalf("confirmed failure left renewal enabled or past-due unset: auto=%v past_due=%v method_present=%v", s.AutoRenew, s.PastDue, s.PaymentMethodID != "")
			}
			if s.ActiveIntentID != before.ActiveIntentID || s.PendingIntentID != in.ID || !s.PaidUntil.Equal(before.PaidUntil) || s.TrialUsed != before.TrialUsed || s.CancelledAt != nil {
				t.Fatal("failed renewal changed paid history, trial or user cancellation")
			}
			stored, e := f.payments.GetEventByID(ctx, event.ID)
			if e != nil || stored.ProcessedAt == nil {
				t.Fatal("failure was not acknowledged atomically")
			}
			var grants int
			if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE idempotency_key=$1`, "topup:yookassa:"+in.ProviderPaymentID).Scan(&grants); e != nil || grants != 0 {
				t.Fatal("failed renewal granted stars")
			}
			revision := s.Revision
			if e = f.processor.ProcessEvent(ctx, event); e != nil {
				t.Fatal(e)
			}
			if _, e = f.processor.SyncIntent(ctx, in.ID); e != nil {
				t.Fatal(e)
			}
			s, e = f.service.Subscription(ctx, f.owner)
			if e != nil || s.Revision != revision {
				t.Fatal("terminal replay rewrote subscription state")
			}
			if n, e := f.service.ProcessSubscriptionRenewals(ctx, 10); e != nil || n != 0 {
				t.Fatalf("failed renewal retried: n=%d err=%v", n, e)
			}
		})
	}
}

func TestSubscriptionPostgresFailedRenewalRollbackAndMismatch(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "provider_amount_mismatch"}[mismatch], func(t *testing.T) {
			ctx := context.Background()
			f, in, event, before := pendingFailedRenewal(t, domain.PaymentIntentCanceled)
			expected := errors.New("injected terminal renewal rollback")
			if mismatch {
				f.provider.amountDelta = 1
				expected = paymentservice.ErrWebhookMismatch
			} else {
				f.tx.rollbackAfterCallback = expected
			}
			if e := f.processor.ProcessEvent(ctx, event); !errors.Is(e, expected) {
				t.Fatalf("expected rejection/rollback, got %v", e)
			}
			s, e := f.service.Subscription(ctx, f.owner)
			if e != nil || s.Revision != before.Revision || !s.AutoRenew || s.PastDue || s.PaymentMethodID != before.PaymentMethodID {
				t.Fatal("rejected transaction changed subscription state")
			}
			pending, e := f.payments.GetIntentByID(ctx, in.ID)
			if e != nil || pending.Status != in.Status {
				t.Fatal("rejected transaction changed payment status")
			}
			stored, e := f.payments.GetEventByID(ctx, event.ID)
			if e != nil || stored.ProcessedAt != nil {
				t.Fatal("rejected event was acknowledged")
			}
			f.tx.rollbackAfterCallback = nil
			f.provider.amountDelta = 0
			if e = f.processor.ProcessEvent(ctx, event); e != nil {
				t.Fatal(e)
			}
			s, e = f.service.Subscription(ctx, f.owner)
			if e != nil || s.AutoRenew || !s.PastDue {
				t.Fatal("corrected retry did not stop failed renewal")
			}
		})
	}
}

func TestSubscriptionPostgresStaleFailurePreservesNewCheckout(t *testing.T) {
	ctx := context.Background()
	f, in, event, _ := pendingFailedRenewal(t, domain.PaymentIntentCanceled)
	// A user starts a new checkout after the previous attempt became terminal.
	if e := f.payments.UpdateIntentStatus(ctx, in.ID, in.Status, domain.PaymentIntentCanceled); e != nil {
		t.Fatal(e)
	}
	next, e := f.service.CreateSubscription(ctx, paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "pro", AutoRenew: true, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	before, e := f.service.Subscription(ctx, f.owner)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.processor.ProcessEvent(ctx, event); e != nil {
		t.Fatal(e)
	}
	after, e := f.service.Subscription(ctx, f.owner)
	if e != nil || after.Revision != before.Revision || after.PendingIntentID != next.Intent.ID || after.PastDue {
		t.Fatal("old failure changed new checkout")
	}
}

func TestSubscriptionPostgresFailedRenewalPreservesUserCancellation(t *testing.T) {
	ctx := context.Background()
	f, _, event, _ := pendingFailedRenewal(t, domain.PaymentIntentCanceled)
	before, err := f.service.CancelSubscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.processor.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	after, err := f.service.Subscription(ctx, f.owner)
	if err != nil || after.Revision != before.Revision || after.AutoRenew || after.PastDue || after.CancelledAt == nil || !after.CancelledAt.Equal(*before.CancelledAt) || !after.PaidUntil.Equal(before.PaidUntil) {
		t.Fatal("provider failure overwrote the user's cancellation or paid history")
	}
}
