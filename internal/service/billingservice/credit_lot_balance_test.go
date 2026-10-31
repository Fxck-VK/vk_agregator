package billingservice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

func TestBalanceForEstimateUsesSpendableOwnerLots(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	owner := uuid.New()
	legacy := uuid.New()
	now := time.Now()
	acc := &domain.CreditAccount{UserID: legacy, OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := repo.CreateAccount(ctx, acc); err != nil {
		t.Fatal(err)
	}
	expiry := now.Add(time.Hour)
	if err := repo.AppendEntry(ctx, &domain.LedgerEntry{AccountID: acc.ID, Type: domain.LedgerTopup, Amount: 60, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	res := &domain.CreditReservation{AccountID: acc.ID, JobID: uuid.New(), Amount: 40, IdempotencyKey: uuid.NewString()}
	if err := repo.ReserveForOwner(ctx, owner, res); err != nil {
		t.Fatal(err)
	}
	svc := billingservice.New(repo, billingservice.WithClock(func() time.Time { return expiry.Add(time.Second) }))
	balance, err := svc.BalanceForEstimate(ctx, owner)
	if err != nil || balance != 100 {
		t.Fatalf("spendable after expiry/hold=%d/%v want100", balance, err)
	}
	if err := repo.AppendEntry(ctx, &domain.LedgerEntry{AccountID: acc.ID, Type: domain.LedgerAdjustment, Amount: -150, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	balance, err = svc.BalanceForEstimate(ctx, owner)
	if err != nil || balance != -30 {
		t.Fatalf("signed spendable=%d/%v want-30", balance, err)
	}
	entries, _ := repo.ListEntries(ctx, acc.ID, 100, 0)
	if len(entries) != 4 {
		t.Fatalf("estimate wrote ledger entries: %d", len(entries))
	}
	foreign, _ := svc.BalanceForEstimate(ctx, legacy)
	if foreign != billingservice.DefaultStartingBalance {
		t.Fatalf("legacy provenance crossed owner: %d", foreign)
	}
}

func TestBalanceForEstimateLegacyFallbackAndLotFailure(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	owner := uuid.New()
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 90}
	if err := repo.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	legacy := struct{ domain.BillingRepository }{repo}
	if got, err := billingservice.New(legacy).BalanceForEstimate(ctx, owner); err != nil || got != 90 {
		t.Fatalf("legacy fallback=%d/%v", got, err)
	}
	broken := &failingLotReader{BillingRepository: repo}
	if _, err := billingservice.New(broken).BalanceForEstimate(ctx, owner); !errors.Is(err, errLotRead) {
		t.Fatalf("lot failure fell back to cached: %v", err)
	}
}

var errLotRead = errors.New("lot read failed")

type failingLotReader struct{ domain.BillingRepository }

func (*failingLotReader) CreditLotBalancesForOwner(context.Context, uuid.UUID, time.Time) ([]domain.CreditLotBalance, error) {
	return nil, errLotRead
}

func TestPaymentGrantClassifiesOnlyVerifiedPaymentReason(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	owner := uuid.New()
	svc := billingservice.New(repo, billingservice.WithStartingBalance(0))
	if err := svc.GrantWithAccount(ctx, repo, owner, 80, uuid.NewString(), "payment top-up via yookassa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.GrantWithAccount(ctx, repo, owner, 10, uuid.NewString(), "referral reward"); err != nil {
		t.Fatal(err)
	}
	lots, err := repo.CreditLotBalancesForOwner(ctx, owner, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var purchased, legacy int64
	for _, lot := range lots {
		if lot.Kind == domain.CreditLotPurchased {
			purchased += lot.AvailableCredits
		}
		if lot.Kind == domain.CreditLotLegacy {
			legacy += lot.AvailableCredits
		}
	}
	if purchased != 80 || legacy != 10 {
		t.Fatalf("grant classification purchased=%d legacy=%d", purchased, legacy)
	}
}

func TestNewAccountStartingGrantIsOneBonusLot(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy_entrypoint", true: "canonical_owner"}[native], func(t *testing.T) {
			ctx := context.Background()
			repo := memory.NewBillingRepo()
			owner := uuid.New()
			svc := billingservice.New(repo)
			ensure := func() (*domain.CreditAccount, error) {
				if native {
					return svc.EnsureAccountForAccount(ctx, repo, owner, domain.CurrencyCredits)
				}
				return svc.EnsureAccount(ctx, owner)
			}
			account, err := ensure()
			if err != nil {
				t.Fatal(err)
			}
			again, err := ensure()
			if err != nil || again.ID != account.ID {
				t.Fatalf("account retry differs: %v", err)
			}
			lots, err := repo.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil || len(lots) != 1 || lots[0].Kind != domain.CreditLotBonus || lots[0].AvailableCredits != 60 || lots[0].ReservedCredits != 0 || lots[0].ExpiresAt != nil {
				t.Fatalf("opening bonus lots=%v err=%v", lots, err)
			}
			entries, err := repo.ListEntries(ctx, account.ID, 10, 0)
			if err != nil || len(entries) != 1 || entries[0].Amount != 60 || entries[0].LotKind != domain.CreditLotBonus {
				t.Fatalf("opening ledger bonus not singular: entries=%d err=%v", len(entries), err)
			}
		})
	}
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	owner := uuid.New()
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 60}
	if err := repo.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	lots, err := repo.CreditLotBalancesForOwner(ctx, owner, time.Now())
	if err != nil || len(lots) != 1 || lots[0].Kind != domain.CreditLotLegacy {
		t.Fatalf("direct legacy caller reclassified: %v/%v", lots, err)
	}
}
