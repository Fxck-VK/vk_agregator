package config

import (
	"testing"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestKIETextRequiresExplicitVerificationAndCredentials(t *testing.T) {
	cfg := Config{KIEBaseURL: "https://api.kie.ai", FeatureTextGPT55Enabled: true}
	if cfg.validateKIEText() == nil || len(cfg.KIETextModels()) != 0 {
		t.Fatal("unconfigured model enabled")
	}
	cfg.KIEProviderEnabled = true
	cfg.KIEAPIKey = "test-key"
	if cfg.validateKIEText() == nil || len(cfg.KIETextModels()) != 0 {
		t.Fatal("unverified limits enabled")
	}
	cfg.KIETextLimitsVerified = true
	if err := cfg.validateKIEText(); err != nil || len(cfg.KIETextModels()) != 1 {
		t.Fatal(err)
	}
	cfg.KIEBaseURL = "https://api.kie.ai@other.invalid"
	if cfg.validateKIEText() == nil {
		t.Fatal("unsafe base URL accepted")
	}
}

func TestTextModelEnvFlagsDefaultOffAndResolveProvider(t *testing.T) {
	for _, model := range providermodels.PaidTextModels() {
		t.Setenv(model.FeatureFlag, "false")
	}
	t.Setenv("KIE_TEXT_LIMITS_VERIFIED", "false")
	t.Setenv("APIMART_TEXT_LIMITS_VERIFIED", "false")
	cfg := Load()
	for _, model := range providermodels.PaidTextModels() {
		if cfg.TextModelEnabled(model.PublicID) {
			t.Fatal("disabled model enabled")
		}
		t.Setenv(model.FeatureFlag, "true")
	}
	t.Setenv("KIE_PROVIDER_ENABLED", "true")
	t.Setenv("KIE_API_KEY", "synthetic")
	t.Setenv("KIE_BASE_URL", "https://api.kie.ai")
	t.Setenv("APIMART_PROVIDER_ENABLED", "true")
	t.Setenv("APIMART_API_KEY", "synthetic")
	t.Setenv("APIMART_BASE_URL", "https://api.apimart.ai/v1")
	cfg = Load()
	if len(cfg.KIETextModels()) != 0 || len(cfg.APIMartTextModels()) != 0 {
		t.Fatal("unverified models registered")
	}
	t.Setenv("KIE_TEXT_LIMITS_VERIFIED", "true")
	t.Setenv("APIMART_TEXT_LIMITS_VERIFIED", "true")
	cfg = Load()
	for _, model := range providermodels.PaidTextModels() {
		if !cfg.TextModelEnabled(model.PublicID) {
			t.Fatalf("env flag missing: %s", model.PublicID)
		}
	}
	if len(cfg.KIETextModels()) != 10 || len(cfg.APIMartTextModels()) != 1 || cfg.APIMartTextModels()[0] != "claude-fable-5.1" {
		t.Fatal("incorrect provider split")
	}
	if err := cfg.validateKIEText(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.validateAPIMartText(); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"http://api.apimart.ai/v1", "https://api.apimart.ai@other.invalid/v1", "https://api.apimart.ai/v1?key=synthetic", "https://api.apimart.ai"} {
		cfg.APIMartBaseURL = base
		if cfg.validateAPIMartText() == nil {
			t.Fatal("unsafe/incompatible text base accepted")
		}
	}
}
