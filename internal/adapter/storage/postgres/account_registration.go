package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.AccountRegistrationRepository = (*AccountIdentityRepository)(nil)

func (r *AccountIdentityRepository) registrationRetry(ctx context.Context, accountID uuid.UUID, email string) (domain.IdentityResolution, error) {
	identity, err := r.ResolveIdentity(ctx, domain.IdentityProviderEmail, email)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	if identity.AccountID != accountID || identity.VerifiedAt.IsZero() {
		return domain.IdentityResolution{}, domain.ErrConflict
	}
	if _, err := NewAccountSecurityRepository(r.db).FindCredential(ctx, accountID, domain.AccountCredentialPassword); err != nil {
		return domain.IdentityResolution{}, err
	}
	return domain.IdentityResolution{AccountID: accountID, Identity: identity}, nil
}

// One data-modifying statement rolls all inserts back together on conflict or
// storage failure. No ON CONFLICT UPDATE can change an existing password.
func (r *AccountIdentityRepository) RegisterEmailAccount(ctx context.Context, accountID uuid.UUID, email, hash string, now time.Time) (domain.IdentityResolution, error) {
	if accountID == uuid.Nil || email == "" || hash == "" {
		return domain.IdentityResolution{}, domain.ErrInvalidIdentity
	}
	if result, err := r.registrationRetry(ctx, accountID, email); !errors.Is(err, domain.ErrNotFound) {
		return result, err
	}
	const query = `WITH new_account AS (
  INSERT INTO accounts (id,status,role,account_type,locale,timezone,risk_level,created_at,updated_at)
  VALUES ($1,'active','user','personal','ru','Europe/Moscow',0,$4,$4) RETURNING id
 ), new_identity AS (
  INSERT INTO account_identities (id,account_id,provider,external_id,normalized_id,verified_at,last_used_at,created_at,updated_at)
  SELECT $5,id,'email',$2,$2,$4,$4,$4,$4 FROM new_account RETURNING ` + accountIdentityColumns + `
 ), new_credential AS (
  INSERT INTO account_credentials (id,account_id,credential_type,secret_hash,changed_at,created_at,updated_at)
  SELECT $6,account_id,'password',$3,$4,$4,$4 FROM new_identity RETURNING account_id
 ), audit AS (
  INSERT INTO account_links_audit (id,account_id,action,provider,identity_id,created_at)
  SELECT $7::uuid,account_id,'linked','email',id,$4 FROM new_identity
  UNION ALL SELECT $8::uuid,account_id,'password_set','email',NULL,$4 FROM new_credential
 ) SELECT ` + accountIdentityColumns + ` FROM new_identity`
	var identity domain.AccountIdentity
	err := mapError(scanAccountIdentity(r.db.QueryRow(ctx, query, accountID, email, hash, now, uuid.New(), uuid.New(), uuid.New(), uuid.New()), &identity))
	if errors.Is(err, domain.ErrConflict) {
		return r.registrationRetry(ctx, accountID, email)
	}
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	return domain.IdentityResolution{AccountID: identity.AccountID, Identity: &identity}, nil
}
