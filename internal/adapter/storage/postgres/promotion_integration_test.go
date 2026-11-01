package postgres_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"path/filepath"
	"sync"
	"testing"
	"time"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/promotionservice"
)

func promotionFixture(t *testing.T, ctx context.Context) (*subscriptionPgFixture, *promotionservice.Service) {
	t.Helper()
	pool := rewardPool(t)
	f := &subscriptionPgFixture{pool: pool, owner: uuid.New(), billing: postgres.NewBillingRepository(pool), payments: postgres.NewPaymentRepository(pool), provider: &subscriptionPgProvider{Provider: paymentmock.New()}}
	if _, e := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, f.owner); e != nil {
		t.Fatal(e)
	}
	f.tx = &subscriptionPgTx{pool: pool}
	f.service = paymentservice.New(f.payments, f.provider, paymentservice.Config{SubscriptionStore: f.billing, SubscriptionBilling: f.billing, SubscriptionTx: f.tx, RecurringEnabled: true, AccountReturnURL: "https://epifera.test/app/payment-return"})
	f.processor = paymentservice.NewWebhookProcessor(f.payments, f.provider, billingservice.New(f.billing), f.tx)
	conn, e := pool.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Release()
	root, e := repoRoot()
	if e != nil {
		t.Fatal(e)
	}
	preparedRunMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000067_subscription_promotions.up.sql"))
	return f, promotionservice.New(f.tx, f.payments)
}
func createPromotion(t *testing.T, ctx context.Context, svc *promotionservice.Service, code string, max int64) *domain.Promotion {
	t.Helper()
	p, e := svc.Mutate(ctx, "operator_test", uuid.New(), &domain.Promotion{Code: code, Percent: 15, Enabled: true, StartsAt: time.Now().Add(-time.Hour), PlanCodes: []string{"start", "lite"}, MaxUses: max})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func promoCheckout(owner uuid.UUID, code string) paymentservice.CreateSubscriptionInput {
	return paymentservice.CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: uuid.NewString(), PromoCode: code, PromoRevision: 1}
}

func TestPromotionPostgresAtomicSettlementReplayAndRefundEligibility(t *testing.T) {
	ctx := context.Background()
	f, svc := promotionFixture(t, ctx)
	p := createPromotion(t, ctx, svc, "FIRST15", 1)
	in := promoCheckout(f.owner, p.Code)
	result, e := f.service.CreateSubscription(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	if result.Intent.Amount != 46665 || result.Intent.Credits != 1250 {
		t.Fatal("wrong discounted payment")
	}
	if _, e = f.service.CreateSubscription(ctx, in); e != nil {
		t.Fatal("replay", e)
	}
	if e = f.payments.CreateIntent(ctx, &domain.PaymentIntent{AccountID: f.owner, Status: domain.PaymentIntentCreated, Amount: 100, Credits: 2, Provider: domain.PaymentProviderMock, IdempotencyKey: uuid.NewString()}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("competing legacy writer accepted", e)
	}
	if e = f.provider.SetPaymentStatus(result.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); e != nil {
		t.Fatal(e)
	}
	f.tx.rollbackAfterCallback = errors.New("injected rollback")
	if _, e = f.processor.SyncIntent(ctx, result.Intent.ID); e == nil {
		t.Fatal("missing rollback")
	}
	f.tx.rollbackAfterCallback = nil
	current, e := f.payments.GetIntentByID(ctx, result.Intent.ID)
	if e != nil || current.Status == domain.PaymentIntentSucceeded {
		t.Fatal("status escaped rollback", e)
	}
	list, e := svc.List(ctx, 10, 0)
	if e != nil || list[0].ReservedCount != 1 || list[0].RedeemedCount != 0 {
		t.Fatal("usage escaped rollback", e, list)
	}
	for i := 0; i < 2; i++ {
		if _, e = f.processor.SyncIntent(ctx, result.Intent.ID); e != nil {
			t.Fatal(e)
		}
	}
	list, e = svc.List(ctx, 10, 0)
	if e != nil || list[0].ReservedCount != 0 || list[0].RedeemedCount != 1 {
		t.Fatal("consume not once", e, list)
	}
	state, e := f.service.Subscription(ctx, f.owner)
	if e != nil || state.Plan.Amount != 54900 || state.NextPlan.Amount != 54900 || state.Plan.Stars != 1250 {
		t.Fatal("full plan changed", e)
	}
	// A historical refunded status remains disqualifying even after state reset.
	if _, e = f.pool.Exec(ctx, `UPDATE payment_intents SET status='refunded' WHERE id=$1`, result.Intent.ID); e != nil {
		t.Fatal(e)
	}
	createPromotion(t, ctx, svc, "OTHER", 0)
	if _, e = f.service.QuotePromotion(ctx, f.owner, "OTHER"); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("refund reset eligibility", e)
	}
}

