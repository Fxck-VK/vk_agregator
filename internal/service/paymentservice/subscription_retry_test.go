package paymentservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

type retryFlow struct {
	svc       *Service
	processor *WebhookProcessor
	credits   *memory.BillingRepo
	repo      *memory.PaymentRepo
	owner     uuid.UUID
	now       time.Time
	pending   *domain.PaymentIntent
}

func TestSubscriptionRetryLatePaidDoesNotReviveExpiredConsent(t *testing.T) {
	f := retryFixture(t, 1)
	f.fail(t, "insufficient_funds")
	f.now = f.now.Add(24 * time.Hour)
	f.submit(t, f.now)
	in := f.pending
	s := f.state(t)
	f.now = s.RetryUntil.Add(time.Second)
	paid := domain.ProviderPayment{ProviderPaymentID: in.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: in.Amount, Currency: in.Currency, Paid: true, Captured: true, PaymentMethodSaved: true, PaymentMethodID: "test-saved-method"}
	if _, err := f.processor.applyProviderPayment(context.Background(), f.repo, f.credits, in, paid); err != nil {
		t.Fatal(err)
	}
	s = f.state(t)
	if s.AutoRenew || s.PaymentMethodID != "" || s.ActiveIntentID != in.ID || s.PastDue {
		t.Fatal("late paid retry revived expired recurring consent or lost paid access")
	}
}

func TestSubscriptionRetryStaleBatchClockCannotSubmitAfterDeadline(t *testing.T) {
	f := retryFixture(t, 1)
	f.fail(t, "insufficient_funds")
	s := f.state(t)
	s.RetryUntil = time.Now().UTC().Add(-time.Minute)
	s.NextAttemptAt = s.RetryUntil.Add(-time.Hour)
	if err := f.credits.SaveSubscription(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	calls := f.calls()
	f.submit(t, s.RetryUntil.Add(-time.Second))
	if f.calls() != calls || f.state(t).AutoRenew {
		t.Fatal("stale batch time authorized submit after actual deadline")
	}
}

func TestSubscriptionRetryErrorBackoffPreservesVerifiedDailySchedule(t *testing.T) {
	f := retryFixture(t, 1)
	f.fail(t, "insufficient_funds")
	s := f.state(t)
	s.NextAttemptAt = time.Now().UTC().Add(-time.Second)
	if err := f.credits.SaveSubscription(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	expected := time.Now().UTC().Add(24 * time.Hour)
	baseRunner := f.svc.cfg.SubscriptionTx
	calls := 0
	sentinel := errors.New("ambiguous submit after verified callback")
	f.svc.cfg.SubscriptionTx = TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		calls++
		if calls == 1 {
			// The authenticated callback won the lock before stale error handling.
			current, err := f.credits.GetSubscription(ctx, f.owner)
			if err != nil {
				return err
			}
			current.RetryFailures++
			current.NextAttemptAt = expected
			if err = f.credits.SaveSubscription(ctx, current); err != nil {
				return err
			}
			return sentinel
		}
		return baseRunner.RunPaymentTx(ctx, fn)
	})
	if _, err := f.svc.ProcessSubscriptionRenewals(context.Background(), 10); !errors.Is(err, sentinel) {
		t.Fatalf("expected ambiguous failure: %v", err)
	}
	if !f.state(t).NextAttemptAt.Equal(expected) {
		t.Fatal("stale5-minute backoff shortened verified24-hour retry schedule")
	}
}

