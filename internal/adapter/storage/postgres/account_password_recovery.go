package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
)

// ResetCredentialForLinkedEmail linearizes recovery with email unlink/replace.
// All security writes use this transaction; errors preserve the old credential.
func (r *AccountSecurityRepository) ResetCredentialForLinkedEmail(ctx context.Context, credential domain.AccountCredential, email string, revokeSessions, recordAudit bool, at time.Time) error {
	if err := credential.Validate(); err != nil {
		return err
	}
	if credential.CredentialType != domain.AccountCredentialPassword {
		return domain.ErrInvalidIdentity
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return errors.New("password recovery requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer tx.Rollback(ctx)
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", credential.AccountID).Scan(&locked); err != nil {
		return mapError(err)
	}
	var verified bool
	err = tx.QueryRow(ctx, "SELECT verified_at IS NOT NULL FROM account_identities WHERE account_id=$1 AND provider='email' AND normalized_id=$2", credential.AccountID, email).Scan(&verified)
	if err != nil {
		return mapError(err)
	}
	if !verified {
		return domain.ErrUnverifiedLogin
	}
	security := NewAccountSecurityRepository(tx)
	if _, err := security.UpsertCredential(ctx, credential); err != nil {
		return err
	}
	if revokeSessions {
		if _, err := NewAccountSessionRepository(tx).RevokeAllSessions(ctx, credential.AccountID, at); err != nil {
			return err
		}
	}
	if recordAudit {
		if err := security.RecordAccountAudit(ctx, domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: credential.AccountID, Action: domain.AccountLinkActionPasswordReset, Provider: domain.IdentityProviderEmail, CreatedAt: at}); err != nil {
			return err
		}
	}
	return mapError(tx.Commit(ctx))
}
