// Command provider-balance-bot runs a read-only Telegram admin bot for provider
// balance monitoring. It does not call generation APIs or touch user billing.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	telegramdelivery "vk-ai-aggregator/internal/adapter/delivery/telegram"
	apimartbalance "vk-ai-aggregator/internal/adapter/providerbalance/apimart"
	deepinfrabalance "vk-ai-aggregator/internal/adapter/providerbalance/deepinfra"
	poyobalance "vk-ai-aggregator/internal/adapter/providerbalance/poyo"
	runwaybalance "vk-ai-aggregator/internal/adapter/providerbalance/runway"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/service/providerbalance"
)

func main() {
	logger := slog.New(logging.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("provider balance bot failed", logging.ErrorAttr(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if !cfg.ProviderBalanceBotEnabled {
		logger.Info("provider balance bot disabled")
		return nil
	}

	telegramClient := telegramdelivery.New(telegramdelivery.Config{
		BotToken: cfg.AlertTelegramBotToken,
		ChatID:   cfg.TelegramAdminChatID,
		ThreadID: cfg.TelegramAdminThreadID,
	})
	balanceSvc := providerbalance.New(
		buildProviderBalanceCheckers(cfg),
		telegramClient,
		providerbalance.Config{
			WarnRemainBalance: cfg.APIMartBalanceWarnRemainBalance,
			WarnRemainCredits: cfg.APIMartBalanceWarnRemainCredits,
			CacheTTL:          5 * time.Minute,
		},
	)

	interval := cfg.ProviderBalancePollInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	logger.Info(
		"provider balance bot started",
		"poll_interval", interval.String(),
		"telegram_thread_configured", cfg.TelegramAdminThreadID != 0,
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runTelegramPolling(ctx, telegramClient, balanceSvc, cfg, logger)
	}()
	go func() {
		defer wg.Done()
		runBalanceWarnings(ctx, balanceSvc, interval, logger)
	}()

	<-ctx.Done()
	wg.Wait()
	logger.Info("provider balance bot stopped")
	return nil
}

func buildProviderBalanceCheckers(cfg config.Config) []providerbalance.Checker {
	var checkers []providerbalance.Checker
	if strings.TrimSpace(cfg.APIMartAPIKey) != "" {
		checkers = append(checkers, apimartbalance.New(apimartbalance.Config{
			APIKey:  cfg.APIMartAPIKey,
			BaseURL: cfg.APIMartBaseURL,
		}))
	}
	if cfg.PoYoProviderEnabled {
		checkers = append(checkers, poyobalance.New(poyobalance.Config{
			APIKey:  cfg.PoYoAPIKey,
			BaseURL: cfg.PoYoBaseURL,
		}))
	}
	if cfg.RunwayProviderEnabled {
		checkers = append(checkers, runwaybalance.New(runwaybalance.Config{
			APISecret: cfg.RunwayMLAPISecret,
			BaseURL:   cfg.RunwayMLBaseURL,
		}))
	}
	if cfg.DeepInfraBalanceProviderEnabled {
		checkers = append(checkers, deepinfrabalance.New(deepinfrabalance.Config{
			APIKey:         cfg.DeepInfraAPIKey,
			BalanceBaseURL: cfg.DeepInfraBalanceBaseURL,
		}))
	}
	return checkers
}

type updateClient interface {
	GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]telegramdelivery.Update, error)
}

type commandService interface {
	HandleCommand(ctx context.Context, text string) error
}

type warningService interface {
	CheckAndWarn(ctx context.Context) error
}

func runTelegramPolling(ctx context.Context, client updateClient, svc commandService, cfg config.Config, logger *slog.Logger) {
	var offset int64
	for {
		updates, err := client.GetUpdates(ctx, offset, 30)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			logger.Warn("telegram getUpdates failed", logging.ErrorAttr(err))
			if !sleepContext(ctx, 5*time.Second) {
				return
			}
			continue
		}
		for _, update := range updates {
			if update.ID >= offset {
				offset = update.ID + 1
			}
			if !telegramUpdateAllowed(update, cfg) {
				continue
			}
			if strings.TrimSpace(update.Text) == "" {
				continue
			}
			if err := svc.HandleCommand(ctx, update.Text); err != nil {
				logger.Warn("provider balance command failed", logging.ErrorAttr(err))
			}
		}
	}
}

func runBalanceWarnings(ctx context.Context, svc warningService, interval time.Duration, logger *slog.Logger) {
	check := func() {
		if err := svc.CheckAndWarn(ctx); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			logger.Warn("provider balance warning check failed", logging.ErrorAttr(err))
		}
	}
	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func telegramUpdateAllowed(update telegramdelivery.Update, cfg config.Config) bool {
	if strings.TrimSpace(update.ChatID) != strings.TrimSpace(cfg.TelegramAdminChatID) {
		return false
	}
	if cfg.TelegramAdminThreadID != 0 && update.ThreadID != cfg.TelegramAdminThreadID {
		return false
	}
	return true
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
