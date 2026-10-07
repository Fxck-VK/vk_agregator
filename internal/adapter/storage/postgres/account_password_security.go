package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/domain"
)

func (r *AccountSecurityRepository) ExecutePasswordSecurity(ctx context.Context, op domain.PasswordSecurityOperation, _ domain.PasswordSecurityDependencies) error {
	if err := op.Validate(); err != nil {
		return err
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return errors.New("password security requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer tx.Rollback(ctx)
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE /* account-password-security */", op.AccountID).Scan(&locked); err != nil {
		return mapError(err)
	}
	var bindingID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM account_identities WHERE account_id=$1 AND provider='email' AND normalized_id=$2 AND verified_at IS NOT NULL AND id=$3 AND updated_at=$4`, op.AccountID, op.EmailBinding.NormalizedID, op.EmailBinding.ID, op.EmailBinding.UpdatedAt).Scan(&bindingID)
	if err != nil {
		return mapError(err)
	}
	security := NewAccountSecurityRepository(tx)
	if !op.Reset {
		stored, err := security.FindCredential(ctx, op.AccountID, domain.AccountCredentialPassword)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if (op.ExpectedHash == "" && stored != nil) || (op.ExpectedHash != "" && (stored == nil || stored.SecretHash != op.ExpectedHash)) {
			return domain.ErrConflict
		}
	}
	if op.PreserveSessionID != uuid.Nil {
		var current uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM account_sessions WHERE account_id=$1 AND id=$2 AND revoked_at IS NULL AND expires_at>$3 AND access_expires_at>$3 FOR UPDATE`, op.AccountID, op.PreserveSessionID, op.At).Scan(&current); err != nil {
			return mapError(err)
		}
	}
	if op.Credential != nil {
		if op.Reset {
			_, err = security.UpsertCredential(ctx, *op.Credential)
		} else {
			_, err = security.CompareAndSwapCredential(ctx, *op.Credential, op.ExpectedHash)
		}
		if err != nil {
			return err
		}
	}
	if op.Audit.Action != domain.AccountLinkActionLogin {
		if _, err := tx.Exec(ctx, `UPDATE account_sessions SET revoked_at=$2, updated_at=$2 WHERE account_id=$1 AND revoked_at IS NULL AND id<>$3`, op.AccountID, op.At, op.PreserveSessionID); err != nil {
			return mapError(err)
		}
	}
	if op.Session != nil {
		if _, err := NewAccountSessionRepository(tx).CreateSession(ctx, *op.Session); err != nil {
			return err
		}
	}
	if err := security.RecordAccountAudit(ctx, op.Audit); err != nil {
		return err
	}
	if op.Audit.Action != domain.AccountLinkActionLogin {
		kind := domain.AccountSecurityNoticePasswordChanged
		if op.Reset {
			kind = domain.AccountSecurityNoticePasswordReset
		}
		if err := EnqueueAccountSecurityNotifications(ctx, tx, op.AccountID, kind, nil, op.At); err != nil {
			return err
		}
	}
	return mapError(tx.Commit(ctx))
}
