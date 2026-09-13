package config_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestGrokImageFlagsAndRequiredConfig(t *testing.T) {
	for _, version := range []string{"1_5", "2_0"} {
		t.Run(version, func(t *testing.T) {
			flag := "FEATURE_APIMART_GROK_IMAGE_" + version + "_ENABLED"
			enabled := func(cfg config.Config) bool {
				if version == "1_5" {
					return cfg.FeatureAPIMartGrokImage15Enabled
				}
				return cfg.FeatureAPIMartGrokImage20Enabled
			}
			t.Setenv(flag, "")
			if enabled(config.Load()) {
				t.Fatal("Grok enabled by default")
			}
			t.Setenv(flag, "true")
			if !enabled(config.Load()) {
				t.Fatal("Grok flag not loaded")
			}
			cfg := config.Config{Env: "development", Provider: "mock", ProviderChain: []string{"mock"}, FeatureAPIMartGrokImage15Enabled: version == "1_5", FeatureAPIMartGrokImage20Enabled: version == "2_0"}
			for _, required := range []string{"APIMART_PROVIDER_ENABLED", "APIMART_API_KEY", "APIMART_BASE_URL"} {
				if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), required) {
					t.Fatalf("expected missing %s", required)
				}
				switch required {
				case "APIMART_PROVIDER_ENABLED":
					cfg.APIMartProviderEnabled = true
				case "APIMART_API_KEY":
					cfg.APIMartAPIKey = "test-key"
				case "APIMART_BASE_URL":
					cfg.APIMartBaseURL = "https://api.apimart.ai/v1"
				}
			}
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
