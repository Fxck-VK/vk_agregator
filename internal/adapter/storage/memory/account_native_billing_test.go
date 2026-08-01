package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestAccountNativeCreditAccountsDoNotClaimNilLegacyUserKey(t *testing.T) {
	ctx := context.Background()
	repo := NewBillingRepo()
	ownerA, ownerB := uuid.New(), uuid.New()

	first := &domain.CreditAccount{UserID: uuid.Nil, OwnerAccountID: ownerA, Currency: domain.CurrencyCredits}
	second := &domain.CreditAccount{UserID: uuid.Nil, OwnerAccountID: ownerB, Currency: domain.CurrencyCredits}
	if err := repo.CreateAccount(ctx, first); err != nil {
		t.Fatalf("create first native account: %v", err)
	}
	if err := repo.CreateAccount(ctx, second); err != nil {
		t.Fatalf("create second native account: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("native accounts collided: %s", first.ID)
	}
	if got, err := repo.GetAccountByOwner(ctx, ownerA, domain.CurrencyCredits); err != nil || got.ID != first.ID {
		t.Fatalf("first canonical lookup = %#v, %v; want %s", got, err, first.ID)
	}
	if got, err := repo.GetAccountByOwner(ctx, ownerB, domain.CurrencyCredits); err != nil || got.ID != second.ID {
		t.Fatalf("second canonical lookup = %#v, %v; want %s", got, err, second.ID)
	}
	if _, err := repo.GetAccountByOwner(ctx, ownerB, domain.CurrencyStars); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner B stars lookup error = %v, want ErrNotFound", err)
	}
}

func TestAccountNativeReservationDerivesOwnerAndRejectsMismatch(t *testing.T) {
	ctx := context.Background()
	repo := NewBillingRepo()
	owner, foreignOwner := uuid.New(), uuid.New()
	account := &domain.CreditAccount{
		OwnerAccountID: owner,
		Currency:       domain.CurrencyCredits,
		BalanceCached:  100,
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}

	reservation := &domain.CreditReservation{
		AccountID:      account.ID,
		OwnerAccountID: foreignOwner,
		JobID:          uuid.New(),
		Amount:         40,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := repo.ReserveForOwner(ctx, foreignOwner, reservation); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("reserve with foreign owner error = %v, want ErrConflict", err)
	}
	if got, err := repo.GetReservationByJob(ctx, reservation.JobID); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("foreign-owner reservation = %#v, %v; want none", got, err)
	}
}

func TestAccountNativeReservationRejectsForgedPayloadOwner(t *testing.T) {
	ctx := context.Background()
	repo := NewBillingRepo()
	owner, foreignOwner := uuid.New(), uuid.New()
	account := &domain.CreditAccount{
		OwnerAccountID: owner,
		Currency:       domain.CurrencyCredits,
		BalanceCached:  100,
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}

	reservation := &domain.CreditReservation{
		AccountID:      account.ID,
		OwnerAccountID: foreignOwner,
		JobID:          uuid.New(),
		Amount:         40,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := repo.ReserveForOwner(ctx, owner, reservation); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("reserve with forged payload owner error = %v, want ErrConflict", err)
	}
	if got, err := repo.GetReservationByJob(ctx, reservation.JobID); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("forged-owner reservation = %#v, %v; want none", got, err)
	}

	valid := &domain.CreditReservation{
		AccountID:      account.ID,
		JobID:          uuid.New(),
		Amount:         40,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := repo.ReserveForOwner(ctx, owner, valid); err != nil {
		t.Fatalf("create valid reservation: %v", err)
	}
	forgedReplay := *valid
	forgedReplay.OwnerAccountID = foreignOwner
	if err := repo.ReserveForOwner(ctx, owner, &forgedReplay); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("forged exact replay error = %v, want ErrConflict", err)
	}
}

func TestAccountNativeCaptureRequiresExactReservedAmountWithoutMutation(t *testing.T) {
	ctx := context.Background()
	repo := NewBillingRepo()
	owner := uuid.New()
	account := &domain.CreditAccount{
		OwnerAccountID: owner,
		Currency:       domain.CurrencyCredits,
		BalanceCached:  100,
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	reservation := &domain.CreditReservation{
		AccountID:      account.ID,
		JobID:          uuid.New(),
		Amount:         40,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := repo.Reserve(ctx, reservation); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	for _, amount := range []int64{0, -1, 39, 41} {
		if err := repo.Capture(ctx, reservation.ID, amount, "cap:"+uuid.NewString()); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("capture amount %d error = %v, want ErrConflict", amount, err)
		}
		got, err := repo.GetReservation(ctx, reservation.ID)
		if err != nil || got.Status != domain.ReservationReserved {
			t.Fatalf("reservation after capture amount %d = %#v, %v; want reserved", amount, got, err)
		}
		current, err := repo.GetAccount(ctx, account.ID)
		if err != nil || current.BalanceCached != 100 {
			t.Fatalf("balance after capture amount %d = %#v, %v; want 100", amount, current, err)
		}
	}
}
