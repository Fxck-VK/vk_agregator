package config_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestDEVModelSmokeRequiresDevelopmentAndProviderReadiness(t *testing.T) {
	base := config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://api.example.com"}
	for name, mutate := range map[string]func(*config.Config){
		"production":        func(c *config.Config) { c.Env = "production" },
		"loadtest":          func(c *config.Config) { c.Env = "loadtest" },
		"staging":           func(c *config.Config) { c.Env = "staging" },
		"missing key":       func(c *config.Config) { c.APIMartAPIKey = "" },
		"missing base":      func(c *config.Config) { c.APIMartBaseURL = "" },
		"provider disabled": func(c *config.Config) { c.APIMartProviderEnabled = false },
		"router disabled":   func(c *config.Config) { c.FeatureVideoRouterEnabled = false },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "FEATURE_DEV_MODEL_SMOKE_ENABLED") {
				t.Fatalf("missing smoke guard: %v", err)
			}
		})
	}
	t.Setenv("FEATURE_DEV_MODEL_SMOKE_ENABLED", "")
	if config.Load().FeatureDEVModelSmokeEnabled {
		t.Fatal("smoke must default off")
	}
	t.Setenv("FEATURE_DEV_MODEL_SMOKE_ENABLED", "true")
	if !config.Load().FeatureDEVModelSmokeEnabled {
		t.Fatal("explicit smoke flag not loaded")
	}
}
