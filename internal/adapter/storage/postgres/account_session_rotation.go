package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/domain"
)

// A single statement makes replacement and revocation one transaction. A
// conflicting concurrent refresh gets no row; insertion failure rolls back the
// revocation. Neither operation can commit without the other.
func (r *AccountSessionRepository) RotateSession(ctx context.Context, oldRefreshHash string, session domain.AccountSession) (*domain.AccountSession, error) {
	if err := session.Validate(); err != nil {
		return nil, err
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, errors.New("session rotation requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer tx.Rollback(ctx)
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE /* account-session-rotation */", session.AccountID).Scan(&locked); err != nil {
		return nil, mapError(err)
	}
	const q = `WITH revoked AS (
  UPDATE account_sessions SET revoked_at = $12, updated_at = $12
  WHERE refresh_token_hash = $1 AND account_id = $3 AND revoked_at IS NULL AND expires_at > $12
  RETURNING id
 ) INSERT INTO account_sessions (
  id, account_id, identity_id, access_token_hash, access_expires_at, refresh_token_hash,
  device_id, ip_hash, user_agent_hash, expires_at, created_at, updated_at
 ) SELECT $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12 FROM revoked
 RETURNING ` + accountSessionColumns
	var out domain.AccountSession
	err = scanAccountSession(tx.QueryRow(ctx, q, oldRefreshHash, session.ID, session.AccountID,
		nullableUUIDPtr(session.IdentityID), session.AccessTokenHash, nullableTimePtr(session.AccessExpiresAt),
		session.RefreshTokenHash, session.DeviceID, session.IPHash, session.UserAgentHash, session.ExpiresAt, session.CreatedAt), &out)
	if err != nil {
		return nil, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}
