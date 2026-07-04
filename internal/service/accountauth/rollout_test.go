package accountauth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestAccountIdentityRolloutPreservesLegacyVKBusinessState(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserRepo()
	identities := memory.NewAccountIdentityRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo)
	resolver := identityresolver.New(users, identities, billing)
	auth := accountauth.New(resolver)
	jobs := memory.NewJobRepo()
	payments := memory.NewPaymentRepo()

	legacyUserID := uuid.New()
	accountID := uuid.New()
	oldUser := &domain.User{
		ID:        legacyUserID,
		AccountID: accountID,
		VKUserID:  9001001,
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
		Locale:    "ru",
		Timezone:  "Europe/Moscow",
	}
	if err := users.Create(ctx, oldUser); err != nil {
		t.Fatalf("create legacy user: %v", err)
	}
	if err := billing.Grant(ctx, accountID, 70, "rollout:old-topup", "old paid topup"); err != nil {
		t.Fatalf("grant old balance: %v", err)
	}
	oldJob := &domain.Job{
		UserID:         legacyUserID,
		AccountID:      accountID,
		Source:         "vk_bot",
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusSucceeded,
		IdempotencyKey: "rollout:old-job",
		CorrelationID:  "rollout-old-job",
	}
	if err := jobs.Create(ctx, oldJob); err != nil {
		t.Fatalf("create old job: %v", err)
	}
	oldIntent := &domain.PaymentIntent{
		UserID:            legacyUserID,
		AccountID:         accountID,
		Status:            domain.PaymentIntentSucceeded,
		Amount:            1000,
		Currency:          domain.CurrencyRUB,
		Credits:           70,
		PriceVersion:      1,
		Provider:          domain.PaymentProviderMock,
		ProviderPaymentID: "mock-pay-rollout-old",
		IdempotencyKey:    "rollout:old-payment",
	}
	if err := payments.CreateIntent(ctx, oldIntent); err != nil {
		t.Fatalf("create old payment intent: %v", err)
	}

	resolvedOld, err := resolver.ResolveOrCreate(ctx, domain.IdentityProviderVK, "9001001")
	if err != nil {
		t.Fatalf("resolve old VK user: %v", err)
	}
	if resolvedOld.AccountID != accountID {
		t.Fatalf("old VK account id = %s, want %s", resolvedOld.AccountID, accountID)
	}
	assertBalance(t, ctx, billing, accountID, 100)
	assertOldJobsVisible(t, ctx, jobs, accountID, oldJob.ID)
	assertOldPaymentsVisible(t, ctx, payments, accountID, oldIntent.ID)
	assertLedgerOwnedByAccount(t, ctx, billingRepo, accountID)

	newVK, err := resolver.ResolveOrCreate(ctx, domain.IdentityProviderVK, "9002002")
	if err != nil {
		t.Fatalf("resolve new VK user: %v", err)
	}
	if newVK.AccountID == uuid.Nil || newVK.User == nil || newVK.Identity == nil {
		t.Fatalf("new VK user did not get implicit account: %+v", newVK)
	}
	if newVK.User.EffectiveAccountID() != newVK.AccountID {
		t.Fatalf("new VK effective account = %s, want %s", newVK.User.EffectiveAccountID(), newVK.AccountID)
	}

	linked, err := auth.LinkVerifiedIdentity(ctx, accountID, accountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "Owner@Example.COM",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link email to existing account: %v", err)
	}
	if linked.AccountID != accountID || linked.Provider != domain.IdentityProviderEmail || linked.NormalizedID != "owner@example.com" {
		t.Fatalf("linked email = %+v, want same account/email identity", linked)
	}
	linkedAgain, err := auth.LinkVerifiedIdentity(ctx, accountID, accountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link same email again: %v", err)
	}
	if linkedAgain.ID != linked.ID {
		t.Fatalf("email relink is not idempotent: first=%s second=%s", linked.ID, linkedAgain.ID)
	}
	resolvedLinkedEmail, err := auth.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve linked email: %v", err)
	}
	if resolvedLinkedEmail.AccountID != accountID {
		t.Fatalf("linked email resolved to account %s, want existing account %s", resolvedLinkedEmail.AccountID, accountID)
	}
	assertBalance(t, ctx, billing, accountID, 100)

	other, err := auth.ResolveVerifiedEmailPassword(ctx, "other@example.com")
	if err != nil {
		t.Fatalf("resolve other account: %v", err)
	}
	_, err = auth.LinkVerifiedIdentity(ctx, other.AccountID, other.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("conflicting email link error = %v, want conflict", err)
	}
	assertBalance(t, ctx, billing, accountID, 100)
	assertOldPaymentsVisible(t, ctx, payments, accountID, oldIntent.ID)
	assertOldJobsVisible(t, ctx, jobs, accountID, oldJob.ID)
}

func assertBalance(t *testing.T, ctx context.Context, billing *billingservice.Service, accountID uuid.UUID, want int64) {
	t.Helper()
	got, err := billing.BalanceForEstimate(ctx, accountID)
	if err != nil {
		t.Fatalf("read balance for %s: %v", accountID, err)
	}
	if got != want {
		t.Fatalf("balance for account %s = %d, want %d", accountID, got, want)
	}
}

func assertOldJobsVisible(t *testing.T, ctx context.Context, jobs *memory.JobRepo, accountID, wantJobID uuid.UUID) {
	t.Helper()
	got, err := jobs.List(ctx, domain.JobFilter{AccountID: &accountID}, 10, 0)
	if err != nil {
		t.Fatalf("list jobs by account: %v", err)
	}
	for _, job := range got {
		if job.ID == wantJobID && job.AccountID == accountID {
			return
		}
	}
	t.Fatalf("old job %s not visible by account %s: %+v", wantJobID, accountID, got)
}

func assertOldPaymentsVisible(t *testing.T, ctx context.Context, payments *memory.PaymentRepo, accountID, wantIntentID uuid.UUID) {
	t.Helper()
	got, err := payments.ListIntents(ctx, domain.PaymentIntentFilter{AccountID: &accountID}, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents by account: %v", err)
	}
	for _, intent := range got {
		if intent.ID == wantIntentID && intent.AccountID == accountID {
			return
		}
	}
	t.Fatalf("old payment %s not visible by account %s: %+v", wantIntentID, accountID, got)
}

func assertLedgerOwnedByAccount(t *testing.T, ctx context.Context, billingRepo *memory.BillingRepo, accountID uuid.UUID) {
	t.Helper()
	account, err := billingRepo.GetAccountByUser(ctx, accountID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get credit account by canonical owner: %v", err)
	}
	entries, err := billingRepo.ListEntries(ctx, account.ID, 20, 0)
	if err != nil {
		t.Fatalf("list ledger entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected ledger entries for account %s", account.ID)
	}
	for _, entry := range entries {
		if entry.OwnerAccountID != accountID {
			t.Fatalf("ledger entry owner = %s, want account_id %s: %+v", entry.OwnerAccountID, accountID, entry)
		}
	}
}
