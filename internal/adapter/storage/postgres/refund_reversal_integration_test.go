package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// The money provider is entirely in memory; all attribution and transaction
// checks run against the isolated PostgreSQL integration schema.
type reversalRefundProvider struct {
	*subscriptionPgProvider
	verifiedRefund domain.VerifiedPaymentRefund
	refundCalls    atomic.Int32
	failRefund     bool
}

func (p *reversalRefundProvider) GetRefund(_ context.Context, id string) (domain.VerifiedPaymentRefund, error) {
	if id != p.verifiedRefund.ID {
		return domain.VerifiedPaymentRefund{}, domain.ErrNotFound
	}
	return p.verifiedRefund, nil
}
func (p *reversalRefundProvider) CreateRefund(ctx context.Context, in domain.CreateRefundInput) (domain.RefundResult, error) {
	p.refundCalls.Add(1)
	if p.failRefund {
		return domain.RefundResult{}, errors.New("injected refund provider failure")
	}
	return p.subscriptionPgProvider.Provider.CreateRefund(ctx, in)
}

type reversalRefundFixture struct {
	*subscriptionPgFixture
	provider            *reversalRefundProvider
	subscription, topup *domain.PaymentIntent
	event               *domain.PaymentEvent
}

func newReversalRefundFixture(t *testing.T) *reversalRefundFixture {
	t.Helper()
	ctx := context.Background()
	f := newSubscriptionPgFixture(t, ctx)
	p := &reversalRefundProvider{subscriptionPgProvider: f.provider}
	f.service = paymentservice.New(f.payments, p, paymentservice.Config{SubscriptionStore: f.billing, SubscriptionBilling: f.billing, SubscriptionTx: f.tx, RecurringEnabled: true, AccountReturnURL: "https://epifera.test/app/payment-return"})
	f.processor = paymentservice.NewWebhookProcessor(f.payments, p, billingservice.New(f.billing), f.tx)
	sub, err := f.service.CreateSubscription(ctx, paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "lite", ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetPaymentStatus(sub.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	if _, err := f.processor.SyncIntent(ctx, sub.Intent.ID); err != nil {
		t.Fatal(err)
	}
	product := &domain.PaymentProduct{Code: "refund-proof-" + uuid.NewString(), Title: "Test top-up", Amount: 9900, Currency: domain.CurrencyRUB, Credits: 198, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, PriceVersion: 1, IsActive: true, PaymentSubject: "service", PaymentMode: "full_prepayment"}
	if err := f.payments.CreateProduct(ctx, product); err != nil {
		t.Fatal(err)
	}
	topup, err := f.service.CreateAccountIntent(ctx, paymentservice.CreateAccountIntentInput{AccountID: f.owner, ProductCode: product.Code, ReceiptEmail: "fixture@example.test", IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetPaymentStatus(topup.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	if _, err := f.processor.SyncIntent(ctx, topup.Intent.ID); err != nil {
		t.Fatal(err)
	}
	p.verifiedRefund = domain.VerifiedPaymentRefund{ID: "sub-refund-" + uuid.NewString(), PaymentID: sub.Intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: sub.Intent.Amount, Currency: domain.CurrencyRUB}
	payload, _ := json.Marshal(map[string]string{"id": p.verifiedRefund.ID, "payment_id": sub.Intent.ProviderPaymentID})
	event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "refund.succeeded", ProviderPaymentID: sub.Intent.ProviderPaymentID, ProviderRefundID: p.verifiedRefund.ID, DedupKey: "webhook:yookassa:refund.succeeded:" + p.verifiedRefund.ID, Payload: payload}
	if _, err := f.payments.CreateEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	return &reversalRefundFixture{subscriptionPgFixture: f, provider: p, subscription: sub.Intent, topup: topup.Intent, event: event}
}

func (f *reversalRefundFixture) revoke(t *testing.T) {
	t.Helper()
	if err := f.processor.ProcessEvent(context.Background(), f.event); err != nil {
		t.Fatal(err)
	}
}
func (f *reversalRefundFixture) balance(t *testing.T) int64 {
	t.Helper()
	a, err := f.billing.GetAccountByOwner(context.Background(), f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	return a.BalanceCached
}
func (f *reversalRefundFixture) refundInput() paymentservice.RefundIntentInput {
	return paymentservice.RefundIntentInput{IntentID: f.topup.ID, IdempotencyKey: "proof-refund:" + f.topup.ID.String(), Reason: "test unused top-up refund"}
}

func TestTopupRefundAfterSubscriptionReversal(t *testing.T) {
	ctx := context.Background()
	f := newReversalRefundFixture(t)
	f.revoke(t)
	if balance := f.balance(t); balance != 258 {
		t.Fatalf("after isolated subscription reversal balance=%d want258", balance)
	}
	result, err := f.processor.RefundIntent(ctx, f.refundInput())
	if err != nil {
		t.Fatalf("unused older top-up blocked by unrelated subscription reversal: %v", err)
	}
	if result.Refund.Status != domain.PaymentRefundSucceeded || result.Intent.Status != domain.PaymentIntentRefunded {
		t.Fatal("refund did not settle")
	}
	if balance := f.balance(t); balance != 60 {
		t.Fatalf("after full refund balance=%d want60", balance)
	}
	again, err := f.processor.RefundIntent(ctx, f.refundInput())
	if err != nil || again.Refund.ID != result.Refund.ID || f.provider.refundCalls.Load() != 1 {
		t.Fatalf("refund replay differs: %v calls=%d", err, f.provider.refundCalls.Load())
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
	var debits int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE account_id=$1 AND type='adjustment' AND amount=-198 AND status='committed'`, account.ID).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if debits != 1 {
		t.Fatalf("refund debit count=%d want1", debits)
	}
}

func (f *reversalRefundFixture) hold(t *testing.T, capture bool) {
	t.Helper()
	ctx := context.Background()
	jobID := uuid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, jobID, f.owner, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	bill := billingservice.New(f.billing)
	reservation, err := bill.ReserveForAccount(ctx, f.owner, jobID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if capture {
		if err := bill.CaptureForAccount(ctx, f.owner, reservation.ID, 10); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTopupRefundAfterSubscriptionReversalRejectsUnprovenDebits(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		before func(*testing.T, *reversalRefundFixture)
		after  func(*testing.T, *reversalRefundFixture)
	}{
		{name: "subscription_was_spent", before: func(t *testing.T, f *reversalRefundFixture) { f.hold(t, true) }},
		{name: "subscription_still_reserved", before: func(t *testing.T, f *reversalRefundFixture) { f.hold(t, false) }},
		{name: "refund_not_processed", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE payment_events SET processed_at=NULL WHERE id=$1`, f.event.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "source_payment_not_refunded", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE payment_intents SET status='succeeded' WHERE id=$1`, f.subscription.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "source_snapshot_mismatch", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE payment_intents SET metadata=jsonb_set(metadata,'{subscription,plan,stars}','399') WHERE id=$1`, f.subscription.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "source_owner_mismatch", after: func(t *testing.T, f *reversalRefundFixture) {
			foreign := uuid.New()
			if _, err := f.pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, foreign); err != nil {
				t.Fatal(err)
			}
			if _, err := f.pool.Exec(ctx, `UPDATE payment_intents SET account_id=$2 WHERE id=$1`, f.subscription.ID, foreign); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsupported_reversal_denomination", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE ledger_entries SET credit_denomination_version=1 WHERE owner_account_id=$1 AND idempotency_key LIKE 'credit-lot-expiry:%'`, f.owner); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "grant_not_fully_reversed", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE ledger_entries SET amount=-399 WHERE owner_account_id=$1 AND idempotency_key LIKE 'credit-lot-expiry:%'`, f.owner); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing_subscription_lot", after: func(t *testing.T, f *reversalRefundFixture) {
			if _, err := f.pool.Exec(ctx, `UPDATE credit_lots SET kind='legacy',expires_at=NULL WHERE account_id IN(SELECT id FROM credit_accounts WHERE owner_account_id=$1) AND kind='subscription'`, f.owner); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "forged_expiry_key", after: func(t *testing.T, f *reversalRefundFixture) {
			account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.billing.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, Type: domain.LedgerAdjustment, Amount: -1, Status: domain.LedgerStatusCommitted, IdempotencyKey: "credit-lot-expiry:" + uuid.NewString(), Reason: "credit lot expiry"}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "real_later_adjustment", after: func(t *testing.T, f *reversalRefundFixture) {
			account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.billing.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, Type: domain.LedgerAdjustment, Amount: -1, Status: domain.LedgerStatusCommitted, IdempotencyKey: uuid.NewString(), Reason: "test actual spend"}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReversalRefundFixture(t)
			if tc.before != nil {
				tc.before(t, f)
			}
			f.revoke(t)
			if tc.after != nil {
				tc.after(t, f)
			}
			before := f.balance(t)
			_, err := f.processor.RefundIntent(ctx, f.refundInput())
			if !errors.Is(err, paymentservice.ErrRefundCreditsSpent) && !errors.Is(err, paymentservice.ErrRefundNotAllowed) {
				t.Fatalf("unproven/real debit accepted or wrong failure: %v", err)
			}
			if f.provider.refundCalls.Load() != 0 || f.balance(t) != before {
				t.Fatal("rejected refund submitted money or mutated balance")
			}
			var refunds int
			if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refunds WHERE intent_id=$1`, f.topup.ID).Scan(&refunds); err != nil {
				t.Fatal(err)
			}
			if refunds != 0 {
				t.Fatal("rejected refund persisted a refund record")
			}
		})
	}
}

func TestTopupRefundAfterSubscriptionReversalProviderFailureAndRollback(t *testing.T) {
	ctx := context.Background()
	t.Run("provider_failure_compensates_once", func(t *testing.T) {
		f := newReversalRefundFixture(t)
		f.revoke(t)
		f.provider.failRefund = true
		_, err := f.processor.RefundIntent(ctx, f.refundInput())
		if err == nil || f.provider.refundCalls.Load() != 1 {
			t.Fatalf("expected provider failure after safe debit, calls=%d err=%v", f.provider.refundCalls.Load(), err)
		}
		if f.balance(t) != 258 {
			t.Fatal("provider failure lost purchased stars")
		}
		account, e := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
		if e != nil {
			t.Fatal(e)
		}
		assertPostgresLotProjection(t, ctx, f.pool, account.ID)
		again, e := f.processor.RefundIntent(ctx, f.refundInput())
		if e != nil || again.Refund.Status != domain.PaymentRefundFailed || f.provider.refundCalls.Load() != 1 || f.balance(t) != 258 {
			t.Fatalf("failed-refund replay repeated compensation/provider call: %v", e)
		}
	})
	t.Run("outer_transaction_rolls_back_proof_and_debit", func(t *testing.T) {
		f := newReversalRefundFixture(t)
		f.revoke(t)
		sentinel := errors.New("injected refund transaction rollback")
		f.tx.rollbackAfterCallback = sentinel
		_, err := f.processor.RefundIntent(ctx, f.refundInput())
		if !errors.Is(err, sentinel) || f.provider.refundCalls.Load() != 0 || f.balance(t) != 258 {
			t.Fatalf("rollback leaked debit/provider submit: %v", err)
		}
		var refunds int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refunds WHERE intent_id=$1`, f.topup.ID).Scan(&refunds); err != nil {
			t.Fatal(err)
		}
		if refunds != 0 {
			t.Fatal("outer rollback left a refund record")
		}
		f.tx.rollbackAfterCallback = nil
		if _, err := f.processor.RefundIntent(ctx, f.refundInput()); err != nil || f.balance(t) != 60 || f.provider.refundCalls.Load() != 1 {
			t.Fatalf("retry after rollback failed: %v", err)
		}
	})
}

func TestTopupRefundAfterSubscriptionReversalSerializesWithBilling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newReversalRefundFixture(t)
	f.revoke(t)
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, f.owner); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := f.processor.RefundIntent(ctx, f.refundInput()); done <- err }()
	blocked := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM accounts WHERE id=%FOR UPDATE'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked || f.provider.refundCalls.Load() != 0 {
		t.Fatal("refund did not wait for the canonical owner lock before proof/debit")
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.NewBillingRepositoryTx(tx).AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, Type: domain.LedgerAdjustment, Amount: -1, IdempotencyKey: uuid.NewString(), Reason: "concurrent actual spend"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, paymentservice.ErrRefundCreditsSpent) {
		t.Fatalf("refund ignored concurrent committed spend: %v", err)
	}
	if f.provider.refundCalls.Load() != 0 || f.balance(t) != 257 {
		t.Fatal("concurrent spend/refund changed the wrong balance or submitted money")
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
}

type noReversalProofBilling struct{ domain.BillingRepository }
type errorReversalProofBilling struct {
	domain.BillingRepository
	err error
}

func (r errorReversalProofBilling) IsUnusedSubscriptionRefundReversal(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, r.err
}

func TestTopupRefundAfterSubscriptionReversalRequiresAvailableProof(t *testing.T) {
	for _, failProof := range []bool{false, true} {
		t.Run(map[bool]string{false: "capability_missing", true: "proof_read_failed"}[failProof], func(t *testing.T) {
			ctx := context.Background()
			f := newReversalRefundFixture(t)
			f.revoke(t)
			sentinel := errors.New("attribution read unavailable")
			runner := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
				return postgres.RunInTx(ctx, f.pool, func(ctx context.Context, tx pgx.Tx) error {
					var billing domain.BillingRepository = noReversalProofBilling{postgres.NewBillingRepositoryTx(tx)}
					if failProof {
						billing = errorReversalProofBilling{billing, sentinel}
					}
					return fn(ctx, postgres.NewPaymentRepositoryTx(tx), billing)
				})
			})
			processor := paymentservice.NewWebhookProcessor(f.payments, f.provider, billingservice.New(f.billing), runner)
			_, err := processor.RefundIntent(ctx, f.refundInput())
			if failProof && !errors.Is(err, sentinel) {
				t.Fatalf("proof error was not propagated: %v", err)
			}
			if !failProof && !errors.Is(err, paymentservice.ErrRefundCreditsSpent) {
				t.Fatalf("missing proof did not fail closed: %v", err)
			}
			if f.balance(t) != 258 || f.provider.refundCalls.Load() != 0 {
				t.Fatal("unavailable proof allowed a debit or provider call")
			}
		})
	}
}

func TestTopupRefundAfterSubscriptionReversalConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	f := newReversalRefundFixture(t)
	f.revoke(t)
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() { <-start; _, err := f.processor.RefundIntent(ctx, f.refundInput()); done <- err }()
	}
	close(start)
	successes := 0
	for range 2 {
		err := <-done
		if err == nil {
			successes++
		} else if !errors.Is(err, paymentservice.ErrRefundCreditsSpent) && !errors.Is(err, paymentservice.ErrRefundNotAllowed) && !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("unexpected concurrent refund error: %v", err)
		}
	}
	if successes == 0 || f.balance(t) != 60 {
		t.Fatal("concurrent exact retry lost or duplicated the refund")
	}
	var refunds, debits int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refunds WHERE intent_id=$1`, f.topup.ID).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE account_id=$1 AND type='adjustment' AND amount=-198`, account.ID).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 || debits != 1 {
		t.Fatalf("concurrent refund persisted refunds/debits=%d/%d want1/1", refunds, debits)
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
}
