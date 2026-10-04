package memory

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type AccountRegistrationRepo struct {
	identities *AccountIdentityRepo
	security   *AccountSecurityRepo
}

func NewAccountRegistrationRepo(identities *AccountIdentityRepo, security *AccountSecurityRepo) *AccountRegistrationRepo {
	return &AccountRegistrationRepo{identities, security}
}

var _ domain.AccountRegistrationRepository = (*AccountRegistrationRepo)(nil)

func (r *AccountRegistrationRepo) RegisterEmailAccount(_ context.Context, accountID uuid.UUID, email, hash string, now time.Time) (domain.IdentityResolution, error) {
	if accountID == uuid.Nil || email == "" || hash == "" {
		return domain.IdentityResolution{}, domain.ErrInvalidIdentity
	}
	r.identities.mu.Lock()
	defer r.identities.mu.Unlock()
	r.security.mu.Lock()
	defer r.security.mu.Unlock()
	key := identityKey(domain.IdentityProviderEmail, email)
	if existing := r.identities.byKey[key]; existing != nil {
		if existing.AccountID != accountID || existing.VerifiedAt.IsZero() || r.security.credentials[accountCredentialKey(accountID, domain.AccountCredentialPassword)] == nil {
			return domain.IdentityResolution{}, domain.ErrConflict
		}
		return domain.IdentityResolution{AccountID: accountID, Identity: cloneAccountIdentity(existing)}, nil
	}
	// This ID belongs to a server-issued proof and cannot reuse another identity.
	for _, identity := range r.identities.byID {
		if identity.AccountID == accountID {
			return domain.IdentityResolution{}, domain.ErrConflict
		}
	}
	identity := &domain.AccountIdentity{ID: uuid.New(), AccountID: accountID, Provider: domain.IdentityProviderEmail, ExternalID: email, NormalizedID: email, VerifiedAt: now, LastUsedAt: now, CreatedAt: now, UpdatedAt: now}
	r.identities.byKey[key] = identity
	r.identities.byID[identity.ID] = identity
	credential := &domain.AccountCredential{ID: uuid.New(), AccountID: accountID, CredentialType: domain.AccountCredentialPassword, SecretHash: hash, ChangedAt: &now, CreatedAt: now, UpdatedAt: now}
	r.security.credentials[accountCredentialKey(accountID, domain.AccountCredentialPassword)] = credential
	r.identities.appendAuditLocked(identity, domain.AccountLinkActionLinked)
	r.security.audits = append(r.security.audits, domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: accountID, Action: domain.AccountLinkActionPasswordSet, Provider: domain.IdentityProviderEmail, CreatedAt: now})
	return domain.IdentityResolution{AccountID: accountID, Identity: cloneAccountIdentity(identity)}, nil
}
