package promotionservice

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestPromotionQuoteReserveCapAndReplay(t *testing.T) {
	ctx := context.Background()
	r := memory.NewPaymentRepo()
	owner := uuid.New()
	p := &domain.Promotion{Code: " first15 ", Percent: 15, Enabled: true, StartsAt: time.Now().Add(-time.Hour), MaxUses: 1, PlanCodes: []string{"start", "lite"}}
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	key := uuid.New()
	p, err := r.MutatePromotion(ctx, "operator_test", key, "same", p)
	if err != nil {
		t.Fatal(err)
	}
	replay, e := r.MutatePromotion(ctx, "operator_test", key, "same", p)
	if e != nil || replay.ID != p.ID {
		t.Fatal("replay", e)
	}
	if _, e = r.MutatePromotion(ctx, "operator_test", key, "changed", p); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("changed replay accepted", e)
	}
	quote, e := QuoteLocked(ctx, r, owner, "first15", time.Now())
	if e != nil || quote.Code != "FIRST15" || len(quote.Plans) != 2 {
		t.Fatal(quote, e)
	}
	for _, plan := range quote.Plans {
		if plan.PlanCode == "start" && (plan.Amount != 46665 || plan.Stars != 1250) {
			t.Fatal(plan)
		}
	}
	id := uuid.New()
	snap, e := ReserveLocked(ctx, r, owner, id, "first15", 1, "start", time.Now())
	if e != nil || snap.BaseAmount != 54900 || snap.Amount != 46665 {
		t.Fatal(snap, e)
	}
	if _, e = QuoteLocked(ctx, r, uuid.New(), "first15", time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("cap ignored", e)
	}
	if e = r.CreateIntent(ctx, &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, IdempotencyKey: "competing"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("competing payment accepted", e)
	}
	if e = r.FinishPromotionLocked(ctx, owner, id, false); e != nil {
		t.Fatal(e)
	}
	if _, e = QuoteLocked(ctx, r, uuid.New(), "first15", time.Now()); e != nil {
		t.Fatal("release failed", e)
	}
}

func TestPromotionHistoricalRefundAndPendingDisqualify(t *testing.T) {
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentSucceeded, domain.PaymentIntentRefunded, domain.PaymentIntentPartiallyRefunded, domain.PaymentIntentCreated, domain.PaymentIntentProviderPending} {
		r := memory.NewPaymentRepo()
		owner := uuid.New()
		ctx := context.Background()
		if err := r.CreateIntent(ctx, &domain.PaymentIntent{AccountID: owner, Status: status, IdempotencyKey: uuid.NewString()}); err != nil {
			t.Fatal(err)
		}
		ok, e := r.PromotionEligibleLocked(ctx, owner)
		if e != nil || ok {
			t.Fatalf("status %s eligible=%v err=%v", status, ok, e)
		}
	}
}
