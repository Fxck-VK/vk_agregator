package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidIdentity is returned when a provider identity cannot be normalized.
var ErrInvalidIdentity = errors.New("domain: invalid identity")

// IdentityProvider names an external login/channel identity provider.
type IdentityProvider string

const (
	IdentityProviderVK       IdentityProvider = "vk"
	IdentityProviderTelegram IdentityProvider = "telegram"
	IdentityProviderGoogle   IdentityProvider = "google"
	IdentityProviderApple    IdentityProvider = "apple"
	IdentityProviderEmail    IdentityProvider = "email"
	IdentityProviderPhone    IdentityProvider = "phone"
	IdentityProviderPassword IdentityProvider = "password"
)

// NormalizeIdentityProvider trims and lowercases a provider code.
func NormalizeIdentityProvider(provider IdentityProvider) IdentityProvider {
	return IdentityProvider(strings.ToLower(strings.TrimSpace(string(provider))))
}

// NormalizeExternalIdentity returns the storage key used for identity lookup.
func NormalizeExternalIdentity(provider IdentityProvider, externalID string) (string, error) {
	provider = NormalizeIdentityProvider(provider)
	normalized := strings.TrimSpace(externalID)
	switch provider {
	case IdentityProviderVK, IdentityProviderTelegram:
		normalized = strings.TrimLeft(normalized, "+")
	case IdentityProviderGoogle, IdentityProviderApple, IdentityProviderPassword:
		normalized = strings.ToLower(normalized)
	case IdentityProviderEmail:
		normalized = strings.ToLower(normalized)
	case IdentityProviderPhone:
		replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
		normalized = replacer.Replace(normalized)
	default:
		return "", ErrInvalidIdentity
	}
	if normalized == "" {
		return "", ErrInvalidIdentity
	}
	return normalized, nil
}

// Account is the future canonical owner of money, jobs, artifacts and history.
type Account struct {
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	Role        string    `json:"role"`
	AccountType string    `json:"account_type"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`
	RiskLevel   int       `json:"risk_level"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AccountIdentity binds one external identity to one account.
type AccountIdentity struct {
	ID           uuid.UUID        `json:"id"`
	AccountID    uuid.UUID        `json:"account_id"`
	Provider     IdentityProvider `json:"provider"`
	ExternalID   string           `json:"external_id"`
	NormalizedID string           `json:"normalized_id"`
	VerifiedAt   time.Time        `json:"verified_at"`
	LastUsedAt   time.Time        `json:"last_used_at"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// IdentityResolution is the result of resolving an external identity. User is
// populated while legacy user-scoped ownership is still being migrated.
type IdentityResolution struct {
	AccountID uuid.UUID        `json:"account_id"`
	Identity  *AccountIdentity `json:"identity,omitempty"`
	User      *User            `json:"user,omitempty"`
}
