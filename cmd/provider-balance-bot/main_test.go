package main

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/providerbalance"
)

func TestRunReturnsCleanlyWhenProviderBalanceBotDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{}

	if err := run(context.Background(), cfg, logger); err != nil {
		t.Fatalf("run disabled returned error: %v", err)
	}
}

func TestRunValidatesMissingRequiredConfigWhenEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{ProviderBalanceBotEnabled: true}

	err := run(context.Background(), cfg, logger)
	if err == nil {
		t.Fatal("expected config validation error")
	}
	if !strings.Contains(err.Error(), "ALERT_TELEGRAM_BOT_TOKEN") {
		t.Fatalf("expected ALERT_TELEGRAM_BOT_TOKEN validation error, got %v", err)
	}
}

func TestBuildProviderBalanceCheckersReturnsAPIMartOnlyWhenPoYoDisabled(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartAPIKey:       "apimart-key",
		APIMartBaseURL:      "https://api.apimart.ai/v1",
		PoYoProviderEnabled: false,
		PoYoAPIKey:          "poyo-key",
		PoYoBaseURL:         "https://api.poyo.ai",
	})

	if got, want := checkerNames(checkers), []string{"apimart"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func TestBuildProviderBalanceCheckersIncludesPoYoWhenEnabled(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartAPIKey:       "apimart-key",
		APIMartBaseURL:      "https://api.apimart.ai/v1",
		PoYoProviderEnabled: true,
		PoYoAPIKey:          "poyo-key",
		PoYoBaseURL:         "https://api.poyo.ai",
	})

	if got, want := checkerNames(checkers), []string{"apimart", "poyo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func TestBuildProviderBalanceCheckersSkipsAPIMartWithoutKey(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartBaseURL:      "https://api.apimart.ai/v1",
		PoYoProviderEnabled: true,
		PoYoAPIKey:          "poyo-key",
		PoYoBaseURL:         "https://api.poyo.ai",
	})

	if got, want := checkerNames(checkers), []string{"poyo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func TestBuildProviderBalanceCheckersIncludesRunwayWhenEnabled(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartAPIKey:         "apimart-key",
		APIMartBaseURL:        "https://api.apimart.ai/v1",
		RunwayProviderEnabled: true,
		RunwayMLAPISecret:     "runway-key",
		RunwayMLBaseURL:       "https://api.dev.runwayml.com/v1",
	})

	if got, want := checkerNames(checkers), []string{"apimart", "runway"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func TestBuildProviderBalanceCheckersIncludesDeepInfraWhenEnabled(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartAPIKey:                   "apimart-key",
		APIMartBaseURL:                  "https://api.apimart.ai/v1",
		DeepInfraAPIKey:                 "deepinfra-key",
		DeepInfraBalanceProviderEnabled: true,
		DeepInfraBalanceBaseURL:         "https://api.deepinfra.com",
	})

	if got, want := checkerNames(checkers), []string{"apimart", "deepinfra"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func TestBuildProviderBalanceCheckersIncludesAllOptionalProviders(t *testing.T) {
	checkers := buildProviderBalanceCheckers(config.Config{
		APIMartAPIKey:                   "apimart-key",
		APIMartBaseURL:                  "https://api.apimart.ai/v1",
		PoYoProviderEnabled:             true,
		PoYoAPIKey:                      "poyo-key",
		PoYoBaseURL:                     "https://api.poyo.ai",
		RunwayProviderEnabled:           true,
		RunwayMLAPISecret:               "runway-key",
		RunwayMLBaseURL:                 "https://api.dev.runwayml.com/v1",
		DeepInfraAPIKey:                 "deepinfra-key",
		DeepInfraBalanceProviderEnabled: true,
		DeepInfraBalanceBaseURL:         "https://api.deepinfra.com",
	})

	if got, want := checkerNames(checkers), []string{"apimart", "poyo", "runway", "deepinfra"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checker names = %#v, want %#v", got, want)
	}
}

func checkerNames(checkers []providerbalance.Checker) []string {
	names := make([]string, 0, len(checkers))
	for _, checker := range checkers {
		names = append(names, checker.Name())
	}
	return names
}
