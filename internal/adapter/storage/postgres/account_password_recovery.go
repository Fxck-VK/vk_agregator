package postgres

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

// ResetCredentialForLinkedEmail retains the old adapter API but always commits
// revocation, audit and notice with the credential. Flags cannot weaken security.
func (r *AccountSecurityRepository) ResetCredentialForLinkedEmail(ctx context.Context, credential domain.AccountCredential, email string, _, _ bool, at time.Time) error {
	binding, err := NewAccountIdentityRepository(r.db).ResolveIdentity(ctx, domain.IdentityProviderEmail, email)
	if err != nil {
		return err
	}
	return r.ExecutePasswordSecurity(ctx, domain.PasswordSecurityOperation{AccountID: credential.AccountID, EmailBinding: *binding, Credential: &credential, Reset: true, At: at, Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: credential.AccountID, Action: domain.AccountLinkActionPasswordReset, Provider: domain.IdentityProviderEmail, CreatedAt: at}}, domain.PasswordSecurityDependencies{})
}
