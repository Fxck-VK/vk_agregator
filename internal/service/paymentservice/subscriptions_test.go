package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

type subscriptionProvider struct {
	domain.PaymentProvider
	inputs    []domain.CreatePaymentInput
	immediate bool
	refund    domain.VerifiedPaymentRefund
}

func (p *subscriptionProvider) GetRefund(context.Context, string) (domain.VerifiedPaymentRefund, error) {
	return p.refund, nil
}

func (p *subscriptionProvider) Code() domain.PaymentProviderCode {
	return domain.PaymentProviderYooKassa
}
func (p *subscriptionProvider) CreatePayment(ctx context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	p.inputs = append(p.inputs, in)
	result, err := p.PaymentProvider.CreatePayment(ctx, in)
	if p.immediate {
		result.Status = domain.PaymentIntentSucceeded
	}
	return result, err
}

func subscriptionFixture(t *testing.T) (*Service, *WebhookProcessor, *memory.BillingRepo, *memory.PaymentRepo, uuid.UUID) {
	t.Helper()
	repo, credits, provider := memory.NewPaymentRepo(), memory.NewBillingRepo(), &subscriptionProvider{PaymentProvider: paymentmock.New()}
	tx := TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, repo, credits)
	})
	svc := New(repo, provider, Config{AccountReturnURL: "https://epifera.test/app/payment-return", SubscriptionStore: credits, SubscriptionBilling: credits, SubscriptionTx: tx, RecurringEnabled: true})
	processor := NewWebhookProcessor(repo, provider, billingservice.New(credits), tx)
	return svc, processor, credits, repo, uuid.New()
}

