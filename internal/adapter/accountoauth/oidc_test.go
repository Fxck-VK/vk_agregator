package accountoauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestJWKSCacheDoesNotShareKeysAcrossProvidersWithSameKeyID(t *testing.T) {
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := first
		if r.URL.Path == "/second" {
			key = second
		}
		w.Header().Set("Cache-Control", "max-age=3600")
		json.NewEncoder(w).Encode(jwksResponse{Keys: []jwkKey{{Kty: "RSA", Kid: "shared-kid", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	defer server.Close()
	verifier := NewRemoteJWKSOIDCVerifier(server.Client())
	config := func(name string) OIDCVerifyConfig {
		return OIDCVerifyConfig{Issuers: []string{name}, Audiences: []string{"client"}, JWKSURL: server.URL + "/" + name}
	}
	token := func(issuer string, key *rsa.PrivateKey) string {
		header, _ := json.Marshal(jwtHeader{Algorithm: "RS256", KeyID: "shared-kid"})
		claims, _ := json.Marshal(map[string]any{"sub": "subject", "iss": issuer, "aud": "client", "exp": time.Now().Add(time.Hour).Unix()})
		signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
		sum := sha256.Sum256([]byte(signed))
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if err != nil {
			t.Fatal(err)
		}
		return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
	}
	if _, err := verifier.VerifyIDToken(context.Background(), token("first", first), config("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyIDToken(context.Background(), token("second", first), config("second")); err == nil {
		t.Fatal("another provider's cached key accepted a forged token")
	}
	if _, err := verifier.VerifyIDToken(context.Background(), token("second", second), config("second")); err != nil {
		t.Fatal(err)
	}
}

func TestAppleStringEmailVerifiedClaimIsAccepted(t *testing.T) {
	var raw rawOIDCClaims
	if err := json.Unmarshal([]byte(`{"sub":"subject","nonce":"nonce","email_verified":"true"}`), &raw); err != nil {
		t.Fatal(err)
	}
	if !raw.toClaims().EmailVerified || raw.toClaims().Nonce != "nonce" {
		t.Fatal("claims changed")
	}
}

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
