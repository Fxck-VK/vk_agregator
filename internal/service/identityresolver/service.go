// Package identityresolver owns mapping external channel identities to the
// canonical account layer while legacy user-scoped ownership is being migrated.
package identityresolver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// BillingEnsurer is the subset of billingservice.Service needed by the
// resolver to keep legacy user credit accounts available.
type BillingEnsurer interface {
	EnsureAccount(ctx context.Context, userID uuid.UUID) (*domain.CreditAccount, error)
}

// Service resolves external identities into canonical account ids.
type Service struct {
	users      domain.UserRepository
	identities domain.AccountIdentityRepository
	billing    BillingEnsurer
}

// New builds an identity resolver. identities may be nil in old unit tests; in
// that compatibility mode VK resolution still goes through this service and
// returns the legacy user id as AccountID.
func New(users domain.UserRepository, identities domain.AccountIdentityRepository, billing BillingEnsurer) *Service {
	return &Service{users: users, identities: identities, billing: billing}
}

var _ domain.IdentityResolver = (*Service)(nil)

// Resolve returns the account id already bound to provider/externalID.
func (s *Service) Resolve(ctx context.Context, provider domain.IdentityProvider, externalID string) (uuid.UUID, error) {
	provider, normalizedID, _, err := normalize(provider, externalID)
	if err != nil {
		return uuid.Nil, err
	}
	if s.identities == nil {
		return uuid.Nil, domain.ErrNotFound
	}
	identity, err := s.identities.ResolveIdentity(ctx, provider, normalizedID)
	if err != nil {
		return uuid.Nil, err
	}
	return identity.AccountID, nil
}

// ResolveOrCreate resolves an identity or creates the account/identity binding.
func (s *Service) ResolveOrCreate(ctx context.Context, provider domain.IdentityProvider, externalID string) (domain.IdentityResolution, error) {
	provider, normalizedID, externalID, err := normalize(provider, externalID)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	if provider == domain.IdentityProviderVK {
		return s.resolveOrCreateVK(ctx, externalID, normalizedID)
	}
	if s.identities == nil {
		return domain.IdentityResolution{}, domain.ErrNotFound
	}
	identity, err := s.identities.ResolveIdentity(ctx, provider, normalizedID)
	if err == nil {
		return domain.IdentityResolution{AccountID: identity.AccountID, Identity: identity}, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.IdentityResolution{}, err
	}
	identity, err = s.identities.CreateAccountWithIdentity(ctx, provider, externalID, normalizedID)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	return domain.IdentityResolution{AccountID: identity.AccountID, Identity: identity}, nil
}

// LinkIdentity binds another external identity to an account.
func (s *Service) LinkIdentity(ctx context.Context, accountID uuid.UUID, provider domain.IdentityProvider, externalID string) (*domain.AccountIdentity, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	provider, normalizedID, externalID, err := normalize(provider, externalID)
	if err != nil {
		return nil, err
	}
	if s.identities == nil {
		return nil, domain.ErrNotFound
	}
	return s.identities.LinkIdentity(ctx, accountID, provider, externalID, normalizedID)
}

// UnlinkIdentity removes an identity binding from an account.
func (s *Service) UnlinkIdentity(ctx context.Context, accountID, identityID uuid.UUID) error {
	if accountID == uuid.Nil || identityID == uuid.Nil {
		return domain.ErrInvalidIdentity
	}
	if s.identities == nil {
		return domain.ErrNotFound
	}
	return s.identities.UnlinkIdentity(ctx, accountID, identityID)
}

func (s *Service) resolveOrCreateVK(ctx context.Context, externalID, normalizedID string) (domain.IdentityResolution, error) {
	if s.users == nil {
		return domain.IdentityResolution{}, errors.New("identity resolver: users repository is required")
	}
	vkUserID, err := strconv.ParseInt(normalizedID, 10, 64)
	if err != nil || vkUserID <= 0 {
		return domain.IdentityResolution{}, domain.ErrInvalidIdentity
	}
	created := false
	user, err := s.users.GetByVKUserID(ctx, vkUserID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.IdentityResolution{}, err
		}
		user = &domain.User{
			VKUserID: vkUserID,
			Role:     domain.RoleUser,
			Status:   domain.StatusActive,
			Locale:   "ru",
			Timezone: "Europe/Moscow",
		}
		if err := s.users.Create(ctx, user); err != nil {
			if !errors.Is(err, domain.ErrConflict) {
				return domain.IdentityResolution{}, err
			}
			user, err = s.users.GetByVKUserID(ctx, vkUserID)
			if err != nil {
				return domain.IdentityResolution{}, err
			}
		} else {
			created = true
		}
	}
	accountID := user.ID
	var identity *domain.AccountIdentity
	if s.identities != nil {
		identity, err = s.identities.EnsureIdentityForUser(ctx, user, domain.IdentityProviderVK, externalID, normalizedID)
		if err != nil {
			return domain.IdentityResolution{}, fmt.Errorf("ensure identity: %w", err)
		}
		accountID = identity.AccountID
	}
	user.AccountID = accountID
	if created && s.billing != nil {
		if _, err := s.billing.EnsureAccount(ctx, user.ID); err != nil {
			return domain.IdentityResolution{}, fmt.Errorf("ensure billing account: %w", err)
		}
	}
	return domain.IdentityResolution{AccountID: accountID, Identity: identity, User: user}, nil
}

func normalize(provider domain.IdentityProvider, externalID string) (domain.IdentityProvider, string, string, error) {
	provider = domain.NormalizeIdentityProvider(provider)
	externalID = strings.TrimSpace(externalID)
	normalizedID, err := domain.NormalizeExternalIdentity(provider, externalID)
	if err != nil {
		return "", "", "", err
	}
	return provider, normalizedID, externalID, nil
}
