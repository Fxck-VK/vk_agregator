package accountoauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestTelegramAdapterVerifiesLegacyAuthData(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	data := signedTelegramData("bot-token", map[string]string{
		"id":         "987654321",
		"first_name": "Safe",
		"auth_date":  strconv.FormatInt(now.Add(-time.Minute).Unix(), 10),
	})
	adapter := NewTelegramAdapter(TelegramConfig{
		BotToken: "bot-token",
		Clock:    func() time.Time { return now },
	})

	login, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderTelegram,
		AuthData: data,
	})
	if err != nil {
		t.Fatalf("verify telegram: %v", err)
	}
	if login.Method != domain.AccountLoginTelegram || login.ExternalID != "987654321" || !login.Verified {
		t.Fatalf("login = %+v", login)
	}
}

func TestTelegramAdapterVerifiesOIDCIDToken(t *testing.T) {
	verifier := &fakeOIDCVerifier{
		claims: OIDCClaims{
			Subject:   "987654321",
			Issuer:    TelegramIssuer,
			Audience:  []string{"123456"},
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	adapter := NewTelegramAdapter(TelegramConfig{
		ClientIDs: []string{"123456"},
		Verifier:  verifier,
	})

	login, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderTelegram,
		IDToken:  "telegram-id-token",
	})
	if err != nil {
		t.Fatalf("verify telegram oidc: %v", err)
	}
	if login.Method != domain.AccountLoginTelegram || login.ExternalID != "987654321" || !login.Verified {
		t.Fatalf("login = %+v", login)
	}
	if verifier.seenToken != "telegram-id-token" {
		t.Fatalf("verifier token = %q", verifier.seenToken)
	}
}

func TestTelegramAdapterRejectsTamperedLegacyAuthData(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	data := signedTelegramData("bot-token", map[string]string{
		"id":         "987654321",
		"first_name": "Safe",
		"auth_date":  strconv.FormatInt(now.Add(-time.Minute).Unix(), 10),
	})
	data["first_name"] = "Tampered"
	adapter := NewTelegramAdapter(TelegramConfig{
		BotToken: "bot-token",
		Clock:    func() time.Time { return now },
	})

	if _, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderTelegram,
		AuthData: data,
	}); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatalf("error = %v, want ErrInvalidAssertion", err)
	}
}

func TestTelegramAdapterRejectsExpiredLegacyAuthData(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	data := signedTelegramData("bot-token", map[string]string{
		"id":        "987654321",
		"auth_date": strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10),
	})
	adapter := NewTelegramAdapter(TelegramConfig{
		BotToken: "bot-token",
		MaxAge:   time.Hour,
		Clock:    func() time.Time { return now },
	})

	if _, err := adapter.Verify(context.Background(), VerifyRequest{
		Provider: domain.IdentityProviderTelegram,
		AuthData: data,
	}); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatalf("error = %v, want ErrInvalidAssertion", err)
	}
}

func signedTelegramData(botToken string, data map[string]string) map[string]string {
	copied := cloneMap(data)
	checkString := telegramCheckString(copied)
	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(checkString))
	copied["hash"] = hex.EncodeToString(mac.Sum(nil))
	return copied
}
