package accountoauth

import (
	"net/http"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	GoogleJWKSURL   = "https://www.googleapis.com/oauth2/v3/certs"
	AppleJWKSURL    = "https://appleid.apple.com/auth/keys"
	TelegramIssuer  = "https://oauth.telegram.org"
	TelegramJWKSURL = "https://oauth.telegram.org/.well-known/jwks.json"
)

// Config contains all OAuth adapter configuration used by cmd/api.
type Config struct {
	GoogleClientIDs []string
	GoogleJWKSURL   string
	AppleClientIDs  []string
	AppleJWKSURL    string

	TelegramBotToken  string
	TelegramMaxAge    time.Duration
	TelegramClientIDs []string
	TelegramIssuer    string
	TelegramJWKSURL   string

	VKIDClientIDs []string
	VKIDIssuer    string
	VKIDJWKSURL   string

	Clock func() time.Time
}

// NewRegistryFromConfig builds all configured OAuth adapters. Providers with
// missing trust material are still registered fail-closed, so endpoints return
// a controlled 503 instead of trusting client data.
func NewRegistryFromConfig(cfg Config) *Registry {
	verifier := NewRemoteJWKSOIDCVerifier(&http.Client{Timeout: defaultOAuthHTTPTimeout})
	googleJWKSURL := strings.TrimSpace(cfg.GoogleJWKSURL)
	if googleJWKSURL == "" {
		googleJWKSURL = GoogleJWKSURL
	}
	appleJWKSURL := strings.TrimSpace(cfg.AppleJWKSURL)
	if appleJWKSURL == "" {
		appleJWKSURL = AppleJWKSURL
	}
	return NewRegistry(
		NewOIDCAdapter(OIDCAdapterConfig{
			Provider: domain.IdentityProviderGoogle,
			Method:   domain.AccountLoginGoogle,
			Issuers:  []string{"accounts.google.com", "https://accounts.google.com"},
			Audience: cfg.GoogleClientIDs,
			JWKSURL:  googleJWKSURL,
			Verifier: verifier,
			Clock:    cfg.Clock,
		}),
		NewOIDCAdapter(OIDCAdapterConfig{
			Provider: domain.IdentityProviderApple,
			Method:   domain.AccountLoginApple,
			Issuers:  []string{"https://appleid.apple.com"},
			Audience: cfg.AppleClientIDs,
			JWKSURL:  appleJWKSURL,
			Verifier: verifier,
			Clock:    cfg.Clock,
		}),
		NewOIDCAdapter(OIDCAdapterConfig{
			Provider: domain.IdentityProviderVK,
			Method:   domain.AccountLoginVKID,
			Issuers:  []string{cfg.VKIDIssuer},
			Audience: cfg.VKIDClientIDs,
			JWKSURL:  cfg.VKIDJWKSURL,
			Verifier: verifier,
			Clock:    cfg.Clock,
		}),
		NewTelegramAdapter(TelegramConfig{
			BotToken:  cfg.TelegramBotToken,
			MaxAge:    cfg.TelegramMaxAge,
			Clock:     cfg.Clock,
			ClientIDs: cfg.TelegramClientIDs,
			Issuer:    cfg.TelegramIssuer,
			JWKSURL:   cfg.TelegramJWKSURL,
			Verifier:  verifier,
		}),
	)
}
