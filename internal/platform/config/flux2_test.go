package config_test

import (
	"strings"
	"testing"
	"vk-ai-aggregator/internal/platform/config"
)

func TestFlux2FlagDefaultsOffAndRequiresAPIMart(t *testing.T) {
	defer clearEnv(t, "FEATURE_APIMART_FLUX_2_PRO_ENABLED")()
	if config.Load().FeatureAPIMartFlux2ProEnabled {
		t.Fatal("FLUX.2 enabled by default")
	}
	t.Setenv("FEATURE_APIMART_FLUX_2_PRO_ENABLED", "true")
	if !config.Load().FeatureAPIMartFlux2ProEnabled {
		t.Fatal("FLUX.2 flag not loaded")
	}
	cfg := config.Config{Env: "development", Provider: "mock", ProviderChain: []string{"mock"}, FeatureAPIMartFlux2ProEnabled: true}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "FEATURE_APIMART_FLUX_2_PRO_ENABLED") {
		t.Fatalf("missing provider not rejected: %v", err)
	}
}
