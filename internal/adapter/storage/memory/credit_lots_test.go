package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestCreditLotsFEFOExpiryAndLateTerminal(t *testing.T) {
	for _, capture := range []bool{false, true} {
		t.Run(map[bool]string{false: "release", true: "capture"}[capture], func(t *testing.T) {
			ctx := context.Background()
			r := NewBillingRepo()
			owner := uuid.New()
			a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 20}
			if err := r.CreateAccount(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 80, IdempotencyKey: uuid.NewString(), LotKind: domain.CreditLotPurchased}); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour)
			e := &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 60, IdempotencyKey: uuid.NewString(), LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry}
			if err := r.AppendEntry(ctx, e); err != nil {
				t.Fatal(err)
			}
			res := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 40, IdempotencyKey: uuid.NewString()}
			if err := r.ReserveForOwner(ctx, owner, res); err != nil {
				t.Fatal(err)
			}
			if err := r.ReserveForOwner(ctx, owner, res); err != nil {
				t.Fatal(err)
			}
			b, err := r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for _, lot := range b {
				if lot.Kind == domain.CreditLotSubscription && (lot.AvailableCredits != 20 || lot.ReservedCredits != 40) {
					t.Fatalf("FEFO lot: %+v", lot)
				}
			}
			n, err := r.ExpireCreditLots(ctx, expiry.Add(time.Second), 10)
			if err != nil || n != 1 {
				t.Fatalf("expiry %d %v", n, err)
			}
			// Move the expiry clock back in the stored lot to exercise real late terminal paths.
			for id, lot := range r.lots {
				if lot.Kind == domain.CreditLotSubscription {
					past := time.Now().Add(-time.Second)
					lot.ExpiresAt = &past
					r.lots[id] = lot
				}
			}
			key := uuid.NewString()
			if capture {
				err = r.CaptureForOwner(ctx, owner, res.ID, 40, key)
			} else {
				err = r.ReleaseForOwner(ctx, owner, res.ID, key)
			}
			if err != nil {
				t.Fatal(err)
			}
			if capture {
				err = r.CaptureForOwner(ctx, owner, res.ID, 40, key)
			} else {
				err = r.ReleaseForOwner(ctx, owner, res.ID, key)
			}
			if err != nil {
				t.Fatal(err)
			}
			got, _ := r.GetAccount(ctx, a.ID)
			if got.BalanceCached != 100 {
				t.Fatalf("balance %d want purchased/legacy 100", got.BalanceCached)
			}
			assertLotProjection(t, r, a.ID)
		})
	}
}

func TestCreditLotsSignedMovementsOwnerAndConcurrency(t *testing.T) {
	ctx := context.Background()
	r := NewBillingRepo()
	owner := uuid.New()
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := r.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	for _, amount := range []int64{-120, 45, -5} {
		if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: amount, IdempotencyKey: uuid.NewString()}); err != nil {
			t.Fatal(err)
		}
		assertLotProjection(t, r, a.ID)
	}
	if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerRefund, Amount: 30, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	bad := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 1, IdempotencyKey: uuid.NewString()}
	if err := r.ReserveForOwner(ctx, uuid.New(), bad); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-owner %v", err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 5, IdempotencyKey: uuid.NewString()}
			err := r.Reserve(ctx, res)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			} else if !errors.Is(err, domain.ErrInsufficientCredits) {
				t.Errorf("reserve: %v", err)
			}
		}()
	}
	wg.Wait()
	if success != 10 {
		t.Fatalf("reserved %d want 10", success)
	}
	assertLotProjection(t, r, a.ID)
}

func assertLotProjection(t *testing.T, r *BillingRepo, account uuid.UUID) {
	t.Helper()
	var lots, ledger int64
	for _, l := range r.lots {
		if l.AccountID == account {
			lots += l.AvailableCredits + l.ReservedCredits
		}
	}
	for _, e := range r.ledger {
		if e.AccountID == account && e.Status == domain.LedgerStatusCommitted {
			ledger += e.Amount
		}
	}
	if lots != ledger || lots != r.accounts[account].BalanceCached {
		t.Fatalf("projection lots=%d ledger=%d cached=%d", lots, ledger, r.accounts[account].BalanceCached)
	}
}

func TestCreditLotsLateReleaseConflictDoesNotPartiallyMutate(t *testing.T) {
	ctx := context.Background()
	r := NewBillingRepo()
	a := &domain.CreditAccount{OwnerAccountID: uuid.New(), Currency: domain.CurrencyCredits}
	if err := r.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	grant := &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 20, IdempotencyKey: uuid.NewString(), LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry}
	if err := r.AppendEntry(ctx, grant); err != nil {
		t.Fatal(err)
	}
	res := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 20, IdempotencyKey: uuid.NewString()}
	if err := r.Reserve(ctx, res); err != nil {
		t.Fatal(err)
	}
	l := r.lots[grant.ID]
	past := time.Now().Add(-time.Hour)
	l.ExpiresAt = &past
	r.lots[l.ID] = l
	collision := "credit-lot-expiry:release:" + res.ID.String() + ":" + grant.ID.String()
	if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: 0, IdempotencyKey: collision}); err != nil {
		t.Fatal(err)
	}
	count := len(r.ledger)
	if err := r.Release(ctx, res.ID, uuid.NewString()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("release %v", err)
	}
	if len(r.ledger) != count || r.lots[grant.ID].ReservedCredits != 20 || r.reservations[res.ID].Status != domain.ReservationReserved {
		t.Fatal("release conflict partially mutated accounting")
	}
	assertLotProjection(t, r, a.ID)
}

