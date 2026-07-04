package accountoauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestOIDCAdapterVerifiesSubjectOnly(t *testing.T) {
	verifier := &fakeOIDCVerifier{
		claims: OIDCClaims{
			Subject:   "google-subject-123",
			Issuer:    "https://accounts.google.com",
			Audience:  []string{"client-a"},
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	adapter := NewOIDCAdapter(OIDCAdapterConfig{
		Provider: domain.IdentityProviderGoogle,
		Method:   domain.AccountLoginGoogle,
		Issuers:  []string{"https://accounts.google.com"},
		Audience: []string{"client-a"},
		JWKSURL:  GoogleJWKSURL,
		Verifier: verifier,
	})

	login, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderGoogle,
		IDToken:  "signed-token",
	})
	if err != nil {
		t.Fatalf("verify oidc: %v", err)
	}
	if login.Method != domain.AccountLoginGoogle || login.ExternalID != "google-subject-123" || !login.Verified {
		t.Fatalf("login = %+v", login)
	}
	if verifier.seenToken != "signed-token" {
		t.Fatalf("verifier token = %q", verifier.seenToken)
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
