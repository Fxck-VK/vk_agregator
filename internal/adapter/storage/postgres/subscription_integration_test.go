package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// Each test uses its own schema inside TEST_DATABASE_URL, never the app DB.
func subscriptionIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := preparedAccountIntegrationPool(t, ctx)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000063_credit_lots.up.sql", "000064_account_subscriptions.up.sql"} {
		preparedRunMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", name))
	}
	return pool
}

func TestSubscriptionPostgresGetLockSaveCASAndRollback(t *testing.T) {
	ctx := context.Background()
	pool := subscriptionIntegrationPool(t, ctx)
	repo := postgres.NewBillingRepository(pool)
	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.GetSubscription(ctx, owner); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing subscription error=%v wantErrNotFound", err)
	}
	paidUntil := time.Now().UTC().Truncate(time.Microsecond).Add(7 * 24 * time.Hour)
	retryAt := paidUntil.Add(15 * time.Minute)
	if err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		bound := postgres.NewBillingRepositoryTx(tx)
		state, err := bound.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		if state.AccountID != owner || state.Revision != 0 {
			t.Fatalf("initial locked owner/revision %s/%d", state.AccountID, state.Revision)
		}
		stale := *state
		state.Plan = domain.SubscriptionPlanSnapshot{Code: "lite", Name: "Lite", Amount: 19900, Stars: 400, PeriodDays: 7, Trial: true, NextPlanCode: "start", PolicyVersion: 1}
		state.PaidUntil = paidUntil
		state.AutoRenew = true
		state.NextAttemptAt = retryAt
		if err := bound.SaveSubscription(ctx, state); err != nil {
			return err
		}
		if state.Revision != 1 {
			t.Fatalf("saved revision %d", state.Revision)
		}
		if err := bound.SaveSubscription(ctx, &stale); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("stale CAS error %v", err)
		}
		_, err = tx.Exec(ctx, `SELECT 1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	state, err := repo.GetSubscription(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || state.Plan.Code != "lite" || !state.PaidUntil.Equal(paidUntil) || !state.AutoRenew {
		t.Fatal("saved subscription snapshot did not round trip")
	}
	if _, err := repo.GetSubscription(ctx, foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign owner read %v", err)
	}
	due, err := repo.ListDueSubscriptions(ctx, paidUntil, 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("renewal ignored retry deadline: %d/%v", len(due), err)
	}
	due, err = repo.ListDueSubscriptions(ctx, retryAt, 10)
	if err != nil || len(due) != 1 || due[0] != owner {
		t.Fatalf("due subscription %v/%v", due, err)
	}
	sentinel := errors.New("subscription outer rollback")
	err = postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		bound := postgres.NewBillingRepositoryTx(tx)
		state, err := bound.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		state.AutoRenew = false
		state.Plan.Code = "pro"
		if err := bound.SaveSubscription(ctx, state); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	state, err = repo.GetSubscription(ctx, owner)
	if err != nil || state.Revision != 1 || !state.AutoRenew || state.Plan.Code != "lite" {
		t.Fatal("outer rollback changed subscription state")
	}
}

// This adapter is entirely in memory. Its YooKassa code exercises recurring
// consent logic while every payment operation still delegates to the mock.
type subscriptionPgProvider struct {
	*paymentmock.Provider
	amountDelta  int64
	cancelReason string
	creates      int
	afterCreate  func(domain.CreatePaymentInput, domain.CreatePaymentResult) error
	bindings     map[string]domain.CreatePaymentInput
}

func (*subscriptionPgProvider) Code() domain.PaymentProviderCode {
	return domain.PaymentProviderYooKassa
}
func (p *subscriptionPgProvider) CreatePayment(ctx context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	p.creates++
	result, err := p.Provider.CreatePayment(ctx, in)
	if err == nil && p.afterCreate != nil {
		err = p.afterCreate(in, result)
	}
	return result, err
}
func (p *subscriptionPgProvider) GetPayment(ctx context.Context, id string) (domain.ProviderPayment, error) {
	payment, err := p.Provider.GetPayment(ctx, id)
	payment.Amount += p.amountDelta
	payment.CancellationReason = p.cancelReason
	payment.PaymentMethodID = "synthetic-test-method"
	payment.PaymentMethodSaved = true
	if binding, ok := p.bindings[id]; ok {
		payment.VerifiedIntentID = binding.IntentID
		payment.VerifiedAccountID = binding.AccountID
	}
	return payment, err
}

type subscriptionPgTx struct {
	pool                  *pgxpool.Pool
	rollbackAfterCallback error
}

func (r *subscriptionPgTx) RunPaymentTx(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
	return postgres.RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := fn(ctx, postgres.NewPaymentRepositoryTx(tx), postgres.NewBillingRepositoryTx(tx)); err != nil {
			return err
		}
		return r.rollbackAfterCallback
	})
}

type subscriptionPgFixture struct {
	pool      *pgxpool.Pool
	owner     uuid.UUID
	billing   *postgres.BillingRepository
	payments  *postgres.PaymentRepository
	provider  *subscriptionPgProvider
	tx        *subscriptionPgTx
	service   *paymentservice.Service
	processor *paymentservice.WebhookProcessor
}

func newSubscriptionPgFixture(t *testing.T, ctx context.Context) *subscriptionPgFixture {
	t.Helper()
	pool := subscriptionIntegrationPool(t, ctx)
	f := &subscriptionPgFixture{pool: pool, owner: uuid.New(), billing: postgres.NewBillingRepository(pool), payments: postgres.NewPaymentRepository(pool), provider: &subscriptionPgProvider{Provider: paymentmock.New()}}
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, f.owner); err != nil {
		t.Fatal(err)
	}
	f.tx = &subscriptionPgTx{pool: pool}
	f.service = paymentservice.New(f.payments, f.provider, paymentservice.Config{SubscriptionStore: f.billing, SubscriptionBilling: f.billing, SubscriptionTx: f.tx, RecurringEnabled: true, AccountReturnURL: "https://epifera.test/app/payment-return"})
	f.processor = paymentservice.NewWebhookProcessor(f.payments, f.provider, billingservice.New(f.billing), f.tx)
	return f
}

func TestSubscriptionPostgresVerifiedSettlementReplayTrialAndCancel(t *testing.T) {
	ctx := context.Background()
	f := newSubscriptionPgFixture(t, ctx)
	input := paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "subscription-pg:" + uuid.NewString()}
	checkout, err := f.service.CreateSubscription(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Intent.Amount != 19900 || checkout.Intent.Credits != 400 || checkout.Intent.Status != domain.PaymentIntentWaitingForUser {
		t.Fatal("checkout snapshot differs from the Lite offer")
	}
	if _, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("checkout granted credits before provider verification: %v", err)
	}
	replay, err := f.service.CreateSubscription(ctx, input)
	if err != nil || replay.Intent.ID != checkout.Intent.ID || replay.Created || f.provider.creates != 1 {
		t.Fatalf("checkout replay differs: %v", err)
	}
	changed := input
	changed.PlanCode = "pro"
	changed.IdempotencyKey = uuid.NewString()
	if _, err := f.service.CreateSubscription(ctx, changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second pending subscription accepted: %v", err)
	}
	if err := f.provider.SetPaymentStatus(checkout.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	if _, err := f.processor.SyncIntent(ctx, checkout.Intent.ID); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()
	state, err := f.service.Subscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Plan.Code != "lite" || !state.TrialUsed || !state.AutoRenew || state.ActiveIntentID != checkout.Intent.ID || state.PendingIntentID != uuid.Nil || state.PeriodStart.Before(before) || state.PeriodStart.After(after) || !state.PaidUntil.Equal(state.PeriodStart.AddDate(0, 0, 7)) {
		t.Fatal("verified settlement did not persist the paid Lite period and recurring consent")
	}
	paidUntil := state.PaidUntil
	revision := state.Revision
	if _, err := f.processor.SyncIntent(ctx, checkout.Intent.ID); err != nil {
		t.Fatal(err)
	}
	state, err = f.service.Subscription(ctx, f.owner)
	if err != nil || state.Revision != revision || !state.PaidUntil.Equal(paidUntil) {
		t.Fatal("settlement replay changed the subscription period")
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil || account.BalanceCached != 460 {
		t.Fatalf("settled balance expected460: %v", err)
	}
	lots, err := f.billing.CreditLotBalancesForOwner(ctx, f.owner, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	subscriptionLots := 0
	for _, lot := range lots {
		if lot.Kind == domain.CreditLotSubscription {
			subscriptionLots++
			if lot.AvailableCredits != 400 || lot.ExpiresAt == nil || !lot.ExpiresAt.Equal(paidUntil.Truncate(time.Microsecond)) {
				t.Fatal("subscription lot does not match its paid period")
			}
		}
	}
	if subscriptionLots != 1 {
		t.Fatalf("subscription lot count=%d want1", subscriptionLots)
	}
	var grants int
	if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE account_id=$1 AND lot_kind='subscription' AND status='committed'`, account.ID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants != 1 {
		t.Fatalf("subscription grant count=%d want1", grants)
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
	cancelled, err := f.service.CancelSubscription(ctx, f.owner)
	if err != nil || cancelled.AutoRenew || cancelled.PaymentMethodID != "" || cancelled.CancelledAt == nil || !cancelled.PaidUntil.Equal(paidUntil) {
		t.Fatalf("cancellation lost paid access or retained renewal: %v", err)
	}
	again, err := f.service.CancelSubscription(ctx, f.owner)
	if err != nil || again == nil || again.CancelledAt == nil || !again.PaidUntil.Equal(paidUntil) || !again.CancelledAt.Equal(*cancelled.CancelledAt) {
		t.Fatalf("cancellation replay changed paid access: %v", err)
	}
	// Advance the stored test period and its lot together; no wall-clock wait or
	// production clock override is needed to verify one trial per real owner.
	expired := time.Now().UTC().Add(-time.Minute)
	if err := f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, billing domain.BillingRepository) error {
		store := billing.(domain.SubscriptionRepository)
		state, err := store.LockSubscription(ctx, f.owner)
		if err != nil {
			return err
		}
		state.PaidUntil = expired
		return store.SaveSubscription(ctx, state)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE credit_lots SET expires_at=$2 WHERE account_id=$1 AND kind='subscription'`, account.ID, expired); err != nil {
		t.Fatal(err)
	}
	input.IdempotencyKey = uuid.NewString()
	if _, err := f.service.CreateSubscription(ctx, input); !errors.Is(err, paymentservice.ErrTrialAlreadyUsed) {
		t.Fatalf("trial reuse error=%v", err)
	}
	var intents int
	if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_intents WHERE account_id=$1`, f.owner).Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if intents != 1 || f.provider.creates != 1 {
		t.Fatal("trial rejection created another intent or submitted payment")
	}
}