type creditLotRevoker interface {
	RevokeCreditLotForGrant(context.Context, uuid.UUID, string, time.Time) error
}

func TestCreditLotRevocationPreservesOtherGrantsAndHolds(t *testing.T) {
	for _, capture := range []bool{false, true} {
		t.Run(map[bool]string{false: "release", true: "capture"}[capture], func(t *testing.T) {
			ctx := context.Background()
			r := NewBillingRepo()
			revoker, ok := any(r).(creditLotRevoker)
			if !ok {
				t.Fatal("missing exact grant-lot revocation")
			}
			owner := uuid.New()
			a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
			if err := r.CreateAccount(ctx, a); err != nil {
				t.Fatal(err)
			}
			purchasedKey := uuid.NewString()
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 80, LotKind: domain.CreditLotPurchased, IdempotencyKey: purchasedKey}); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour)
			grantKey := uuid.NewString()
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 60, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry, IdempotencyKey: grantKey}); err != nil {
				t.Fatal(err)
			}
			spent := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 20, IdempotencyKey: uuid.NewString()}
			if err := r.Reserve(ctx, spent); err != nil {
				t.Fatal(err)
			}
			if err := r.Capture(ctx, spent.ID, 20, uuid.NewString()); err != nil {
				t.Fatal(err)
			}
			held := &domain.CreditReservation{AccountID: a.ID, JobID: uuid.New(), Amount: 30, IdempotencyKey: uuid.NewString()}
			if err := r.Reserve(ctx, held); err != nil {
				t.Fatal(err)
			}
			now := time.Now().Add(-time.Second)
			if err := revoker.RevokeCreditLotForGrant(ctx, uuid.New(), grantKey, now); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("cross-owner revocation %v", err)
			}
			if err := revoker.RevokeCreditLotForGrant(ctx, owner, purchasedKey, now); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("purchased revocation %v", err)
			}
			if err := revoker.RevokeCreditLotForGrant(ctx, owner, grantKey, now); err != nil {
				t.Fatal(err)
			}
			entries := len(r.ledger)
			if err := revoker.RevokeCreditLotForGrant(ctx, owner, grantKey, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if len(r.ledger) != entries {
				t.Fatal("revocation replay appended ledger")
			}
			got, _ := r.GetAccount(ctx, a.ID)
			if got.BalanceCached != 210 {
				t.Fatalf("revoke balance %d want210", got.BalanceCached)
			}
			assertLotProjection(t, r, a.ID)
			var err error
			if capture {
				err = r.Capture(ctx, held.ID, 30, uuid.NewString())
			} else {
				err = r.Release(ctx, held.ID, uuid.NewString())
			}
			if err != nil {
				t.Fatal(err)
			}
			got, _ = r.GetAccount(ctx, a.ID)
			if got.BalanceCached != 180 {
				t.Fatalf("terminal balance %d want180", got.BalanceCached)
			}
			assertLotProjection(t, r, a.ID)
		})
	}
}

func TestCreditLotGenericDebitExpiresUnsweptLotsBeforeConsumption(t *testing.T) {
	for _, debit := range []int64{100, 1000} {
		t.Run(fmt.Sprint(debit), func(t *testing.T) {
			ctx := context.Background()
			r := NewBillingRepo()
			owner := uuid.New()
			a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits}
			if err := r.CreateAccount(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 1000, LotKind: domain.CreditLotPurchased, IdempotencyKey: uuid.NewString()}); err != nil {
				t.Fatal(err)
			}
			expired := time.Now().Add(-time.Hour)
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 400, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expired, IdempotencyKey: uuid.NewString()}); err != nil {
				t.Fatal(err)
			}
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: -debit, IdempotencyKey: uuid.NewString(), Reason: "payment refund debit"}); err != nil {
				t.Fatal(err)
			}
			lots, err := r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			var spendable int64
			for _, lot := range lots {
				spendable += lot.AvailableCredits
			}
			account, _ := r.GetAccount(ctx, a.ID)
			if spendable != 1000-debit || account.BalanceCached != 1000-debit {
				t.Fatalf("debit consumed expired credits: spendable=%d cached=%d want=%d", spendable, account.BalanceCached, 1000-debit)
			}
			assertLotProjection(t, r, a.ID)
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: debit, IdempotencyKey: uuid.NewString(), Reason: "payment refund provider failure compensation"}); err != nil {
				t.Fatal(err)
			}
			lots, err = r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			spendable = 0
			for _, lot := range lots {
				spendable += lot.AvailableCredits
				if lot.Kind == domain.CreditLotSubscription && lot.AvailableCredits != 0 {
					t.Fatal("compensation resurrected expired subscription")
				}
			}
			if spendable != 1000 {
				t.Fatalf("compensation restored expired credits: %d want1000", spendable)
			}
			assertLotProjection(t, r, a.ID)
			if n, err := r.ExpireCreditLots(ctx, time.Now(), 10); err != nil || n != 0 {
				t.Fatalf("generic debit did not persist exact expiry: %d/%v", n, err)
			}
		})
	}
}
