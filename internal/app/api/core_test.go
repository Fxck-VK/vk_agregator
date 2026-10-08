package api

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

func TestUsableLoginProvidersFromConfigRequiresWorkingCapabilities(t *testing.T) {
	providers := usableLoginProvidersFromConfig(config.Config{
		WebOrigin:                         "https://app.example.test",
		AccountOAuthGoogleEnabled:         true,
		AccountOAuthGoogleClientIDs:       []string{"google-web"},
		AccountOAuthAppleClientIDs:        []string{"apple-web"},
		AccountOAuthTelegramClientIDs:     []string{"telegram-web"},
		AccountOAuthVKIDClientIDs:         []string{"vk-web"},
		AccountWebOAuthGoogleClientSecret: "google-secret",
	})
	if !providers.Has(domain.IdentityProviderEmail) {
		t.Fatal("email password capability must protect the primary email")
	}
	for _, disabled := range []domain.IdentityProvider{
		domain.IdentityProviderApple,
		domain.IdentityProviderTelegram,
	} {
		if providers.Has(disabled) {
			t.Fatalf("%s counted without a working verification path", disabled)
		}
	}
	if !providers.Has(domain.IdentityProviderGoogle) {
		t.Fatal("enabled Google with a client id must count as usable")
	}
	if !providers.Has(domain.IdentityProviderVK) {
		t.Fatal("configured VK ID client must count as usable")
	}
}

func TestUsableLoginProvidersFromConfigKeepsGoogleDisabledUntilOptedIn(t *testing.T) {
	providers := usableLoginProvidersFromConfig(config.Config{
		AccountOAuthGoogleClientIDs:       []string{"google-web"},
		AccountWebOAuthGoogleClientSecret: "google-secret",
	})
	if providers.Has(domain.IdentityProviderGoogle) {
		t.Fatal("Google credentials must not count while the feature flag is disabled")
	}
}

func TestUsableLoginProvidersFromConfigCountsConfiguredAlternatePaths(t *testing.T) {
	providers := usableLoginProvidersFromConfig(config.Config{
		WebOrigin:                           "https://app.example.test",
		AccountOAuthAppleClientIDs:          []string{"apple-web"},
		AccountWebOAuthAppleClientSecret:    "apple-secret",
		AccountOAuthTelegramBotToken:        "telegram-bot-token",
		AccountOAuthVKIDClientIDs:           []string{"vk-web"},
		AccountWebOAuthTelegramClientSecret: "telegram-secret",
	})
	for _, enabled := range []domain.IdentityProvider{
		domain.IdentityProviderApple,
		domain.IdentityProviderTelegram,
		domain.IdentityProviderVK,
	} {
		if !providers.Has(enabled) {
			t.Fatalf("%s not counted with configured login capability", enabled)
		}
	}
}
