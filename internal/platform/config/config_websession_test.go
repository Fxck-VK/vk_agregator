package config_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
)

func TestValidateWebOriginFailsClosedInServerEnvironments(t *testing.T) {
	for _, origin := range []string{"", " https://app.example.test", "http://app.example.test", "https://app.example.test/path", "https://user@app.example.test", "https://app.example.test?query=1", "https://app.example.test?", "https://App.Example.test", "https://app.example.test:443", "https://app.example.test/#fragment"} {
		cfg := config.Config{Env: "production", WebOrigin: origin}
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "WEB_ORIGIN") {
			t.Fatalf("WebOrigin %q validation error = %v", origin, err)
		}
	}
	if err := (config.Config{Env: "staging", WebOrigin: "https://app.example.test"}).Validate(); err != nil {
		t.Fatalf("valid staging origin rejected: %v", err)
	}
}

func TestLoadWebOrigin(t *testing.T) {
	t.Setenv("WEB_ORIGIN", "https://app.example.test")
	if got := config.Load().WebOrigin; got != "https://app.example.test" {
		t.Fatalf("WebOrigin = %q", got)
	}
}
