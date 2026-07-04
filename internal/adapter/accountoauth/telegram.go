package accountoauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

// TelegramAdapter verifies Telegram Login Widget/Bot login auth data.
type TelegramAdapter struct {
	botToken string
	maxAge   time.Duration
	clock    func() time.Time
	oidc     *OIDCAdapter
}

// TelegramConfig configures Telegram login verification.
type TelegramConfig struct {
	BotToken  string
	MaxAge    time.Duration
	Clock     func() time.Time
	ClientIDs []string
	Issuer    string
	JWKSURL   string
	Verifier  OIDCVerifier
}

// NewTelegramAdapter creates a fail-closed Telegram login adapter.
func NewTelegramAdapter(cfg TelegramConfig) *TelegramAdapter {
	maxAge := cfg.MaxAge
	if maxAge == 0 {
		maxAge = 24 * time.Hour
	}
	adapter := &TelegramAdapter{
		botToken: strings.TrimSpace(cfg.BotToken),
		maxAge:   maxAge,
		clock:    cfg.Clock,
	}
	if cfg.Verifier != nil {
		issuer := strings.TrimSpace(cfg.Issuer)
		if issuer == "" {
			issuer = TelegramIssuer
		}
		jwksURL := strings.TrimSpace(cfg.JWKSURL)
		if jwksURL == "" {
			jwksURL = TelegramJWKSURL
		}
		adapter.oidc = NewOIDCAdapter(OIDCAdapterConfig{
			Provider: domain.IdentityProviderTelegram,
			Method:   domain.AccountLoginTelegram,
			Issuers:  []string{issuer},
			Audience: cfg.ClientIDs,
			JWKSURL:  jwksURL,
			Verifier: cfg.Verifier,
			Clock:    cfg.Clock,
		})
	}
	return adapter
}

// Provider returns Telegram identity provider.
func (a *TelegramAdapter) Provider() domain.IdentityProvider {
	return domain.IdentityProviderTelegram
}

// Verify checks Telegram HMAC auth data and returns the verified Telegram id.
func (a *TelegramAdapter) Verify(ctx context.Context, req VerifyRequest) (domain.VerifiedAccountLogin, error) {
	if a == nil {
		return domain.VerifiedAccountLogin{}, ErrUnavailable
	}
	if domain.NormalizeIdentityProvider(req.Provider) != domain.IdentityProviderTelegram {
		return domain.VerifiedAccountLogin{}, ErrUnsupportedProvider
	}
	if strings.TrimSpace(req.IDToken) != "" {
		if a.oidc == nil {
			return domain.VerifiedAccountLogin{}, ErrUnavailable
		}
		return a.oidc.Verify(ctx, req)
	}
	if a.botToken == "" {
		return domain.VerifiedAccountLogin{}, ErrUnavailable
	}
	data := cloneMap(req.AuthData)
	id := strings.TrimSpace(data["id"])
	hashHex := strings.TrimSpace(data["hash"])
	authDateRaw := strings.TrimSpace(data["auth_date"])
	if id == "" || hashHex == "" || authDateRaw == "" {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	authDate, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	now := clockNow(a.clock)
	authTime := time.Unix(authDate, 0)
	if authTime.After(now.Add(2*time.Minute)) || now.Sub(authTime) > a.maxAge {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	delete(data, "hash")
	checkString := telegramCheckString(data)
	secret := sha256.Sum256([]byte(a.botToken))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(checkString))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(hashHex))) {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	return domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginTelegram,
		ExternalID: id,
		Verified:   true,
	}, nil
}

func telegramCheckString(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for key, value := range data {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+data[key])
	}
	return strings.Join(parts, "\n")
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
