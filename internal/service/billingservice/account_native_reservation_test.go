package billingservice_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

func TestAccountNativeReservationOwnerReplayAndLedgerOwnership(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo, billingservice.WithStartingBalance(100))
	owner, foreignOwner, jobID := uuid.New(), uuid.New(), uuid.New()

	reservation, err := svc.ReserveForAccount(ctx, owner, jobID, 40)
	if err != nil {
		t.Fatalf("reserve for owner: %v", err)
	}
	if reservation.OwnerAccountID != owner {
		t.Fatalf("stored reservation owner = %s, want account owner %s", reservation.OwnerAccountID, owner)
	}
	if err := repo.ReserveForOwner(ctx, foreignOwner, &domain.CreditReservation{
		AccountID:      reservation.AccountID,
		OwnerAccountID: foreignOwner,
		JobID:          uuid.New(),
		Amount:         1,
		IdempotencyKey: "resv:" + uuid.NewString(),
	}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign owner reserve error = %v, want ErrConflict", err)
	}

	replayed, err := svc.ReserveForAccount(ctx, owner, jobID, 40)
	if err != nil || replayed.ID != reservation.ID {
		t.Fatalf("exact replay = %#v, %v; want reservation %s", replayed, err, reservation.ID)
	}
	if _, err := svc.ReserveForAccount(ctx, owner, jobID, 41); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed amount replay error = %v, want ErrConflict", err)
	}
	if _, err := svc.ReserveForAccount(ctx, foreignOwner, jobID, 40); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign owner replay error = %v, want ErrConflict", err)
	}
	if _, err := svc.ReserveForAccount(ctx, owner, uuid.New(), 60); err != nil {
		t.Fatalf("consume remaining availability: %v", err)
	}
	if replayed, err = svc.ReserveForAccount(ctx, owner, jobID, 40); err != nil || replayed.ID != reservation.ID {
		t.Fatalf("replay after availability changed = %#v, %v", replayed, err)
	}

	entries, err := repo.ListEntries(ctx, reservation.AccountID, 20, 0)
	if err != nil {
		t.Fatalf("list ledger: %v", err)
	}
	for _, entry := range entries {
		if entry.OwnerAccountID != owner || entry.AccountID != reservation.AccountID {
			t.Fatalf("ledger ownership = %#v, want owner=%s account=%s", entry, owner, reservation.AccountID)
		}
	}
}

func TestAccountNativeTerminalOperationsAreExactAndOwnerScoped(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo, billingservice.WithStartingBalance(100))
	owner, foreignOwner := uuid.New(), uuid.New()
	reservation, err := svc.ReserveForAccount(ctx, owner, uuid.New(), 40)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	for _, amount := range []int64{0, -1, 39, 41} {
		if err := svc.CaptureForAccount(ctx, owner, reservation.ID, amount); err == nil {
			t.Fatalf("capture amount %d unexpectedly succeeded", amount)
		}
		stored, _ := repo.GetReservation(ctx, reservation.ID)
		if stored.Status != domain.ReservationReserved {
			t.Fatalf("capture amount %d changed status to %q", amount, stored.Status)
		}
	}
	if err := svc.CaptureForAccount(ctx, foreignOwner, reservation.ID, 40); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign capture error = %v, want ErrConflict", err)
	}
	if err := svc.CaptureForAccount(ctx, owner, reservation.ID, 40); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if err := svc.CaptureForAccount(ctx, owner, reservation.ID, 40); err != nil {
		t.Fatalf("exact capture replay: %v", err)
	}
	if err := svc.ReleaseForAccount(ctx, owner, reservation.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("release after capture error = %v, want ErrConflict", err)
	}

	released, err := svc.ReserveForAccount(ctx, owner, uuid.New(), 10)
	if err != nil {
		t.Fatalf("reserve to release: %v", err)
	}
	if err := svc.ReleaseForAccount(ctx, owner, released.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := svc.ReleaseForAccount(ctx, owner, released.ID); err != nil {
		t.Fatalf("exact release replay: %v", err)
	}
	if err := svc.CaptureForAccount(ctx, owner, released.ID, 10); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("capture after release error = %v, want ErrConflict", err)
	}

	entries, err := repo.ListEntries(ctx, reservation.AccountID, 20, 0)
	if err != nil {
		t.Fatalf("list ledger: %v", err)
	}
	var captures, releases int
	for _, entry := range entries {
		switch entry.Type {
		case domain.LedgerCapture:
			captures++
		case domain.LedgerRelease:
			releases++
		}
	}
	if captures != 1 || releases != 1 {
		t.Fatalf("terminal ledger counts capture/release = %d/%d, want 1/1", captures, releases)
	}
}

func TestAccountNativeLedgerKeyCollisionDoesNotCaptureReservation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo, billingservice.WithStartingBalance(100))
	owner := uuid.New()
	reservation, err := svc.ReserveForAccount(ctx, owner, uuid.New(), 40)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := repo.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      reservation.AccountID,
		OwnerAccountID: owner,
		Type:           domain.LedgerAdjustment,
		Amount:         0,
		IdempotencyKey: "cap:" + reservation.ID.String(),
	}); err != nil {
		t.Fatalf("seed colliding ledger key: %v", err)
	}
	if err := svc.CaptureForAccount(ctx, owner, reservation.ID, 40); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("capture with colliding key error = %v, want ErrConflict", err)
	}
	stored, err := repo.GetReservation(ctx, reservation.ID)
	if err != nil || stored.Status != domain.ReservationReserved {
		t.Fatalf("reservation after key collision = %#v, %v; want reserved", stored, err)
	}
	account, err := repo.GetAccount(ctx, reservation.AccountID)
	if err != nil || account.BalanceCached != 100 {
		t.Fatalf("balance after key collision = %#v, %v; want 100", account, err)
	}
}
