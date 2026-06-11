package referralservice_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

func TestEnsureCodeIsStablePerUser(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewReferralRepo()
	userID := uuid.New()
	codes := []string{"ABC23456", "DEF23456"}
	idx := 0
	svc := referralservice.New(repo, nil, referralservice.Config{},
		referralservice.WithCodeGenerator(func(int) (string, error) {
			code := codes[idx]
			idx++
			return code, nil
		}),
	)

	first, err := svc.EnsureCode(ctx, userID)
	if err != nil {
		t.Fatalf("ensure first code: %v", err)
	}
	second, err := svc.EnsureCode(ctx, userID)
	if err != nil {
		t.Fatalf("ensure second code: %v", err)
	}
	if first.Code != "ABC23456" || second.Code != first.Code {
		t.Fatalf("code must be stable, first=%q second=%q", first.Code, second.Code)
	}
}

func TestApplyReferralRejectsSelfReferral(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewReferralRepo()
	userID := uuid.New()
	svc := referralservice.New(repo, nil, referralservice.Config{},
		referralservice.WithCodeGenerator(func(int) (string, error) { return "SEEF2345", nil }),
	)
	code, err := svc.EnsureCode(ctx, userID)
	if err != nil {
		t.Fatalf("ensure code: %v", err)
	}

	result, err := svc.Apply(ctx, referralservice.ApplyInput{
		Code:           code.Code,
		ReferredUserID: userID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !result.SelfReferral || result.Applied {
		t.Fatalf("expected self-referral rejection, got %+v", result)
	}
	count, _ := repo.CountByReferrer(ctx, userID)
	if count != 0 {
		t.Fatalf("self-referral must not be counted, got %d", count)
	}
}

func TestApplyReferralRegistersRelationWithoutLedgerReward(t *testing.T) {
	ctx := context.Background()
	referrals := memory.NewReferralRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	svc := referralservice.New(referrals, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
	},
		referralservice.WithClock(func() time.Time { return now }),
		referralservice.WithCodeGenerator(func(int) (string, error) { return "REF23456", nil }),
	)

	referrerID := uuid.New()
	referredID := uuid.New()
	code, err := svc.EnsureCode(ctx, referrerID)
	if err != nil {
		t.Fatalf("ensure code: %v", err)
	}

	first, err := svc.Apply(ctx, referralservice.ApplyInput{
		Code:           code.Code,
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if !first.Applied || first.Referral == nil ||
		first.Referral.Status != domain.ReferralStatusRegistered ||
		first.Referral.RewardStatus != domain.ReferralRewardPending {
		t.Fatalf("unexpected first apply result: %+v", first)
	}
	second, err := svc.Apply(ctx, referralservice.ApplyInput{
		Code:           code.Code,
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if !second.AlreadyApplied || second.Applied {
		t.Fatalf("expected idempotent already-applied result, got %+v", second)
	}

	count, err := referrals.CountByReferrer(ctx, referrerID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("referral count = %d, want 1", count)
	}
	_, stats, err := svc.StatsDetailed(ctx, referrerID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.RegisteredCount != 1 || stats.ActivatedCount != 0 || stats.RewardedCount != 0 || stats.Total() != 1 {
		t.Fatalf("unexpected referral stats after apply: %+v", stats)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, referrerID, domain.CurrencyCredits); err == nil {
		t.Fatal("referrer account must not be created by Apply before activation")
	}
}

func TestActivateReferralGrantsLedgerRewardIdempotently(t *testing.T) {
	ctx := context.Background()
	referrals := memory.NewReferralRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	svc := referralservice.New(referrals, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		ReferredSignupRewardCredits: 3,
		RewardOnActivation:          true,
	},
		referralservice.WithClock(func() time.Time { return now }),
		referralservice.WithCodeGenerator(func(int) (string, error) { return "ACT23456", nil }),
	)

	referrerID := uuid.New()
	referredID := uuid.New()
	code, err := svc.EnsureCode(ctx, referrerID)
	if err != nil {
		t.Fatalf("ensure code: %v", err)
	}
	if _, err := svc.Apply(ctx, referralservice.ApplyInput{
		Code:           code.Code,
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	first, err := svc.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("first activate: %v", err)
	}
	if !first.Activated || !first.Rewarded || first.Referral == nil ||
		first.Referral.Status != domain.ReferralStatusRewarded ||
		first.Referral.RewardStatus != domain.ReferralRewardApplied ||
		first.Referral.ActivatedAt == nil ||
		first.Referral.RewardedAt == nil {
		t.Fatalf("unexpected first activate result: %+v", first)
	}
	second, err := svc.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("second activate: %v", err)
	}
	if !second.AlreadyRewarded || second.Activated || second.Rewarded {
		t.Fatalf("expected idempotent already-rewarded result, got %+v", second)
	}

	acc, err := billingRepo.GetAccountByUser(ctx, referrerID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get referrer account: %v", err)
	}
	if acc.BalanceCached != 10 {
		t.Fatalf("referrer balance = %d, want 10", acc.BalanceCached)
	}
	referredAcc, err := billingRepo.GetAccountByUser(ctx, referredID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get referred account: %v", err)
	}
	if referredAcc.BalanceCached != 3 {
		t.Fatalf("referred balance = %d, want 3", referredAcc.BalanceCached)
	}
	entries, err := billingRepo.ListEntries(ctx, acc.ID, 10, 0)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	rewardEntries := 0
	for _, entry := range entries {
		if entry.Reason == "referral signup reward" {
			rewardEntries++
		}
	}
	if rewardEntries != 1 {
		t.Fatalf("reward entries = %d, want 1", rewardEntries)
	}
	_, stats, err := svc.StatsDetailed(ctx, referrerID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.RegisteredCount != 0 || stats.ActivatedCount != 0 || stats.RewardedCount != 1 || stats.Total() != 1 {
		t.Fatalf("unexpected referral stats after activate: %+v", stats)
	}
}

func TestRewardOnActivationFlagAllowsSafeRollout(t *testing.T) {
	ctx := context.Background()
	referrals := memory.NewReferralRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	referrerID := uuid.New()
	referredID := uuid.New()
	disabled := referralservice.New(referrals, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		RewardOnActivation:          false,
	},
		referralservice.WithClock(func() time.Time { return now }),
		referralservice.WithCodeGenerator(func(int) (string, error) { return "RLL23456", nil }),
	)
	code, err := disabled.EnsureCode(ctx, referrerID)
	if err != nil {
		t.Fatalf("ensure code: %v", err)
	}
	if _, err := disabled.Apply(ctx, referralservice.ApplyInput{
		Code:           code.Code,
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	activatedOnly, err := disabled.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("activate with disabled rewards: %v", err)
	}
	if !activatedOnly.Activated || activatedOnly.Rewarded || activatedOnly.AlreadyRewarded {
		t.Fatalf("expected activation without reward, got %+v", activatedOnly)
	}
	referral, err := referrals.GetReferralByReferredUserID(ctx, referredID)
	if err != nil {
		t.Fatalf("get referral: %v", err)
	}
	if referral.Status != domain.ReferralStatusActivated ||
		referral.RewardStatus != domain.ReferralRewardPending ||
		referral.ActivatedAt == nil ||
		referral.RewardedAt != nil {
		t.Fatalf("unexpected activation-only referral: %+v", referral)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, referrerID, domain.CurrencyCredits); err == nil {
		t.Fatal("disabled reward rollout must not create referrer account")
	}

	enabled := referralservice.New(referrals, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		RewardOnActivation:          true,
	}, referralservice.WithClock(func() time.Time { return now.Add(time.Minute) }))
	rewarded, err := enabled.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("activate with enabled rewards: %v", err)
	}
	if rewarded.Activated || !rewarded.Rewarded || rewarded.AlreadyRewarded {
		t.Fatalf("expected deferred one-time reward, got %+v", rewarded)
	}
	again, err := enabled.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: referredID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("repeat activate after reward: %v", err)
	}
	if !again.AlreadyRewarded || again.Rewarded {
		t.Fatalf("expected idempotent already-rewarded result, got %+v", again)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, referrerID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get referrer account: %v", err)
	}
	if acc.BalanceCached != 10 {
		t.Fatalf("referrer balance = %d, want 10", acc.BalanceCached)
	}
	entries, err := billingRepo.ListEntries(ctx, acc.ID, 10, 0)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	rewardEntries := 0
	for _, entry := range entries {
		if entry.Reason == "referral signup reward" {
			rewardEntries++
		}
	}
	if rewardEntries != 1 {
		t.Fatalf("reward entries = %d, want 1", rewardEntries)
	}
}