func TestSubscriptionRetryDeadlineRetiresProvablyUnsentAttempt(t *testing.T) {
	f := retryFixture(t, 1)
	f.fail(t, "insufficient_funds")
	f.now = f.now.Add(24 * time.Hour)
	before := f.state(t)
	calls := f.calls()
	base := f.svc.cfg.SubscriptionTx
	steps := 0
	f.svc.cfg.SubscriptionTx = TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		steps++
		err := base.RunPaymentTx(ctx, fn)
		if err == nil && steps == 2 {
			state, e := f.credits.GetSubscription(ctx, f.owner)
			if e != nil {
				return e
			}
			state.RetryUntil = f.now.Add(-time.Second)
			return f.credits.SaveSubscription(ctx, state)
		}
		return err
	})
	f.submit(t, f.now)
	state := f.state(t)
	unsent, err := f.repo.GetIntentByIdempotencyKey(context.Background(), "subscription-renew:"+before.ActiveIntentID.String()+":retry:1")
	if err != nil || unsent.Status != domain.PaymentIntentCanceled || state.PendingIntentID != uuid.Nil || state.AutoRenew || f.calls() != calls {
		t.Fatal("deadline left a provably unsent attempt blocking a new checkout")
	}
}

func TestSubscriptionRetryStaleSchedulerPreservesCancellation(t *testing.T) {
	f := retryFixture(t, 1)
	selected := f.state(t)
	selected.NextAttemptAt = f.now
	if err := f.credits.SaveSubscription(context.Background(), selected); err != nil {
		t.Fatal(err)
	}
	base := f.svc.cfg.SubscriptionTx
	steps := 0
	var wrapped PaymentTxRunner
	wrapped = TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		steps++
		if steps == 2 {
			f.svc.cfg.SubscriptionTx = base
			_, err := f.svc.CancelSubscription(ctx, f.owner)
			f.svc.cfg.SubscriptionTx = wrapped
			if err != nil {
				return err
			}
			f.fail(t, "insufficient_funds")
		}
		return base.RunPaymentTx(ctx, fn)
	})
	f.svc.cfg.SubscriptionTx = wrapped
	f.submit(t, f.now)
	state := f.state(t)
	if state.CancelledAt == nil || state.AutoRenew || state.PastDue {
		t.Fatal("stale terminal scheduler overwrote explicit user cancellation")
	}
}