func TestSubscriptionCheckoutDoesNotGrantBeforeVerifiedPayment(t *testing.T) {
	svc, _, credits, _, owner := subscriptionFixture(t)
	result, err := svc.CreateSubscription(context.Background(), CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent.Amount != 54900 || result.Intent.Credits != 1250 {
		t.Fatalf("wrong immutable purchase: %#v", result.Intent)
	}
	if _, err := credits.GetAccountByOwner(context.Background(), owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("checkout unexpectedly created balance: %v", err)
	}
	replay, err := svc.CreateSubscription(context.Background(), CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:test"})
	if err != nil || replay.Intent.ID != result.Intent.ID {
		t.Fatalf("checkout replay differs: %v", err)
	}
	_, err = svc.CreateSubscription(context.Background(), CreateSubscriptionInput{AccountID: owner, PlanCode: "pro", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:test"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}
}

func TestSubscriptionVerifiedSettlementGrantsOneExpiringLot(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	result, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:lite"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	processor.now = func() time.Time { return now }
	verified := domain.ProviderPayment{ProviderPaymentID: result.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true}
	for i := 0; i < 2; i++ {
		intent, _ := repo.GetIntentByID(ctx, result.Intent.ID)
		if _, err := processor.applyProviderPayment(ctx, repo, credits, intent, verified); err != nil {
			t.Fatal(err)
		}
	}
	state, err := svc.Subscription(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if state.Plan.Code != "lite" || !state.TrialUsed || !state.PaidUntil.Equal(now.AddDate(0, 0, 7)) {
		t.Fatalf("wrong period: %#v", state)
	}
	account, err := credits.GetAccountByOwner(ctx, owner, domain.CurrencyCredits)
	if err != nil || account.BalanceCached != 460 {
		t.Fatalf("duplicate or missing credit: %#v %v", account, err)
	}
	lots, err := credits.CreditLotBalancesForOwner(ctx, owner, now)
	var subscriptionLots []domain.CreditLotBalance
	for _, lot := range lots {
		if lot.Kind == domain.CreditLotSubscription {
			subscriptionLots = append(subscriptionLots, lot)
		}
	}
	if err != nil || len(subscriptionLots) != 1 || subscriptionLots[0].ExpiresAt == nil || !subscriptionLots[0].ExpiresAt.Equal(state.PaidUntil) {
		t.Fatalf("missing expiring lot: %#v %v", lots, err)
	}
}

func TestSubscriptionRejectsUnpaidSettlementAndConcurrentCheckout(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	first, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "pro", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:one"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "elite", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:two"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("parallel purchase accepted: %v", err)
	}
	bad := domain.ProviderPayment{ProviderPaymentID: first.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 99900, Currency: domain.CurrencyRUB, Paid: false, Captured: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, first.Intent, bad); !errors.Is(err, ErrWebhookUnverified) {
		t.Fatalf("unpaid activated: %v", err)
	}
	state, err := svc.Subscription(ctx, owner)
	if err != nil || !state.PaidUntil.IsZero() {
		t.Fatalf("unpaid state active: %#v %v", state, err)
	}
}

func TestSubscriptionTrialRenewalUsesOneNextPaymentAndPreservesConsent(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	provider := svc.provider.(*subscriptionProvider)
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:trial-recurring"})
	if err != nil {
		t.Fatal(err)
	}
	if !provider.inputs[0].SavePaymentMethod || provider.inputs[0].PaymentMethodID != "" {
		t.Fatal("initial explicit consent missing")
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	state.PaidUntil = time.Now().UTC().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.ProcessSubscriptionRenewals(ctx, 10); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.inputs) != 2 {
		t.Fatalf("duplicate renewal submit: %d", len(provider.inputs))
	}
	renewalInput := provider.inputs[1]
	if renewalInput.Amount != 54900 || renewalInput.Credits != 1250 || renewalInput.PaymentMethodID != "saved-method" || renewalInput.SavePaymentMethod {
		t.Fatal("renewal did not use trial next-plan snapshot")
	}
	renewal, _ := repo.GetIntentByID(ctx, renewalInput.IntentID)
	verified.ProviderPaymentID = renewal.ProviderPaymentID
	verified.Amount = 54900
	if _, err := processor.applyProviderPayment(ctx, repo, credits, renewal, verified); err != nil {
		t.Fatal(err)
	}
	state, _ = svc.Subscription(ctx, owner)
	if state.Plan.Code != "start" || !state.AutoRenew || !state.TrialUsed || state.PendingIntentID != uuid.Nil {
		t.Fatalf("incorrect renewed state: %#v", state)
	}
	paidUntil := state.PaidUntil
	old, _ := repo.GetIntentByID(ctx, initial.Intent.ID)
	oldVerified := verified
	oldVerified.Amount = 19900
	oldVerified.ProviderPaymentID = old.ProviderPaymentID
	if _, err := processor.applyProviderPayment(ctx, repo, credits, old, oldVerified); err != nil {
		t.Fatalf("historical paid callback not idempotent: %v", err)
	}
	state, _ = svc.Subscription(ctx, owner)
	if !state.PaidUntil.Equal(paidUntil) || state.Plan.Code != "start" {
		t.Fatal("historical callback changed current period")
	}
	if _, err := svc.CancelSubscription(ctx, owner); err != nil {
		t.Fatal(err)
	}
	state, _ = svc.Subscription(ctx, owner)
	until := state.PaidUntil
	state.PaidUntil = time.Now().UTC().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProcessSubscriptionRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if len(provider.inputs) != 2 {
		t.Fatal("cancelled subscription submitted payment")
	}
	state.PaidUntil = until
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	if state.PaymentMethodID != "" {
		t.Fatal("cancel retained chargeable method")
	}
}

func TestSubscriptionCancelledBeforeSuccessDoesNotEnableRecurringOrReuseTrial(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:cancel-before-paid"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelSubscription(ctx, owner); err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	if state.AutoRenew || state.PaymentMethodID != "" || state.PaidUntil.IsZero() {
		t.Fatal("late payment undid cancellation or failed paid access")
	}
	state.PaidUntil = time.Now().UTC().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:second-trial"})
	if !errors.Is(err, ErrTrialAlreadyUsed) {
		t.Fatalf("trial reused: %v", err)
	}
}

func TestSubscriptionImmediateCreateSuccessStillRequiresVerifiedSettlement(t *testing.T) {
	svc, _, credits, _, owner := subscriptionFixture(t)
	svc.provider.(*subscriptionProvider).immediate = true
	result, err := svc.CreateSubscription(context.Background(), CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:immediate"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent.Status != domain.PaymentIntentProviderPending {
		t.Fatalf("unverified create stored terminal: %s", result.Intent.Status)
	}
	if _, err := credits.GetAccountByOwner(context.Background(), owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("create response granted stars")
	}
}

func TestSubscriptionAmbiguousInitialCreationDoesNotRetryAfterProviderKeyExpiry(t *testing.T) {
	svc, _, _, _, owner := subscriptionFixture(t)
	provider := svc.provider.(*subscriptionProvider)
	initial, err := svc.CreateSubscription(context.Background(), CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:old"})
	if err != nil {
		t.Fatal(err)
	}
	old := *initial.Intent
	old.ID = uuid.New()
	old.ProviderPaymentID = ""
	old.ConfirmationURL = ""
	old.Status = domain.PaymentIntentCreated
	old.CreatedAt = time.Now().Add(-24 * time.Hour)
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(old.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata[subscriptionSubmitMarker], _ = json.Marshal(old.CreatedAt.UTC().Format(time.RFC3339Nano))
	old.Metadata, _ = json.Marshal(metadata)
	before := len(provider.inputs)
	_, err = svc.ensureProviderPayment(context.Background(), &old, nil, svc.cfg.AccountReturnURL)
	if !errors.Is(err, ErrSubscriptionUnavailable) || len(provider.inputs) != before {
		t.Fatalf("ambiguous old charge retried: %v calls=%d", err, len(provider.inputs)-before)
	}
}

func TestSubscriptionExternalRefundRevokesOnlyItsUnspentLot(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:external-refund"})
	if err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	account, _ := credits.GetAccountByOwner(ctx, owner, domain.CurrencyCredits)
	if err := credits.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, OwnerAccountID: owner, Type: domain.LedgerTopup, Amount: 1000, Status: domain.LedgerStatusCommitted, IdempotencyKey: "purchased:keep", LotKind: domain.CreditLotPurchased}); err != nil {
		t.Fatal(err)
	}
	reservation := &domain.CreditReservation{AccountID: account.ID, OwnerAccountID: owner, JobID: uuid.New(), Amount: 200, Status: domain.ReservationReserved, IdempotencyKey: "consume-sub", ExpiresAt: time.Now().Add(time.Hour)}
	if err := credits.ReserveForOwner(ctx, owner, reservation); err != nil {
		t.Fatal(err)
	}
	if err := credits.CaptureForOwner(ctx, owner, reservation.ID, 200, "capture-sub"); err != nil {
		t.Fatal(err)
	}
	provider := svc.provider.(*subscriptionProvider)
	provider.refund = domain.VerifiedPaymentRefund{ID: "refund-verified", PaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: 19900, Currency: domain.CurrencyRUB}
	event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "refund.succeeded", ProviderPaymentID: initial.Intent.ProviderPaymentID, ProviderRefundID: "refund-verified", DedupKey: "sub:refund"}
	if _, err := repo.CreateEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := processor.ProcessEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	account, _ = credits.GetAccountByOwner(ctx, owner, domain.CurrencyCredits)
	if account.BalanceCached != 1060 {
		t.Fatalf("refund touched other lots or left unused sub stars: %d", account.BalanceCached)
	}
	state, _ := svc.Subscription(ctx, owner)
	if state.AutoRenew || state.PaidUntil.After(time.Now()) || !state.TrialUsed {
		t.Fatalf("refund did not revoke paid access: %#v", state)
	}
}

func TestSubscriptionCancelClearsNeverSubmittedRenewal(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:queued-cancel"})
	if err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	purchase := subscriptionPurchase{Plan: state.NextPlan, Next: state.NextPlan, AutoRenew: true, Renewal: true, PreviousIntentID: state.ActiveIntentID}
	raw, _ := json.Marshal(map[string]any{"subscription": purchase})
	pending := &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Amount: 54900, Credits: 1250, Currency: domain.CurrencyRUB, Status: domain.PaymentIntentCreated, Provider: svc.provider.Code(), IdempotencyKey: "sub:queued-only", Metadata: raw}
	if err := repo.CreateIntent(ctx, pending); err != nil {
		t.Fatal(err)
	}
	state.PendingIntentID = pending.ID
	state.PaidUntil = time.Now().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	state, err = svc.CancelSubscription(ctx, owner)
	if err != nil || state.PendingIntentID != uuid.Nil {
		t.Fatalf("queued renewal stranded: %#v %v", state, err)
	}
	pending, _ = repo.GetIntentByID(ctx, pending.ID)
	if pending.Status != domain.PaymentIntentCanceled {
		t.Fatal("never submitted renewal remains payable")
	}
	if _, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "sub:manual-after-cancel"}); err != nil {
		t.Fatalf("new manual checkout blocked: %v", err)
	}
}

