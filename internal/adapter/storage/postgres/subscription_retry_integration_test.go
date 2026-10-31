package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestSubscriptionPostgresRetryPersistsAndRecoversOnce(t *testing.T) {
	ctx := context.Background()
	f, failed, event, before := pendingFailedRenewal(t, domain.PaymentIntentCanceled, 1)
	f.provider.cancelReason = "insufficient_funds"
	if err := f.processor.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	state, err := f.service.Subscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	if !state.AutoRenew || !state.PastDue || state.PaymentMethodID == "" || state.PendingIntentID != uuid.Nil || !state.RetryPolicy.Enabled() || state.RetryFailures != 1 || state.RetryUntil.Sub(state.NextAttemptAt) != 13*24*time.Hour {
		t.Fatal("retry policy or state did not persist atomically")
	}
	if !state.PaidUntil.Equal(before.PaidUntil) || state.ActiveIntentID != before.ActiveIntentID {
		t.Fatal("failed retry changed paid history")
	}
	revision := state.Revision
	if err = f.processor.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	state, err = f.service.Subscription(ctx, f.owner)
	if err != nil || state.Revision != revision {
		t.Fatal("failure replay rewrote schedule")
	}
	// Simulate tomorrow only in this isolated fixture; production always uses
	// the stored24-hour schedule. Concurrent workers must create one new intent.
	if err = f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, b domain.BillingRepository) error {
		store := b.(domain.SubscriptionRepository)
		s, e := store.LockSubscription(ctx, f.owner)
		if e != nil {
			return e
		}
		s.NextAttemptAt = time.Now().UTC().Add(-time.Second)
		return store.SaveSubscription(ctx, s)
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := f.service.ProcessSubscriptionRenewals(ctx, 10); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil && !errors.Is(e, domain.ErrConflict) {
			t.Fatal(e)
		}
	}
	state, err = f.service.Subscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := f.payments.GetIntentByID(ctx, state.PendingIntentID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID == failed.ID || retry.Amount != failed.Amount || retry.Credits != failed.Credits || retry.IdempotencyKey == failed.IdempotencyKey || f.provider.creates != 3 {
		t.Fatal("retry duplicated submit or changed original cycle terms")
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	balance := account.BalanceCached
	if err = f.provider.SetPaymentStatus(retry.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"id": retry.ProviderPaymentID})
	success := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, ProviderPaymentID: retry.ProviderPaymentID, EventType: "payment.succeeded", DedupKey: uuid.NewString(), Payload: payload}
	if _, err = f.payments.CreateEvent(ctx, success); err != nil {
		t.Fatal(err)
	}
	if err = f.processor.ProcessEvent(ctx, success); err != nil {
		t.Fatal(err)
	}
	if err = f.processor.ProcessEvent(ctx, success); err != nil {
		t.Fatal(err)
	}
	state, err = f.service.Subscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	account, err = f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	if state.PastDue || !state.AutoRenew || state.RetryFailures != 0 || !state.RetryUntil.IsZero() || state.ActiveIntentID != retry.ID || account.BalanceCached != balance+retry.Credits {
		t.Fatal("successful recovery did not reset once and grant correct stars")
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
}

func TestSubscriptionPostgresRetryRollbackAndCancellation(t *testing.T) {
	ctx := context.Background()
	f, _, event, before := pendingFailedRenewal(t, domain.PaymentIntentCanceled, 1)
	f.provider.cancelReason = "insufficient_funds"
	rollback := errors.New("retry atomic rollback")
	f.tx.rollbackAfterCallback = rollback
	if err := f.processor.ProcessEvent(ctx, event); !errors.Is(err, rollback) {
		t.Fatalf("expected rollback: %v", err)
	}
	state, err := f.service.Subscription(ctx, f.owner)
	if err != nil || state.Revision != before.Revision || state.RetryFailures != 0 || !state.RetryUntil.IsZero() {
		t.Fatal("rollback leaked retry state")
	}
	stored, err := f.payments.GetEventByID(ctx, event.ID)
	if err != nil || stored.ProcessedAt != nil {
		t.Fatal("rollback acknowledged failure")
	}
	f.tx.rollbackAfterCallback = nil
	if err = f.processor.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.CancelSubscription(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	state, err = f.service.Subscription(ctx, f.owner)
	if err != nil || state.AutoRenew || state.PaymentMethodID != "" || !state.RetryUntil.IsZero() {
		t.Fatal("user cancellation retained retry permission")
	}
	var due int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM account_subscriptions WHERE owner_account_id=$1 AND due_at IS NOT NULL`, f.owner).Scan(&due); err != nil || due != 0 {
		t.Fatal("canceled retry remained scheduled")
	}
	if n, err := f.service.ProcessSubscriptionRenewals(ctx, 10); err != nil || n != 0 {
		t.Fatal("canceled recovery submitted a payment")
	}
}

func TestSubscriptionPostgresRetryDeadlineAndLateSettlement(t *testing.T) {
	for _, latePaid := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline_stops_new_submit", true: "late_paid_does_not_restore_renewal"}[latePaid], func(t *testing.T) {
			ctx := context.Background()
			f, _, failure, _ := pendingFailedRenewal(t, domain.PaymentIntentCanceled, 1)
			f.provider.cancelReason = "insufficient_funds"
			if err := f.processor.ProcessEvent(ctx, failure); err != nil {
				t.Fatal(err)
			}
			if latePaid {
				if err := f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, b domain.BillingRepository) error {
					store := b.(domain.SubscriptionRepository)
					s, e := store.LockSubscription(ctx, f.owner)
					if e != nil {
						return e
					}
					s.NextAttemptAt = time.Now().Add(-time.Second)
					return store.SaveSubscription(ctx, s)
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := f.service.ProcessSubscriptionRenewals(ctx, 10); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, b domain.BillingRepository) error {
				store := b.(domain.SubscriptionRepository)
				s, e := store.LockSubscription(ctx, f.owner)
				if e != nil {
					return e
				}
				s.RetryUntil = time.Now().UTC().Add(-time.Second)
				s.NextAttemptAt = s.RetryUntil.Add(-time.Second)
				return store.SaveSubscription(ctx, s)
			}); err != nil {
				t.Fatal(err)
			}
			before, err := f.service.Subscription(ctx, f.owner)
			if err != nil {
				t.Fatal(err)
			}
			calls := f.provider.creates
			if latePaid {
				in, err := f.payments.GetIntentByID(ctx, before.PendingIntentID)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.provider.SetPaymentStatus(in.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
					t.Fatal(err)
				}
				if _, err = f.processor.SyncIntent(ctx, in.ID); err != nil {
					t.Fatal(err)
				}
				after, e := f.service.Subscription(ctx, f.owner)
				if e != nil || after.AutoRenew || after.PaymentMethodID != "" || after.ActiveIntentID != in.ID || !after.PaidUntil.After(time.Now()) {
					t.Fatal("late verified payment lost paid period or restored expired permission")
				}
			} else {
				if _, err := f.service.ProcessSubscriptionRenewals(ctx, 10); err != nil {
					t.Fatal(err)
				}
				after, e := f.service.Subscription(ctx, f.owner)
				if e != nil || after.AutoRenew || after.PaymentMethodID != "" || f.provider.creates != calls {
					t.Fatal("expired retry window submitted another charge")
				}
			}
		})
	}
}
