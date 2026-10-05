package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
)

func (r *AccountIdentityRepository) lockEmailAccount(ctx context.Context, accountID uuid.UUID) (pgx.Tx, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, errors.New("account email management requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", accountID).Scan(&locked); err != nil {
		_ = tx.Rollback(ctx)
		return nil, mapError(err)
	}
	return tx, nil
}

func emailRows(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) ([]*domain.AccountIdentity, error) {
	rows, err := tx.Query(ctx, "SELECT "+accountIdentityColumns+" FROM account_identities WHERE account_id=$1 AND provider='email'", accountID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := make([]*domain.AccountIdentity, 0)
	for rows.Next() {
		var identity domain.AccountIdentity
		if err := scanAccountIdentity(rows, &identity); err != nil {
			return nil, mapError(err)
		}
		out = append(out, &identity)
	}
	return out, mapError(rows.Err())
}

func (r *AccountIdentityRepository) linkEmailIdentity(ctx context.Context, accountID uuid.UUID, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	tx, err := r.lockEmailAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var existing domain.AccountIdentity
	err = scanAccountIdentity(tx.QueryRow(ctx, "SELECT "+accountIdentityColumns+" FROM account_identities WHERE provider='email' AND normalized_id=$1", normalizedID), &existing)
	if err == nil {
		if existing.AccountID != accountID {
			return nil, domain.ErrConflict
		}
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapError(err)
	}
	rows, err := emailRows(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	if len(rows) >= domain.MaxAccountEmails {
		return nil, domain.ErrAccountEmailLimit
	}
	const q = `WITH stamp AS (
 SELECT GREATEST(clock_timestamp(), COALESCE(max(created_at) + interval '1 microsecond', clock_timestamp())) AS at
 FROM account_identities WHERE account_id=$1 AND provider='email'
 ), inserted AS (
 INSERT INTO account_identities (id,account_id,provider,external_id,normalized_id,verified_at,last_used_at,created_at,updated_at)
 SELECT $2,$1,'email',$3,$4,at,at,at,at FROM stamp RETURNING ` + accountIdentityColumns + `
 ), audit AS (
 INSERT INTO account_links_audit (id,account_id,actor_account_id,action,provider,identity_id,created_at)
 SELECT $5,account_id,NULL,'linked',provider,id,clock_timestamp() FROM inserted RETURNING id
 ) SELECT ` + accountIdentityColumns + ` FROM inserted`
	var identity domain.AccountIdentity
	if err := scanAccountIdentity(tx.QueryRow(ctx, q, accountID, uuid.New(), externalID, normalizedID, uuid.New()), &identity); err != nil {
		return nil, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return &identity, nil
}

func (r *AccountIdentityRepository) ReplaceBackupEmailIdentity(ctx context.Context, accountID, identityID uuid.UUID, externalID, normalizedID string, expected time.Time) (*domain.AccountIdentity, error) {
	tx, err := r.lockEmailAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := emailRows(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	var target *domain.AccountIdentity
	for _, row := range rows {
		if row.ID == identityID {
			target = row
		}
	}
	if target == nil || expected.IsZero() || !target.UpdatedAt.Equal(expected) || domain.AccountEmailRoles(rows)[identityID] != domain.AccountEmailBackup {
		return nil, domain.ErrAccountBackupEmailChanged
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, "SELECT account_id FROM account_identities WHERE provider='email' AND normalized_id=$1", normalizedID).Scan(&existing)
	if err == nil {
		if existing == accountID {
			return nil, domain.ErrAccountEmailAlreadyLinked
		}
		return nil, domain.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapError(err)
	}
	const q = `WITH stamp AS (SELECT GREATEST(clock_timestamp(), $5::timestamptz + interval '1 microsecond') AS at), changed AS (
 UPDATE account_identities SET external_id=$3, normalized_id=$4, verified_at=stamp.at,last_used_at=stamp.at,updated_at=stamp.at
 FROM stamp WHERE account_id=$1 AND id=$2 AND updated_at=$5 RETURNING ` + `account_identities.id, account_identities.account_id, provider, external_id, normalized_id, verified_at, last_used_at, created_at, updated_at` + `
 ), audit AS (
 INSERT INTO account_links_audit (id,account_id,actor_account_id,action,provider,identity_id,created_at)
 SELECT $6::uuid,account_id,NULL::uuid,'unlinked',provider,id,clock_timestamp() FROM changed
 UNION ALL SELECT $7::uuid,account_id,NULL::uuid,'linked',provider,id,clock_timestamp() FROM changed RETURNING id
 ) SELECT ` + accountIdentityColumns + ` FROM changed`
	var identity domain.AccountIdentity
	err = scanAccountIdentity(tx.QueryRow(ctx, q, accountID, identityID, externalID, normalizedID, expected, uuid.New(), uuid.New()), &identity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountBackupEmailChanged
	}
	if err != nil {
		return nil, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return &identity, nil
}