func retryFixture(t *testing.T, version int) *retryFlow {
	t.Helper()
	ctx := context.Background()
	svc, p, b, r, owner := subscriptionFixture(t)
	f := &retryFlow{svc, p, b, r, owner, time.Now().UTC(), nil}
	first, e := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, RetryPolicyVersion: version, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	p.now = func() time.Time { return f.now }
	paid := domain.ProviderPayment{ProviderPaymentID: first.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: first.Intent.Amount, Currency: first.Intent.Currency, Paid: true, Captured: true, PaymentMethodSaved: true, PaymentMethodID: "test-saved-method"}
	if _, e = p.applyProviderPayment(ctx, r, b, first.Intent, paid); e != nil {
		t.Fatal(e)
	}
	state, e := b.GetSubscription(ctx, owner)
	if e != nil {
		t.Fatal(e)
	}
	state.PaidUntil = f.now.Add(-time.Minute)
	if e = b.SaveSubscription(ctx, state); e != nil {
		t.Fatal(e)
	}
	f.submit(t, f.now)
	return f
}
func (f *retryFlow) submit(t *testing.T, now time.Time) {
	t.Helper()
	if e := f.svc.renewSubscription(context.Background(), f.owner, now); e != nil {
		t.Fatal(e)
	}
	state, e := f.credits.GetSubscription(context.Background(), f.owner)
	if e != nil {
		t.Fatal(e)
	}
	if state.PendingIntentID != uuid.Nil {
		f.pending, e = f.repo.GetIntentByID(context.Background(), state.PendingIntentID)
		if e != nil {
			t.Fatal(e)
		}
	}
}
func (f *retryFlow) fail(t *testing.T, reason string) {
	t.Helper()
	in := f.pending
	verified := domain.ProviderPayment{ProviderPaymentID: in.ProviderPaymentID, Status: domain.PaymentIntentCanceled, Amount: in.Amount, Currency: in.Currency, CancellationReason: reason}
	if _, e := f.processor.applyProviderPayment(context.Background(), f.repo, f.credits, in, verified); e != nil {
		t.Fatal(e)
	}
}
func (f *retryFlow) state(t *testing.T) *domain.AccountSubscription {
	t.Helper()
	s, e := f.credits.GetSubscription(context.Background(), f.owner)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func (f *retryFlow) calls() int { return len(f.svc.provider.(*subscriptionProvider).inputs) }

func TestSubscriptionRetryDailyRecoveryAndReplay(t *testing.T) {
	f := retryFixture(t, 1)
	ctx := context.Background()
	firstAttempt := f.pending.ID
	before, _ := f.credits.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	f.fail(t, "insufficient_funds")
	s := f.state(t)
	if !s.AutoRenew || !s.PastDue || s.PaymentMethodID == "" || s.PendingIntentID != uuid.Nil || s.RetryFailures != 1 || !s.RetryUntil.Equal(f.now.Add(14*24*time.Hour)) || !s.NextAttemptAt.Equal(f.now.Add(24*time.Hour)) {
		t.Fatal("verified insufficient funds did not schedule the approved daily14-day retry")
	}
	revision := s.Revision
	f.fail(t, "insufficient_funds")
	if f.state(t).Revision != revision {
		t.Fatal("old failure replay rescheduled recovery")
	}
	calls := f.calls()
	f.submit(t, f.now.Add(24*time.Hour-time.Nanosecond))
	if f.calls() != calls {
		t.Fatal("retry submitted before24 hours")
	}
	f.now = f.now.Add(24 * time.Hour)
	f.submit(t, f.now)
	next := f.pending
	if next.ID == firstAttempt || next.Amount != 54900 || next.Credits != 1250 || f.calls() != calls+1 {
		t.Fatal("new retry did not preserve terms or use one new intent")
	}
	f.submit(t, f.now)
	if f.calls() != calls+1 || f.pending.ID != next.ID {
		t.Fatal("concurrent/repeated scheduling duplicated payment")
	}
	paid := domain.ProviderPayment{ProviderPaymentID: next.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: next.Amount, Currency: next.Currency, Paid: true, Captured: true, PaymentMethodSaved: true, PaymentMethodID: "test-saved-method"}
	if _, e := f.processor.applyProviderPayment(ctx, f.repo, f.credits, next, paid); e != nil {
		t.Fatal(e)
	}
	if _, e := f.processor.applyProviderPayment(ctx, f.repo, f.credits, next, paid); e != nil {
		t.Fatal(e)
	}
	s = f.state(t)
	after, _ := f.credits.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if s.PastDue || !s.AutoRenew || s.RetryFailures != 0 || !s.RetryUntil.IsZero() || s.Plan.Code != "start" || !s.PeriodStart.Equal(f.now) || after.BalanceCached != before.BalanceCached+1250 {
		t.Fatal("recovered payment did not grant exactly once and reset recovery")
	}
}

func TestSubscriptionRetryWindowAndAttemptLimits(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		f := retryFixture(t, 1)
		f.fail(t, "insufficient_funds")
		calls := f.calls()
		f.submit(t, f.now.Add(14*24*time.Hour))
		s := f.state(t)
		if s.AutoRenew || s.PaymentMethodID != "" || f.calls() != calls {
			t.Fatal("new payment submitted at/after retry deadline")
		}
	})
	t.Run("fourteen_total_failures", func(t *testing.T) {
		f := retryFixture(t, 1)
		for i := 0; i < 14; i++ {
			if i > 0 {
				f.now = f.now.Add(24 * time.Hour)
				f.submit(t, f.now)
			}
			f.fail(t, "insufficient_funds")
		}
		s := f.state(t)
		calls := f.calls()
		f.submit(t, f.now.Add(24*time.Hour))
		if s.AutoRenew || s.RetryFailures != 14 || s.PaymentMethodID != "" || f.calls() != calls {
			t.Fatal("retry attempts were not bounded")
		}
	})
	t.Run("delayed_worker_no_burst", func(t *testing.T) {
		f := retryFixture(t, 1)
		f.fail(t, "insufficient_funds")
		calls := f.calls()
		f.now = f.now.Add(10 * 24 * time.Hour)
		f.submit(t, f.now)
		f.fail(t, "insufficient_funds")
		f.submit(t, f.now)
		s := f.state(t)
		if f.calls() != calls+1 || !s.NextAttemptAt.Equal(f.now.Add(24*time.Hour)) {
			t.Fatal("late worker caught up missed days with multiple charges")
		}
	})
}

