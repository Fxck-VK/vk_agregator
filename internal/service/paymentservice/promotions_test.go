package paymentservice

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/promotionservice"
)

func TestPromotionSubscriptionFrozenAmountFullStarsAndReplay(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	manager := promotionservice.New(svc.cfg.SubscriptionTx, repo)
	p, e := manager.Mutate(ctx, "operator_test", uuid.New(), &domain.Promotion{Code: "FIRST15", Percent: 15, Enabled: true, StartsAt: time.Now().Add(-time.Hour), PlanCodes: []string{"start"}})
	if e != nil {
		t.Fatal(e)
	}
	q, e := svc.QuotePromotion(ctx, owner, " first15 ")
	if e != nil || q.Plans[0].Amount != 46665 {
		t.Fatal(q, e)
	}
	in := CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "promo:one", PromoCode: "first15", PromoRevision: 1}
	result, e := svc.CreateSubscription(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	if result.Intent.Amount != 46665 || result.Intent.Credits != 1250 {
		t.Fatal(result.Intent)
	}
	repo.PutProduct(&domain.PaymentProduct{Code: "stars", Amount: 40000, Currency: domain.CurrencyRUB, Credits: 800, CreditDenominationVersion: 2, IsActive: true})
	if _, e = svc.CreateAccountIntent(ctx, CreateAccountIntentInput{AccountID: owner, ProductCode: "stars", ReceiptEmail: "test@example.test", IdempotencyKey: uuid.NewString()}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("pending promo web topup", e)
	}
	if _, e = svc.CreateIntent(ctx, CreateIntentInput{UserID: uuid.New(), AccountID: owner, ProductCode: "stars", ReceiptEmail: "test@example.test", IdempotencyKey: uuid.NewString(), ForceNew: true}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("pending promo legacy topup", e)
	}
	purchase, e := subscriptionPurchaseFrom(result.Intent)
	if e != nil || purchase.Plan.Amount != 54900 || purchase.Next.Amount != 54900 || purchase.Promotion == nil {
		t.Fatal(purchase, e)
	}
	in.PromoCode = "FIRST15"
	same, e := svc.CreateSubscription(ctx, in)
	if e != nil || same.Intent.ID != result.Intent.ID {
		t.Fatal("replay", e)
	}
	in.PromoRevision = 2
	if _, e = svc.CreateSubscription(ctx, in); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("changed replay", e)
	}
	p.Enabled = false
	p.Percent = 30
	p.ReservedCount = 0
	p.RedeemedCount = 0
	p.CreatedAt = time.Time{}
	p.UpdatedAt = time.Time{}
	if _, e = manager.Mutate(ctx, "operator_test", uuid.New(), p); e != nil {
		t.Fatal(e)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: result.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 46665, Currency: domain.CurrencyRUB, Paid: true, Captured: true}
	for i := 0; i < 2; i++ {
		current, _ := repo.GetIntentByID(ctx, result.Intent.ID)
		if _, e = processor.applyProviderPayment(ctx, repo, credits, current, verified); e != nil {
			t.Fatal(e)
		}
	}
	list, e := repo.ListPromotions(ctx, 10, 0)
	if e != nil || list[0].RedeemedCount != 1 || list[0].ReservedCount != 0 {
		t.Fatal(list, e)
	}
	state, e := svc.Subscription(ctx, owner)
	if e != nil || state.Plan.Stars != 1250 || state.NextPlan.Amount != 54900 {
		t.Fatal(state, e)
	}
	if _, e = svc.QuotePromotion(ctx, owner, "first15"); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("paid eligibility", e)
	}
}

func TestPromotionVerifiedUnpaidReleaseAndUnknownRetains(t *testing.T) {
	svc, processor, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	manager := promotionservice.New(svc.cfg.SubscriptionTx, repo)
	if _, e := manager.Mutate(ctx, "operator_test", uuid.New(), &domain.Promotion{Code: "ONE", Percent: 10, Enabled: true, StartsAt: time.Now().Add(-time.Hour), PlanCodes: []string{"start"}, MaxUses: 1}); e != nil {
		t.Fatal(e)
	}
	result, e := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: "promo:cancel", PromoCode: "ONE", PromoRevision: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.QuotePromotion(ctx, uuid.New(), "ONE"); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("pending cap", e)
	}
	verified := domain.ProviderPayment{ProviderPaymentID: result.Intent.ProviderPaymentID, Status: domain.PaymentIntentCanceled, Amount: result.Intent.Amount, Currency: domain.CurrencyRUB}
	invalid := verified
	invalid.Paid = true
	if _, e = processor.applyProviderPayment(ctx, repo, credits, result.Intent, invalid); !errors.Is(e, ErrWebhookMismatch) {
		t.Fatal("paid cancellation accepted", e)
	}
	current, e := repo.GetIntentByID(ctx, result.Intent.ID)
	if e != nil || current.Status != domain.PaymentIntentWaitingForUser {
		t.Fatal("unverified terminal mutated payment", e)
	}
	if _, e = processor.applyProviderPayment(ctx, repo, credits, result.Intent, verified); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.QuotePromotion(ctx, owner, "ONE"); e != nil {
		t.Fatal("not released", e)
	}
}

