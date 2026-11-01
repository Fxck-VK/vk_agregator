package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/logging"
)

type subscriptionRenewalWorker interface {
	ProcessSubscriptionRenewals(context.Context, int) (int, error)
}
type subscriptionExpiryWorker interface {
	ExpireCreditLots(context.Context, time.Time, int) (int64, error)
}

func runSubscriptionMaintenanceOnce(ctx context.Context, renewals subscriptionRenewalWorker, expiry subscriptionExpiryWorker, rewards ...domain.WebReferralRewardProcessor) error {
	var expireErr, renewErr, rewardErr error
	for _, reward := range rewards {
		if reward != nil {
			_, err := reward.ProcessWebReferralRewards(ctx, time.Now().UTC(), 100)
			rewardErr = errors.Join(rewardErr, err)
		}
	}
	if expiry != nil {
		_, expireErr = expiry.ExpireCreditLots(ctx, time.Now().UTC(), 100)
	}
	if renewals != nil {
		_, renewErr = renewals.ProcessSubscriptionRenewals(ctx, 20)
	}
	return errors.Join(expireErr, renewErr, rewardErr)
}
func runSubscriptionMaintenanceLoop(ctx context.Context, renewals subscriptionRenewalWorker, expiry subscriptionExpiryWorker, logger *slog.Logger, rewards ...domain.WebReferralRewardProcessor) {
	run := func() {
		if ctx.Err() == nil {
			if err := runSubscriptionMaintenanceOnce(ctx, renewals, expiry, rewards...); err != nil {
				logger.Error("subscription maintenance failed", logging.ErrorAttr(err))
			}
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
