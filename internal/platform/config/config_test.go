package config_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestValidateProductionSecrets(t *testing.T) {
	cfg := config.Config{Env: "production", VKConfirmationToken: "dev-confirmation"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"VK_SECRET", "ADMIN_TOKEN", "VK_CONFIRMATION_TOKEN"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %s in validation error, got %q", want, msg)
		}
	}
}

func TestValidateRealModesRequireCredentialsOutsideProduction(t *testing.T) {
	cfg := config.Config{
		Env:                 "development",
		Provider:            "openai",
		VKDeliveryMode:      "real",
		VKConfirmationToken: "dev-confirmation",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"OPENAI_API_KEY", "VK_ACCESS_TOKEN"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %s in validation error, got %q", want, msg)
		}
	}
}

func TestLoadProviderChain(t *testing.T) {
	t.Setenv("PROVIDER", "mock")
	t.Setenv("PROVIDER_CHAIN", "openai,mock")

	cfg := config.Load()
	if !reflect.DeepEqual(cfg.ProviderChain, []string{"openai", "mock"}) {
		t.Fatalf("provider chain = %#v", cfg.ProviderChain)
	}
}

func TestValidateOpenAIModerationRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:                "development",
		Provider:           "mock",
		ProviderChain:      []string{"mock"},
		ModerationProvider: "openai",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY validation error, got %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
