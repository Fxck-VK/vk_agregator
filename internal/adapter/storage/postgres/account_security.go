package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountSecurityRepository stores credential verifiers and PII-free account
// security audit rows.
type AccountSecurityRepository struct {
	db Querier
}

func NewAccountSecurityRepository(db Querier) *AccountSecurityRepository {
	return &AccountSecurityRepository{db: db}
}

var _ domain.AccountCredentialRepository = (*AccountSecurityRepository)(nil)
var _ domain.AccountLinkAuditRepository = (*AccountSecurityRepository)(nil)

const accountCredentialColumns = `id, account_id, credential_type, secret_hash, changed_at, created_at, updated_at` // #nosec G101 -- SQL column names only, no credential value.

func (r *AccountSecurityRepository) UpsertCredential(ctx context.Context, credential domain.AccountCredential) (*domain.AccountCredential, error) {
	if err := credential.Validate(); err != nil {
		return nil, err
	}
	const q = `INSERT INTO account_credentials (
			id, account_id, credential_type, secret_hash, changed_at, created_at, updated_at
		) VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4,
			COALESCE($5, now()), COALESCE($6, now()), COALESCE($7, COALESCE($6, now()))
		)
		ON CONFLICT (account_id, credential_type) DO UPDATE
		SET secret_hash = EXCLUDED.secret_hash,
		    changed_at = EXCLUDED.changed_at,
		    updated_at = now()
		RETURNING ` + accountCredentialColumns
	var out domain.AccountCredential
	err := scanAccountCredential(r.db.QueryRow(ctx, q,
		nullableUUIDOrNil(credential.ID),
		credential.AccountID,
		credential.CredentialType,
		credential.SecretHash,
		nullableTimePtr(credential.ChangedAt),
		nullableTime(credential.CreatedAt),
		nullableTime(credential.UpdatedAt),
	), &out)
	if err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSecurityRepository) FindCredential(ctx context.Context, accountID uuid.UUID, credentialType domain.AccountCredentialType) (*domain.AccountCredential, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	const q = `SELECT ` + accountCredentialColumns + `
		FROM account_credentials
		WHERE account_id = $1 AND credential_type = $2`
	var out domain.AccountCredential
	if err := scanAccountCredential(r.db.QueryRow(ctx, q, accountID, credentialType), &out); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSecurityRepository) RecordAccountAudit(ctx context.Context, entry domain.AccountLinkAuditEntry) error {
	if entry.AccountID == uuid.Nil || entry.Action == "" {
		return domain.ErrInvalidIdentity
	}
	const q = `INSERT INTO account_links_audit (
			id, account_id, actor_account_id, action, provider, identity_id, created_at
		) VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, COALESCE($7, now())
		)`
	_, err := r.db.Exec(ctx, q,
		nullableUUIDOrNil(entry.ID),
		entry.AccountID,
		nullableUUIDPtr(entry.ActorAccountID),
		entry.Action,
		emptyProviderAsNil(entry.Provider),
		nullableUUIDPtr(entry.IdentityID),
		nullableTime(entry.CreatedAt),
	)
	return mapError(err)
}

func scanAccountCredential(row rowScanner, credential *domain.AccountCredential) error {
	var changedAt *time.Time
	if err := row.Scan(
		&credential.ID,
		&credential.AccountID,
		&credential.CredentialType,
		&credential.SecretHash,
		&changedAt,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	); err != nil {
		return err
	}
	credential.ChangedAt = changedAt
	return nil
}

func emptyProviderAsNil(provider domain.IdentityProvider) *domain.IdentityProvider {
	if provider == "" {
		return nil
	}
	return &provider
}
