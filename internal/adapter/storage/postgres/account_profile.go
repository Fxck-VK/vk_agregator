package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.AccountProfileRepository = (*AccountIdentityRepository)(nil)

func (r *AccountIdentityRepository) GetAccountDisplayName(ctx context.Context, accountID uuid.UUID) (string, error) {
	if accountID == uuid.Nil {
		return "", domain.ErrInvalidIdentity
	}
	var name string
	err := r.db.QueryRow(ctx, "SELECT display_name FROM accounts WHERE id=$1", accountID).Scan(&name)
	return name, mapError(err)
}

func (r *AccountIdentityRepository) SetAccountDisplayNameIfEmpty(ctx context.Context, accountID, identityID uuid.UUID, name string) error {
	name = domain.NormalizeAccountDisplayName(name)
	if accountID == uuid.Nil || identityID == uuid.Nil || name == "" {
		return domain.ErrInvalidIdentity
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return errors.New("account profile initialization requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer tx.Rollback(ctx)
	// Share the account lock used by unlink/password mutations, then check the
	// binding with a fresh snapshot. A removed/foreign binding cannot set a name.
	var existing string
	if err := tx.QueryRow(ctx, "SELECT display_name FROM accounts WHERE id=$1 FOR UPDATE", accountID).Scan(&existing); err != nil {
		return mapError(err)
	}
	var verified bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM account_identities WHERE id=$2 AND account_id=$1 AND provider='vk' AND verified_at IS NOT NULL)", accountID, identityID).Scan(&verified); err != nil {
		return mapError(err)
	}
	if !verified {
		return domain.ErrUnverifiedLogin
	}
	if existing == "" {
		if _, err := tx.Exec(ctx, "UPDATE accounts SET display_name=$2, updated_at=now() WHERE id=$1 AND display_name=''", accountID, name); err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}
