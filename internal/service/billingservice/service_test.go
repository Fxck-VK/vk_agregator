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

func TestEstimate(t *testing.T) {
	svc := billingservice.New(memory.NewBillingRepo())

	cases := map[domain.OperationType]int64{
		domain.OperationTextGenerate:      1,
		domain.OperationImageGenerate:     10,
		domain.OperationImageEdit:         10,
		domain.OperationVideoGenerate:     50,
		domain.OperationVideoImageToVideo: 50,
	}
	for op, want := range cases {
		got, err := svc.Estimate(op)
		if err != nil {
			t.Fatalf("Estimate(%s) error: %v", op, err)
		}
		if got != want {
			t.Errorf("Estimate(%s) = %d, want %d", op, got, want)
		}
	}

	if _, err := svc.Estimate(domain.OperationAudioTTS); !errors.Is(err, billingservice.ErrUnknownOperation) {
		t.Fatalf("expected ErrUnknownOperation, got %v", err)
	}
}

func TestEnsureAccountStartingBalance(t *testing.T) {
	svc := billingservice.New(memory.NewBillingRepo())
	ctx := context.Background()
	userID := uuid.New()

	acc, err := svc.EnsureAccount(ctx, userID)
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	if acc.BalanceCached != billingservice.DefaultStartingBalance {
		t.Fatalf("balance = %d, want %d", acc.BalanceCached, billingservice.DefaultStartingBalance)
	}

	// Idempotent: second call returns the same account.
	again, err := svc.EnsureAccount(ctx, userID)
	if err != nil {
		t.Fatalf("ensure account again: %v", err)
	}
	if again.ID != acc.ID {
		t.Fatal("expected the same account on second EnsureAccount")
	}
}

func TestBalanceForEstimateDoesNotCreateAccount(t *testing.T) {
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ctx := context.Background()
	userID := uuid.New()

	balance, err := svc.BalanceForEstimate(ctx, userID)
	if err != nil {
		t.Fatalf("balance for estimate: %v", err)
	}
	if balance != billingservice.DefaultStartingBalance {
		t.Fatalf("balance = %d, want %d", balance, billingservice.DefaultStartingBalance)
	}
	if _, err := repo.GetAccountByUser(ctx, userID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("estimate must not create account or ledger, got %v", err)
	}
}

func TestReserveCaptureRefundFlow(t *testing.T) {
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()

	if _, err := svc.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	res, err := svc.Reserve(ctx, userID, jobID, 50)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Capturing reduces the cached balance from 1000 to 950.
	if err := svc.Capture(ctx, res.ID, 50); err != nil {
		t.Fatalf("capture: %v", err)
	}
	acc, _ := svc.EnsureAccount(ctx, userID)
	if acc.BalanceCached != 950 {
		t.Fatalf("balance after capture = %d, want 950", acc.BalanceCached)
	}

	// Refunding the job returns the credits.
	if err := svc.Refund(ctx, userID, jobID, 50); err != nil {
		t.Fatalf("refund: %v", err)
	}
	acc, _ = svc.EnsureAccount(ctx, userID)
	if acc.BalanceCached != 1000 {
		t.Fatalf("balance after refund = %d, want 1000", acc.BalanceCached)
	}
}

func TestReserveRelease(t *testing.T) {
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()
	if _, err := svc.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	res, err := svc.Reserve(ctx, userID, jobID, 600)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	// With 600 reserved, available is 400, so another 500 reservation fails.
	if _, err := svc.Reserve(ctx, userID, uuid.New(), 500); !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatalf("expected ErrInsufficientCredits, got %v", err)
	}
	// Releasing the hold restores availability.
	if err := svc.Release(ctx, res.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := svc.Reserve(ctx, userID, uuid.New(), 500); err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
}