func TestSubscriptionRetryLegacyHardDeclinesAndCancellation(t *testing.T) {
	t.Run("legacy_no_expansion", func(t *testing.T) {
		f := retryFixture(t, 0)
		f.fail(t, "insufficient_funds")
		if f.state(t).AutoRenew {
			t.Fatal("legacy consent silently gained retry attempts")
		}
	})
	for _, reason := range []string{"", "permission_revoked", "general_decline", "card_expired"} {
		t.Run("hard_"+reason, func(t *testing.T) {
			f := retryFixture(t, 1)
			f.fail(t, reason)
			s := f.state(t)
			if s.AutoRenew || s.PaymentMethodID != "" {
				t.Fatal("unapproved cancellation reason retained recurring method")
			}
		})
	}
	t.Run("cancel_between_attempts", func(t *testing.T) {
		f := retryFixture(t, 1)
		f.fail(t, "insufficient_funds")
		calls := f.calls()
		if _, e := f.svc.CancelSubscription(context.Background(), f.owner); e != nil {
			t.Fatal(e)
		}
		f.submit(t, f.now.Add(24*time.Hour))
		s := f.state(t)
		if s.AutoRenew || s.PaymentMethodID != "" || !s.RetryUntil.IsZero() || f.calls() != calls {
			t.Fatal("cancellation did not stop/clear retry cycle")
		}
	})
}

func TestSubscriptionRetryConsentVersionIsBoundToCheckout(t *testing.T) {
	svc, _, _, _, owner := subscriptionFixture(t)
	ctx := context.Background()
	for _, in := range []CreateSubscriptionInput{
		{AccountID: owner, PlanCode: "lite", AutoRenew: false, RetryPolicyVersion: 1, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()},
		{AccountID: owner, PlanCode: "lite", AutoRenew: true, RetryPolicyVersion: 99, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()},
	} {
		if _, err := svc.CreateSubscription(ctx, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatal("unapproved retry policy accepted")
		}
	}
	in := CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()}
	if _, err := svc.CreateSubscription(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.RetryPolicyVersion = 1
	if _, err := svc.CreateSubscription(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("existing checkout silently gained broader retry consent")
	}
}

type immediateTerminalSubscriptionProvider struct {
	*subscriptionProvider
	status domain.PaymentIntentStatus
}

func (p *immediateTerminalSubscriptionProvider) CreatePayment(ctx context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	result, err := p.subscriptionProvider.CreatePayment(ctx, in)
	result.Status = p.status
	return result, err
}

func TestSubscriptionRetryImmediateTerminalNeedsVerifiedGET(t *testing.T) {
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentSucceeded, domain.PaymentIntentCanceled, domain.PaymentIntentFailed, domain.PaymentIntentExpired} {
		t.Run(string(status), func(t *testing.T) {
			svc, _, credits, _, owner := subscriptionFixture(t)
			base := svc.provider.(*subscriptionProvider)
			svc.provider = &immediateTerminalSubscriptionProvider{base, status}
			in := CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, RetryPolicyVersion: 1, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()}
			result, err := svc.CreateSubscription(context.Background(), in)
			if err != nil || result.Intent.Status != domain.PaymentIntentProviderPending {
				t.Fatal("immediate result escaped authenticated GET verification")
			}
			again, err := svc.CreateSubscription(context.Background(), in)
			if err != nil || again.Intent.ID != result.Intent.ID || len(base.inputs) != 1 {
				t.Fatal("unverified terminal outcome caused another provider payment")
			}
			if _, err = credits.GetAccountByOwner(context.Background(), owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("unverified result granted stars")
			}
		})
	}
}
