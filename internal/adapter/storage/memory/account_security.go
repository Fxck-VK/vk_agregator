package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountSecurityRepo stores account credentials and security audit events for
// tests and local mock runs.
type AccountSecurityRepo struct {
	mu          sync.Mutex
	credentials map[string]*domain.AccountCredential
	audits      []domain.AccountLinkAuditEntry
}

func NewAccountSecurityRepo() *AccountSecurityRepo {
	return &AccountSecurityRepo{
		credentials: map[string]*domain.AccountCredential{},
	}
}

var _ domain.AccountCredentialRepository = (*AccountSecurityRepo)(nil)
var _ domain.AccountLinkAuditRepository = (*AccountSecurityRepo)(nil)

func (r *AccountSecurityRepo) UpsertCredential(_ context.Context, credential domain.AccountCredential) (*domain.AccountCredential, error) {
	if err := credential.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	key := accountCredentialKey(credential.AccountID, credential.CredentialType)
	if existing, ok := r.credentials[key]; ok {
		existing.SecretHash = credential.SecretHash
		if credential.ChangedAt != nil {
			changedAt := credential.ChangedAt.UTC()
			existing.ChangedAt = &changedAt
		} else {
			existing.ChangedAt = &now
		}
		existing.UpdatedAt = now
		return cloneAccountCredential(existing), nil
	}
	if credential.ID == uuid.Nil {
		credential.ID = uuid.New()
	}
	if credential.CreatedAt.IsZero() {
		credential.CreatedAt = now
	}
	if credential.UpdatedAt.IsZero() {
		credential.UpdatedAt = credential.CreatedAt
	}
	if credential.ChangedAt == nil {
		changedAt := credential.UpdatedAt
		credential.ChangedAt = &changedAt
	}
	cp := credential
	r.credentials[key] = &cp
	return cloneAccountCredential(&cp), nil
}

func (r *AccountSecurityRepo) FindCredential(_ context.Context, accountID uuid.UUID, credentialType domain.AccountCredentialType) (*domain.AccountCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	credential, ok := r.credentials[accountCredentialKey(accountID, credentialType)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneAccountCredential(credential), nil
}

func (r *AccountSecurityRepo) RecordAccountAudit(_ context.Context, entry domain.AccountLinkAuditEntry) error {
	if entry.AccountID == uuid.Nil || entry.Action == "" {
		return domain.ErrInvalidIdentity
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	r.audits = append(r.audits, entry)
	return nil
}

// AuditEntries returns PII-free security audit rows for tests.
func (r *AccountSecurityRepo) AuditEntries() []domain.AccountLinkAuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.AccountLinkAuditEntry, len(r.audits))
	copy(out, r.audits)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID.String() < out[j].ID.String()
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func accountCredentialKey(accountID uuid.UUID, credentialType domain.AccountCredentialType) string {
	return accountID.String() + ":" + string(credentialType)
}

func cloneAccountCredential(credential *domain.AccountCredential) *domain.AccountCredential {
	if credential == nil {
		return nil
	}
	cp := *credential
	if credential.ChangedAt != nil {
		t := *credential.ChangedAt
		cp.ChangedAt = &t
	}
	return &cp
}
