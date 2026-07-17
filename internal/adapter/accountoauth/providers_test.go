package accountoauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestRegistryFromConfigRegistersProvidersFailClosed(t *testing.T) {
	registry := NewRegistryFromConfig(Config{
		Clock: func() time.Time { return time.Unix(1_700_000_000, 0) },
	})

	tests := []struct {
		name     string
		provider domain.IdentityProvider
		req      VerifyRequest
	}{
		{
			name:     "google",
			provider: domain.IdentityProviderGoogle,
			req: VerifyRequest{
				Provider: domain.IdentityProviderGoogle,
				IDToken:  "signed-google-token",
			},
		},
		{
			name:     "apple",
			provider: domain.IdentityProviderApple,
			req: VerifyRequest{
				Provider: domain.IdentityProviderApple,
				IDToken:  "signed-apple-token",
			},
		},
		{
			name:     "vk id",
			provider: domain.IdentityProviderVK,
			req: VerifyRequest{
				Provider: domain.IdentityProviderVK,
				IDToken:  "signed-vkid-token",
			},
		},
		{
			name:     "telegram",
			provider: domain.IdentityProviderTelegram,
			req: VerifyRequest{
				Provider: domain.IdentityProviderTelegram,
				AuthData: map[string]string{
					"id":        "123",
					"auth_date": "1700000000",
					"hash":      "not-a-real-hash",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if registry.adapters[tt.provider] == nil {
				t.Fatalf("provider %s was not registered", tt.provider)
			}
			if _, err := registry.Verify(context.Background(), tt.req); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("error = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestRegistryFromConfigRejectsUnsupportedProvider(t *testing.T) {
	registry := NewRegistryFromConfig(Config{})

	if _, err := registry.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderEmail,
		IDToken:  "client-supplied-token",
	}); !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("error = %v, want ErrUnsupportedProvider", err)
	}
}