func TestPromotionPostgresGlobalCapacityConcurrencyAndCancellation(t *testing.T) {
	ctx := context.Background()
	f, svc := promotionFixture(t, ctx)
	p := createPromotion(t, ctx, svc, "CAP1", 1)
	owners := []uuid.UUID{f.owner, uuid.New()}
	if _, e := f.pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owners[1]); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan paymentservice.CreateIntentResult, 2)
	errs := make(chan error, 2)
	for _, owner := range owners {
		wg.Add(1)
		go func(owner uuid.UUID) {
			defer wg.Done()
			<-start
			r, e := f.service.CreateSubscription(ctx, promoCheckout(owner, p.Code))
			results <- r
			errs <- e
		}(owner)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	success, conflicts := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("global cap success=%d conflicts=%d", success, conflicts)
	}
	var winner paymentservice.CreateIntentResult
	for r := range results {
		if r.Intent != nil {
			winner = r
		}
	}
	if e := f.provider.SetPaymentStatus(winner.Intent.ProviderPaymentID, domain.PaymentIntentCanceled); e != nil {
		t.Fatal(e)
	}
	if _, e := f.processor.SyncIntent(ctx, winner.Intent.ID); e != nil {
		t.Fatal(e)
	}
	list, e := svc.List(ctx, 10, 0)
	if e != nil || list[0].ReservedCount != 0 || list[0].RedeemedCount != 0 {
		t.Fatal("cancel did not release", e)
	}
	if _, e = f.service.CreateSubscription(ctx, promoCheckout(winner.Intent.AccountID, p.Code)); e != nil {
		t.Fatal("verified cancellation prevents retry", e)
	}
}

func TestPromotionPostgresOwnerGateRollbackAndHistoricalGrant(t *testing.T) {
	ctx := context.Background()
	f, svc := promotionFixture(t, ctx)
	createPromotion(t, ctx, svc, "ROLLBACK", 0)
	sentinel := errors.New("stop after reserve")
	err := f.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, _ domain.BillingRepository) error {
		r := payments.(domain.PromotionRepository)
		_, e := promotionservice.ReserveLocked(ctx, r, f.owner, uuid.New(), "ROLLBACK", 1, "start", time.Now())
		if e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	list, e := svc.List(ctx, 10, 0)
	if e != nil || list[0].ReservedCount != 0 {
		t.Fatal("reservation escaped rollback", e)
	}
	// Historical ledger is authoritative independently of payment/referral rows.
	account := &domain.CreditAccount{OwnerAccountID: f.owner, Currency: domain.CurrencyCredits}
	if e = f.billing.CreateAccount(ctx, account); e != nil {
		t.Fatal(e)
	}
	if e = f.billing.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, OwnerAccountID: f.owner, Type: domain.LedgerTopup, Status: domain.LedgerStatusCommitted, Amount: 50, CreditDenominationVersion: 2, IdempotencyKey: "topup:mock:" + uuid.NewString()}); e != nil {
		t.Fatal(e)
	}
	if _, e = f.service.QuotePromotion(ctx, f.owner, "ROLLBACK"); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("historical paid grant ignored", e)
	}
}

