package accountoauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestOIDCAdapterVerifiesSubjectOnly(t *testing.T) {
	tests := []struct {
		name     string
		provider domain.IdentityProvider
		method   domain.AccountLoginMethod
		issuer   string
		subject  string
		jwksURL  string
	}{
		{
			name:     "google",
			provider: domain.IdentityProviderGoogle,
			method:   domain.AccountLoginGoogle,
			issuer:   "https://accounts.google.com",
			subject:  "google-subject-123",
			jwksURL:  GoogleJWKSURL,
		},
		{
			name:     "apple",
			provider: domain.IdentityProviderApple,
			method:   domain.AccountLoginApple,
			issuer:   "https://appleid.apple.com",
			subject:  "apple-subject-123",
			jwksURL:  AppleJWKSURL,
		},
		{
			name:     "vk id",
			provider: domain.IdentityProviderVK,
			method:   domain.AccountLoginVKID,
			issuer:   "https://vkid.example.test",
			subject:  "777000",
			jwksURL:  "https://vkid.example.test/jwks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeOIDCVerifier{
				claims: OIDCClaims{
					Subject:   tt.subject,
					Issuer:    tt.issuer,
					Audience:  []string{"client-a"},
					ExpiresAt: time.Now().Add(time.Hour),
				},
			}
			adapter := NewOIDCAdapter(OIDCAdapterConfig{
				Provider: tt.provider,
				Method:   tt.method,
				Issuers:  []string{tt.issuer},
				Audience: []string{"client-a"},
				JWKSURL:  tt.jwksURL,
				Verifier: verifier,
			})

			login, err := adapter.Verify(context.Background(), VerifyRequest{
				Provider: tt.provider,
				IDToken:  "signed-token",
			})
			if err != nil {
				t.Fatalf("verify oidc: %v", err)
			}
			if login.Method != tt.method || login.ExternalID != tt.subject || !login.Verified {
				t.Fatalf("login = %+v", login)
			}
			if verifier.seenToken != "signed-token" {
				t.Fatalf("verifier token = %q", verifier.seenToken)
			}
		})
	}
}

func TestOIDCAdapterFailsClosedWithoutTrustMaterial(t *testing.T) {
	adapter := NewOIDCAdapter(OIDCAdapterConfig{
		Provider: domain.IdentityProviderApple,
		Method:   domain.AccountLoginApple,
		Issuers:  []string{"https://appleid.apple.com"},
		Verifier: &fakeOIDCVerifier{},
	})

	if _, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderApple,
		IDToken:  "signed-token",
	}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

type fakeOIDCVerifier struct {
	claims    OIDCClaims
	err       error
	seenToken string
}

func (v *fakeOIDCVerifier) VerifyIDToken(_ context.Context, token string, _ OIDCVerifyConfig) (OIDCClaims, error) {
	v.seenToken = token
	if v.err != nil {
		return OIDCClaims{}, v.err
	}
	return v.claims, nil
}
