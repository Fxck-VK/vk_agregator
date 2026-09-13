package config_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestAPIMartPublicImageFlagsAndRequiredConfig(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		enabled func(config.Config) bool
		config  func() config.Config
	}{
		{
			name:    "gpt image 2.5 flare",
			flag:    "FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED",
			enabled: func(cfg config.Config) bool { return cfg.FeatureAPIMartGPTImage25FlareEnabled },
			config:  func() config.Config { return config.Config{FeatureAPIMartGPTImage25FlareEnabled: true} },
		},
		{
			name:    "gpt image 2.5 sunburst",
			flag:    "FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED",
			enabled: func(cfg config.Config) bool { return cfg.FeatureAPIMartGPTImage25SunburstEnabled },
			config:  func() config.Config { return config.Config{FeatureAPIMartGPTImage25SunburstEnabled: true} },
		},
		{
			name:    "seedream 5.0 lite",
			flag:    "FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED",
			enabled: func(cfg config.Config) bool { return cfg.FeatureAPIMartSeedream50LiteEnabled },
			config:  func() config.Config { return config.Config{FeatureAPIMartSeedream50LiteEnabled: true} },
		},
		{
			name:    "seedream 5.0 pro",
			flag:    "FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED",
			enabled: func(cfg config.Config) bool { return cfg.FeatureAPIMartSeedream50ProEnabled },
			config:  func() config.Config { return config.Config{FeatureAPIMartSeedream50ProEnabled: true} },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.flag, "")
			if test.enabled(config.Load()) {
				t.Fatal("APIMart image route enabled by default")
			}
			t.Setenv(test.flag, "true")
			if !test.enabled(config.Load()) {
				t.Fatal("APIMart image route flag not loaded")
			}

			cfg := test.config()
			cfg.Env = "development"
			cfg.Provider = "mock"
			cfg.ProviderChain = []string{"mock"}
			for _, required := range []string{"APIMART_PROVIDER_ENABLED", "APIMART_API_KEY", "APIMART_BASE_URL"} {
				if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), required) {
					t.Fatalf("expected missing %s, got %v", required, err)
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