func TestSubscriptionPostgresMismatchAndSettlementRollback(t *testing.T) {
	ctx := context.Background()
	f := newSubscriptionPgFixture(t, ctx)
	checkout, err := f.service.CreateSubscription(ctx, paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "subscription-pg-rollback:" + uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := f.service.Subscription(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.provider.SetPaymentStatus(checkout.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	f.provider.amountDelta = 1
	if _, err := f.processor.SyncIntent(ctx, checkout.Intent.ID); !errors.Is(err, paymentservice.ErrWebhookMismatch) {
		t.Fatalf("wrong amount accepted: %v", err)
	}
	f.provider.amountDelta = 0
	assertSubscriptionStillPending(t, ctx, f, checkout.Intent.ID, initial.Revision)
	sentinel := errors.New("injected settlement transaction rollback")
	f.tx.rollbackAfterCallback = sentinel
	if _, err := f.processor.SyncIntent(ctx, checkout.Intent.ID); !errors.Is(err, sentinel) {
		t.Fatalf("injected rollback result %v", err)
	}
	f.tx.rollbackAfterCallback = nil
	assertSubscriptionStillPending(t, ctx, f, checkout.Intent.ID, initial.Revision)
	if _, err := f.processor.SyncIntent(ctx, checkout.Intent.ID); err != nil {
		t.Fatal(err)
	}
	account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil || account.BalanceCached != 1310 {
		t.Fatalf("settlement after rollback expected1310: %v", err)
	}
	assertPostgresLotProjection(t, ctx, f.pool, account.ID)
}

func assertSubscriptionStillPending(t *testing.T, ctx context.Context, f *subscriptionPgFixture, intentID uuid.UUID, revision int64) {
	t.Helper()
	state, err := f.service.Subscription(ctx, f.owner)
	if err != nil || state == nil || !state.PaidUntil.IsZero() || state.ActiveIntentID != uuid.Nil || state.PendingIntentID != intentID || state.Revision != revision {
		t.Fatalf("failed settlement changed subscription: %v", err)
	}
	if _, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("failed settlement left a credit account or grant: %v", err)
	}
	intent, err := f.payments.GetIntentByID(ctx, intentID)
	if err != nil || intent.Status != domain.PaymentIntentWaitingForUser {
		t.Fatalf("failed settlement changed payment status: %v", err)
	}
}

func TestSubscriptionPostgresDurableSubmitMarkerSurvivesAcceptedFailure(t *testing.T) {
	for _, mode := range []string{"accepted_response_error", "accepted_submit_transaction_rollback"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f := newSubscriptionPgFixture(t, ctx)
			initial, err := f.service.CreateSubscription(ctx, paymentservice.CreateSubscriptionInput{AccountID: f.owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "subscription-marker:" + uuid.NewString()})
			if err != nil {
				t.Fatal(err)
			}
			if err := f.provider.SetPaymentStatus(initial.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
				t.Fatal(err)
			}
			if _, err := f.processor.SyncIntent(ctx, initial.Intent.ID); err != nil {
				t.Fatal(err)
			}
			expired := time.Now().UTC().Add(-time.Minute)
			if err := f.tx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, billing domain.BillingRepository) error {
				store := billing.(domain.SubscriptionRepository)
				state, err := store.LockSubscription(ctx, f.owner)
				if err != nil {
					return err
				}
				state.PaidUntil = expired
				return store.SaveSubscription(ctx, state)
			}); err != nil {
				t.Fatal(err)
			}
			account, err := f.billing.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
			if err != nil {
				t.Fatal(err)
			}
			var baselineEntries int
			if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE account_id=$1`, account.ID).Scan(&baselineEntries); err != nil {
				t.Fatal(err)
			}
			sentinel := errors.New("synthetic accepted submission failure")
			var accepted domain.CreatePaymentInput
			var acceptedProviderID string
			f.provider.bindings = map[string]domain.CreatePaymentInput{}
			f.provider.afterCreate = func(in domain.CreatePaymentInput, result domain.CreatePaymentResult) error {
				if in.PaymentMethodID == "" || in.SavePaymentMethod {
					t.Fatal("renewal did not use the saved mock method")
				}
				accepted = in
				acceptedProviderID = result.ProviderPaymentID
				f.provider.bindings[acceptedProviderID] = in
				if mode == "accepted_response_error" {
					return sentinel
				}
				f.tx.rollbackAfterCallback = sentinel
				return nil
			}
			before := time.Now().UTC()
			processed, err := f.service.ProcessSubscriptionRenewals(ctx, 10)
			after := time.Now().UTC()
			f.provider.afterCreate = nil
			f.tx.rollbackAfterCallback = nil
			if !errors.Is(err, sentinel) || processed != 0 || accepted.IntentID == uuid.Nil || acceptedProviderID == "" || f.provider.creates != 2 {
				t.Fatalf("accepted submit failure result processed=%d err=%v", processed, err)
			}
			if accepted.AccountID != f.owner || accepted.Amount != 54900 || accepted.Credits != 1250 {
				t.Fatal("renewal submission lost its canonical owner or next-plan snapshot")
			}
			pending, err := f.payments.GetIntentByID(ctx, accepted.IntentID)
			if err != nil {
				t.Fatal(err)
			}
			if pending.Status != domain.PaymentIntentCreated || pending.ProviderPaymentID != "" {
				t.Fatal("submit rollback left a local provider binding/status")
			}
			var metadata struct {
				Started string `json:"subscription_submit_started_at"`
			}
			if err := json.Unmarshal(pending.Metadata, &metadata); err != nil {
				t.Fatal(err)
			}
			started, err := time.Parse(time.RFC3339Nano, metadata.Started)
			if err != nil || started.IsZero() || started.Before(before) || started.After(after) {
				t.Fatalf("durable submission marker missing after rollback: %v", err)
			}
			state, err := f.service.Subscription(ctx, f.owner)
			if err != nil || state.PendingIntentID != pending.ID || state.ActiveIntentID != initial.Intent.ID {
				t.Fatalf("submit rollback lost pending renewal: %v", err)
			}
			var entries int
			if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE account_id=$1`, account.ID).Scan(&entries); err != nil {
				t.Fatal(err)
			}
			fresh, err := f.billing.GetAccount(ctx, account.ID)
			if err != nil || fresh.BalanceCached != 460 || entries != baselineEntries {
				t.Fatal("unverified accepted payment granted renewal stars")
			}
			cancelled, err := f.service.CancelSubscription(ctx, f.owner)
			if err != nil || cancelled.PendingIntentID != pending.ID || cancelled.AutoRenew || cancelled.CancelledAt == nil || cancelled.PaymentMethodID != "" {
				t.Fatalf("cancellation retired an ambiguous submitted renewal: %v", err)
			}
			pending, err = f.payments.GetIntentByID(ctx, pending.ID)
			if err != nil || pending.Status != domain.PaymentIntentCreated || pending.ProviderPaymentID != "" {
				t.Fatalf("cancellation destroyed recoverable renewal: %v", err)
			}
			fresh, err = f.billing.GetAccount(ctx, account.ID)
			if err != nil || fresh.BalanceCached != 460 {
				t.Fatal("cancellation granted unverified renewal stars")
			}
			// Only the authenticated provider GET supplies recovery IDs. Deliberately
			// conflicting notification payload metadata cannot establish ownership.
			if err := f.provider.SetPaymentStatus(acceptedProviderID, domain.PaymentIntentSucceeded); err != nil {
				t.Fatal(err)
			}
			event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "payment.succeeded", ProviderPaymentID: acceptedProviderID, DedupKey: "subscription-marker-recovery:" + uuid.NewString(), Payload: json.RawMessage(`{"metadata":{"intent_id":"untrusted","account_id":"untrusted"}}`)}
			if _, err := f.payments.CreateEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := f.processor.ProcessEvent(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			recovered, err := f.payments.GetIntentByID(ctx, pending.ID)
			if err != nil || recovered.Status != domain.PaymentIntentSucceeded || recovered.ProviderPaymentID != acceptedProviderID {
				t.Fatalf("late verified provider GET did not recover payment binding: %v", err)
			}
			state, err = f.service.Subscription(ctx, f.owner)
			if err != nil || state.Plan.Code != "start" || state.ActiveIntentID != pending.ID || state.PendingIntentID != uuid.Nil || state.AutoRenew || state.PaymentMethodID != "" || state.CancelledAt == nil || !state.PaidUntil.Equal(state.PeriodStart.AddDate(0, 0, 30)) {
				t.Fatalf("late settlement lost paid access or undid cancellation: %v", err)
			}
			fresh, err = f.billing.GetAccount(ctx, account.ID)
			if err != nil || fresh.BalanceCached != 1710 {
				t.Fatalf("late verified grant/replay expected1710: %v", err)
			}
			if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE account_id=$1`, account.ID).Scan(&entries); err != nil {
				t.Fatal(err)
			}
			if entries != baselineEntries+1 {
				t.Fatal("late verified settlement replay duplicated its grant")
			}
			assertPostgresLotProjection(t, ctx, f.pool, account.ID)
		})
	}
}
