// Package accountservice is the account boundary used by product surfaces.
// VK Bot, Mini App and future Web/Mobile handlers should call this package
// instead of reading or mutating account identities directly.
package accountservice

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultIdentityLimit = 50
	maxIdentityLimit     = 100
)

var errMissingDependency = errors.New("accountservice: missing dependency")

// IdentityStore exposes account identities to the account boundary. Concrete
// repositories may store raw identifiers, but AccountService only returns safe
// DTOs.
type IdentityStore interface {
	ListIdentitiesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.AccountIdentity, error)
}

// IdentityLinker is satisfied by accountauth.Service. It keeps verified
// assertion checks, ownership checks, rate limiting and audit writes outside
// product surfaces.
type IdentityLinker interface {
	LinkVerifiedIdentity(ctx context.Context, actorAccountID, accountID uuid.UUID, login domain.VerifiedAccountLogin) (*domain.AccountIdentity, error)
	UnlinkIdentity(ctx context.Context, actorAccountID, accountID, identityID uuid.UUID) error
}

// Service is the public account boundary for profile and identity management.
type Service struct {
	identities IdentityStore
	linker     IdentityLinker
}

// New builds an account service.
func New(identities IdentityStore, linker IdentityLinker) *Service {
	return &Service{identities: identities, linker: linker}
}

// AccountProfile is safe to expose to authenticated product surfaces.
type AccountProfile struct {
	AccountID    uuid.UUID             `json:"account_id"`
	IdentityRefs []AccountIdentitySafe `json:"identity_refs"`
}

// AccountIdentitySafe is a display-safe account identity DTO. It intentionally
// omits external_id and normalized_id.
type AccountIdentitySafe struct {
	ID         uuid.UUID               `json:"id"`
	AccountID  uuid.UUID               `json:"account_id"`
	Provider   domain.IdentityProvider `json:"provider"`
	Label      string                  `json:"label"`
	Verified   bool                    `json:"verified"`
	LastUsedAt *string                 `json:"last_used_at,omitempty"`
	CreatedAt  string                  `json:"created_at"`
}

// Profile returns the account's safe profile and currently linked identities.
func (s *Service) Profile(ctx context.Context, accountID uuid.UUID) (AccountProfile, error) {
	identities, err := s.ListIdentities(ctx, accountID, defaultIdentityLimit, 0)
	if err != nil {
		return AccountProfile{}, err
	}
	return AccountProfile{
		AccountID:    accountID,
		IdentityRefs: identities,
	}, nil
}

// ListIdentities returns safe identity references for one account.
func (s *Service) ListIdentities(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]AccountIdentitySafe, error) {
	if s == nil || s.identities == nil {
		return nil, errMissingDependency
	}
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	limit = normalizeLimit(limit)
	rows, err := s.identities.ListIdentitiesByAccount(ctx, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]AccountIdentitySafe, 0, len(rows))
	for _, identity := range rows {
		out = append(out, safeIdentityDTO(identity))
	}
	return out, nil
}

// LinkVerifiedIdentity links a verified identity to the current account and
// returns a safe identity reference.
func (s *Service) LinkVerifiedIdentity(ctx context.Context, actorAccountID, accountID uuid.UUID, login domain.VerifiedAccountLogin) (AccountIdentitySafe, error) {
	if s == nil || s.linker == nil {
		return AccountIdentitySafe{}, errMissingDependency
	}
	identity, err := s.linker.LinkVerifiedIdentity(ctx, actorAccountID, accountID, login)
	if err != nil {
		return AccountIdentitySafe{}, err
	}
	return safeIdentityDTO(identity), nil
}

// UnlinkIdentity removes an identity through the shared auth/link boundary.
func (s *Service) UnlinkIdentity(ctx context.Context, actorAccountID, accountID, identityID uuid.UUID) error {
	if s == nil || s.linker == nil {
		return errMissingDependency
	}
	return s.linker.UnlinkIdentity(ctx, actorAccountID, accountID, identityID)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultIdentityLimit
	}
	if limit > maxIdentityLimit {
		return maxIdentityLimit
	}
	return limit
}

func safeIdentityDTO(identity *domain.AccountIdentity) AccountIdentitySafe {
	if identity == nil {
		return AccountIdentitySafe{}
	}
	ref := identity.SafeRef()
	out := AccountIdentitySafe{
		ID:        ref.ID,
		AccountID: ref.AccountID,
		Provider:  ref.Provider,
		Label:     identityLabel(*identity),
		Verified:  ref.Verified,
		CreatedAt: ref.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if ref.LastUsedAt != nil {
		formatted := ref.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		out.LastUsedAt = &formatted
	}
	return out
}

func identityLabel(identity domain.AccountIdentity) string {
	switch domain.NormalizeIdentityProvider(identity.Provider) {
	case domain.IdentityProviderEmail:
		return maskEmail(identity.NormalizedID)
	case domain.IdentityProviderPhone:
		return maskPhone(identity.NormalizedID)
	case domain.IdentityProviderVK:
		return "VK"
	case domain.IdentityProviderTelegram:
		return "Telegram"
	case domain.IdentityProviderGoogle:
		return "Google"
	case domain.IdentityProviderApple:
		return "Apple"
	case domain.IdentityProviderPassword:
		return "Password"
	default:
		return "Identity"
	}
}

func maskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "Email"
	}
	local := []rune(parts[0])
	if len(local) == 1 {
		return "*@" + parts[1]
	}
	return string(local[0]) + "***@" + parts[1]
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 4 {
		return "Phone"
	}
	prefix := ""
	if strings.HasPrefix(phone, "+") {
		prefix = "+"
		phone = strings.TrimPrefix(phone, "+")
	}
	if len(phone) <= 4 {
		return "Phone"
	}
	return prefix + strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}