type orderedSubscriptionStore struct {
	domain.SubscriptionRepository
	owners []uuid.UUID
}

func (s orderedSubscriptionStore) ListDueSubscriptions(context.Context, time.Time, int) ([]uuid.UUID, error) {
	return s.owners, nil
}
func TestSubscriptionBrokenOwnerDoesNotStarveHealthyRenewal(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:healthy-owner"})
	if err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 54900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	state.PaidUntil = time.Now().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	badOwner := uuid.New()
	bad := *state
	bad.AccountID = badOwner
	bad.Revision = 0
	bad.ActiveIntentID = uuid.New()
	if err := credits.SaveSubscription(ctx, &bad); err != nil {
		t.Fatal(err)
	}
	svc.cfg.SubscriptionStore = orderedSubscriptionStore{SubscriptionRepository: credits, owners: []uuid.UUID{badOwner, owner}}
	processed, err := svc.ProcessSubscriptionRenewals(ctx, 10)
	if err == nil || processed != 1 {
		t.Fatalf("healthy renewal starved: count=%d err=%v", processed, err)
	}
	badState, _ := credits.GetSubscription(ctx, badOwner)
	if !badState.NextAttemptAt.After(time.Now()) {
		t.Fatal("failed owner occupies first page without backoff")
	}
}