func TestPromotionPostgresAdminAuditAtomicReplayAndStaleRevision(t *testing.T) {
	ctx := context.Background()
	f, svc := promotionFixture(t, ctx)
	key := uuid.New()
	config := domain.Promotion{Code: "ADMIN15", Percent: 15, Enabled: true, StartsAt: time.Now().UTC().Add(-time.Hour), PlanCodes: []string{"start"}}
	f.tx.rollbackAfterCallback = errors.New("audit rollback")
	if _, e := svc.Mutate(ctx, "operator_test", key, &config); e == nil {
		t.Fatal("rollback ignored")
	}
	f.tx.rollbackAfterCallback = nil
	list, e := svc.List(ctx, 10, 0)
	if e != nil || len(list) != 0 {
		t.Fatal("config escaped audit rollback", e)
	}
	p, e := svc.Mutate(ctx, "operator_test", key, &config)
	if e != nil {
		t.Fatal(e)
	}
	again, e := svc.Mutate(ctx, "operator_test", key, &config)
	if e != nil || again.ID != p.ID {
		t.Fatal("durable replay", e)
	}
	config.Percent = 30
	if _, e = svc.Mutate(ctx, "operator_test", key, &config); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("conflicting replay accepted", e)
	}
	var count int
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM subscription_promotion_mutations`).Scan(&count); e != nil || count != 1 {
		t.Fatal("audit count", count, e)
	}
	p.Percent = 25
	if _, e = svc.Mutate(ctx, "operator_test", uuid.New(), p); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Mutate(ctx, "operator_test", uuid.New(), p); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("stale update", e)
	}
	// An account lock wins over another normal intent writer before promo reserve.
	other := uuid.New()
	if _, e = f.pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, other); e != nil {
		t.Fatal(e)
	}
	if e = postgres.RunInTx(ctx, f.pool, func(ctx context.Context, tx pgx.Tx) error {
		r := postgres.NewPaymentRepositoryTx(tx)
		return r.CreateIntent(ctx, &domain.PaymentIntent{AccountID: other, Status: domain.PaymentIntentCreated, Amount: 100, Credits: 2, PriceVersion: 1, ReceiptEmail: "test@example.test", Provider: domain.PaymentProviderMock, IdempotencyKey: uuid.NewString()})
	}); e != nil {
		t.Fatal(e)
	}
	if _, e = f.service.QuotePromotion(ctx, other, p.Code); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("unresolved normal payment ignored", e)
	}
}

func TestPromotionPostgresAccountSerializesDifferentCodes(t *testing.T) {
	ctx := context.Background()
	f, svc := promotionFixture(t, ctx)
	createPromotion(t, ctx, svc, "CODEONE", 0)
	createPromotion(t, ctx, svc, "CODETWO", 0)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, code := range []string{"CODEONE", "CODETWO"} {
		go func(code string) {
			<-start
			_, err := f.service.CreateSubscription(ctx, promoCheckout(f.owner, code))
			errs <- err
		}(code)
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		err := <-errs
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("same owner: success=%d conflict=%d", success, conflict)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM subscription_promotion_uses WHERE account_id=$1 AND state='reserved'`, f.owner).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestPromotionPostgresMissingSchemaFailsClosedOnlyForPromo(t *testing.T) {
	ctx := context.Background()
	f := newSubscriptionPgFixture(t, ctx)
	if _, err := f.service.QuotePromotion(ctx, f.owner, "MISSING"); err == nil {
		t.Fatal("missing schema accepted")
	}
	if _, err := f.service.CreateSubscription(ctx, promoCheckout(f.owner, "MISSING")); err == nil {
		t.Fatal("missing schema discounted purchase accepted")
	}
	input := promoCheckout(f.owner, "")
	input.PromoRevision = 0
	if _, err := f.service.CreateSubscription(ctx, input); err != nil {
		t.Fatal("ordinary legacy checkout unavailable", err)
	}
}
