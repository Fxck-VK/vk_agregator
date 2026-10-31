package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestSubscriptionCancelPreservesSubmittedButUnboundRenewal(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	first, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:ambiguous-cancel"})
	if err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: first.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 54900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, first.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	purchase := subscriptionPurchase{Plan: state.NextPlan, Next: state.NextPlan, AutoRenew: true, Renewal: true, PreviousIntentID: state.ActiveIntentID}
	raw, _ := json.Marshal(map[string]any{"subscription": purchase, "subscription_submit_started_at": time.Now().UTC().Format(time.RFC3339Nano)})
	pending := &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Amount: 54900, Credits: 1250, CreditDenominationVersion: 2, Currency: domain.CurrencyRUB, Status: domain.PaymentIntentCreated, Provider: svc.provider.Code(), IdempotencyKey: "sub:ambiguous-queued", Metadata: raw}
	if err := repo.CreateIntent(ctx, pending); err != nil {
		t.Fatal(err)
	}
	state.PendingIntentID = pending.ID
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	result, err := svc.CancelSubscription(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if result.PendingIntentID != pending.ID || result.AutoRenew {
		t.Fatal("cancellation lost possibly charged intent or kept renewal")
	}
	stored, _ := repo.GetIntentByID(ctx, pending.ID)
	if stored.Status != domain.PaymentIntentCreated {
		t.Fatal("possibly paid intent marked canceled")
	}
}

func TestLegacyTopupRefundRejectsMixedExpiringFundsBeforeDebit(t *testing.T) {
	ctx := context.Background()
	credits := memory.NewBillingRepo()
	owner := uuid.New()
	account := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits}
	if err := credits.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	if err := credits.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, OwnerAccountID: owner, Type: domain.LedgerTopup, Amount: 400, Status: domain.LedgerStatusCommitted, IdempotencyKey: "sub:active-mixed", LotKind: domain.CreditLotSubscription, LotExpiresAt: &until}); err != nil {
		t.Fatal(err)
	}
	intent := &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Amount: 50000, Credits: 1000, CreditDenominationVersion: 2, Currency: domain.CurrencyRUB, Provider: domain.PaymentProviderYooKassa, ProviderPaymentID: "purchased-refund", Metadata: json.RawMessage(`{}`)}
	if err := credits.AppendEntry(ctx, &domain.LedgerEntry{AccountID: account.ID, OwnerAccountID: owner, Type: domain.LedgerTopup, Amount: 1000, Status: domain.LedgerStatusCommitted, IdempotencyKey: topUpLedgerKey(intent.Provider, intent.ProviderPaymentID), LotKind: domain.CreditLotPurchased}); err != nil {
		t.Fatal(err)
	}
	if err := ensureTopupCreditsRefundable(ctx, credits, account.ID, intent); !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("mixed-expiry refund allowed to erase attribution: %v", err)
	}
	account, _ = credits.GetAccount(ctx, account.ID)
	if account.BalanceCached != 1400 {
		t.Fatal("rejected refund touched funds")
	}
}

func TestSubscriptionCancellationBetweenReceiptAndFirstSubmitRetiresUnsentRenewal(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	initial, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: "sub:cancel-before-first-submit"})
	if err != nil {
		t.Fatal(err)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: initial.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 54900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodID: "saved-method", PaymentMethodSaved: true}
	if _, err := processor.applyProviderPayment(ctx, repo, credits, initial.Intent, verified); err != nil {
		t.Fatal(err)
	}
	state, _ := svc.Subscription(ctx, owner)
	state.PaidUntil = time.Now().UTC().Add(-time.Minute)
	if err := credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	base := svc.cfg.SubscriptionTx
	calls := 0
	var pendingID uuid.UUID
	svc.cfg.SubscriptionTx = TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		calls++
		if calls == 3 {
			current, _ := credits.GetSubscription(ctx, owner)
			pendingID = current.PendingIntentID
			// Interleave cancellation after the durable marker commits, before
			// this invocation enters its locked provider-submit transaction.
			wrapper := svc.cfg.SubscriptionTx
			svc.cfg.SubscriptionTx = base
			_, cancelErr := svc.CancelSubscription(ctx, owner)
			svc.cfg.SubscriptionTx = wrapper
			if cancelErr != nil {
				return cancelErr
			}
		}
		return base.RunPaymentTx(ctx, fn)
	})
	if err := svc.renewSubscription(ctx, owner, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	state, _ = svc.Subscription(ctx, owner)
	if state.PendingIntentID != uuid.Nil || state.AutoRenew || state.CancelledAt == nil {
		t.Fatalf("provably unsent first renewal stranded: %+v", state)
	}
	intent, err := repo.GetIntentByID(ctx, pendingID)
	if err != nil || intent.Status != domain.PaymentIntentCanceled || intent.ProviderPaymentID != "" {
		t.Fatalf("unsent renewal not retired: %+v %v", intent, err)
	}
	if len(svc.provider.(*subscriptionProvider).inputs) != 1 {
		t.Fatal("cancellation submitted another charge")
	}
}

func TestSubscriptionSubmissionReceiptsFenceSupersededInvocation(t *testing.T) {
	svc, _, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	intent := &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Amount: 54900, Credits: 1250, Currency: domain.CurrencyRUB, Status: domain.PaymentIntentCreated, Provider: svc.provider.Code(), IdempotencyKey: "sub:receipt-fence", Metadata: json.RawMessage(`{}`)}
	if err := repo.CreateIntent(ctx, intent); err != nil {
		t.Fatal(err)
	}
	if err := credits.SaveSubscription(ctx, &domain.AccountSubscription{AccountID: owner, PendingIntentID: intent.ID, AutoRenew: true}); err != nil {
		t.Fatal(err)
	}
	firstAt := time.Now().UTC()
	first, err := svc.markSubscriptionSubmit(ctx, owner, intent.ID, firstAt)
	if err != nil || !first.FirstAttempt {
		t.Fatalf("first receipt: %+v %v", first, err)
	}
	second, err := svc.markSubscriptionSubmit(ctx, owner, intent.ID, firstAt.Add(time.Minute))
	if err != nil || second.FirstAttempt || second.ID == first.ID {
		t.Fatalf("retry receipt: %+v %v", second, err)
	}
	stored, _ := repo.GetIntentByID(ctx, intent.ID)
	if ownsSubmissionReceipt(stored, first) || !ownsSubmissionReceipt(stored, second) {
		t.Fatal("superseded invocation still owns the provider-submit right")
	}
	started, marked, err := submissionStartedAt(stored)
	if err != nil || !marked || !started.Equal(firstAt) {
		t.Fatal("retry moved the original provider idempotency deadline")
	}
}
