package config_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestQwenImageFlagAndRequiredConfig(t *testing.T) {
	t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "")
	if config.Load().FeatureAPIMartQwenImage3Enabled {
		t.Fatal("Qwen enabled by default")
	}
	t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "true")
	if !config.Load().FeatureAPIMartQwenImage3Enabled {
		t.Fatal("Qwen flag not loaded")
	}
	cfg := config.Config{Env: "development", Provider: "mock", ProviderChain: []string{"mock"}, FeatureAPIMartQwenImage3Enabled: true}
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
}