func TestPromotionRefundRetainsUseAndRenewalHasRegularPrice(t *testing.T) {
	for _, action := range []string{"refund", "renewal"} {
		t.Run(action, func(t *testing.T) {
			svc, processor, credits, repo, owner := subscriptionFixture(t)
			ctx := context.Background()
			manager := promotionservice.New(svc.cfg.SubscriptionTx, repo)
			if _, err := manager.Mutate(ctx, "operator_test", uuid.New(), &domain.Promotion{Code: "ONCE", Percent: 15, Enabled: true, StartsAt: time.Now().Add(-time.Hour), PlanCodes: []string{"lite", "start"}}); err != nil {
				t.Fatal(err)
			}
			result, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "lite", AutoRenew: true, ReceiptEmail: "test@example.test", IdempotencyKey: uuid.NewString(), PromoCode: "ONCE", PromoRevision: 1})
			if err != nil {
				t.Fatal(err)
			}
			verified := domain.ProviderPayment{ProviderPaymentID: result.Intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Amount: 16915, Currency: domain.CurrencyRUB, Paid: true, Captured: true, PaymentMethodSaved: true, PaymentMethodID: "saved-test-method"}
			if _, err = processor.applyProviderPayment(ctx, repo, credits, result.Intent, verified); err != nil {
				t.Fatal(err)
			}
			if action == "refund" {
				svc.provider.(*subscriptionProvider).refund = domain.VerifiedPaymentRefund{ID: "promo-refund", PaymentID: result.Intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: 16915, Currency: domain.CurrencyRUB}
				event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "refund.succeeded", ProviderPaymentID: result.Intent.ProviderPaymentID, ProviderRefundID: "promo-refund", DedupKey: "promo-refund"}
				if _, err = repo.CreateEvent(ctx, event); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if err = processor.ProcessEvent(ctx, event); err != nil {
						t.Fatal(err)
					}
				}
				if _, err = svc.QuotePromotion(ctx, owner, "ONCE"); !errors.Is(err, domain.ErrConflict) {
					t.Fatal("refund reset eligibility", err)
				}
				state, _ := svc.Subscription(ctx, owner)
				if !state.TrialUsed || state.AutoRenew || state.PaidUntil.After(time.Now()) {
					t.Fatal("refund left access")
				}
			} else {
				state, _ := svc.Subscription(ctx, owner)
				state.PaidUntil = time.Now().Add(-time.Hour)
				if err = credits.SaveSubscription(ctx, state); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.ProcessSubscriptionRenewals(ctx, 10); err != nil {
					t.Fatal(err)
				}
				state, _ = svc.Subscription(ctx, owner)
				next, err := repo.GetIntentByID(ctx, state.PendingIntentID)
				if err != nil {
					t.Fatal(err)
				}
				purchase, err := subscriptionPurchaseFrom(next)
				if err != nil || next.Amount != 54900 || next.Credits != 1250 || purchase.Promotion != nil || !purchase.Renewal {
					t.Fatal("renewal changed price/stars", err)
				}
			}
			list, err := repo.ListPromotions(ctx, 10, 0)
			if err != nil || list[0].RedeemedCount != 1 || list[0].ReservedCount != 0 {
				t.Fatal("use reset", list, err)
			}
		})
	}
}

func TestPromotionMemoryHistoricalPaidGrantExcludesSignupBonus(t *testing.T) {
	svc, _, credits, repo, owner := subscriptionFixture(t)
	ctx := context.Background()
	manager := promotionservice.New(svc.cfg.SubscriptionTx, repo)
	if _, err := manager.Mutate(ctx, "operator_test", uuid.New(), &domain.Promotion{Code: "HISTORY", Percent: 15, Enabled: true, StartsAt: time.Now().Add(-time.Hour), PlanCodes: []string{"start"}}); err != nil {
		t.Fatal(err)
	}
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 60}
	if err := credits.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.QuotePromotion(ctx, owner, "HISTORY"); err != nil {
		t.Fatal("signup bonus disqualifies", err)
	}
	if err := credits.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, OwnerAccountID: owner, Type: domain.LedgerTopup, Status: domain.LedgerStatusCommitted, Amount: 50, IdempotencyKey: "topup:mock:historical"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.QuotePromotion(ctx, owner, "HISTORY"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("historical purchase ignored", err)
	}
	if _, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: owner, PlanCode: "start", ReceiptEmail: "test@example.test", IdempotencyKey: uuid.NewString(), PromoCode: "HISTORY", PromoRevision: 1}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("historical checkout allowed", err)
	}
}
