package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountSessionRepository is the PostgreSQL implementation of
// domain.AccountSessionRepository.
type AccountSessionRepository struct {
	db Querier
}

func NewAccountSessionRepository(db Querier) *AccountSessionRepository {
	return &AccountSessionRepository{db: db}
}

var _ domain.AccountSessionRepository = (*AccountSessionRepository)(nil)

const accountSessionColumns = `id, account_id, identity_id, access_token_hash, access_expires_at,
	refresh_token_hash, device_id, ip_hash, user_agent_hash, expires_at, revoked_at, created_at, updated_at`

func (r *AccountSessionRepository) CreateSession(ctx context.Context, session domain.AccountSession) (*domain.AccountSession, error) {
	if err := session.Validate(); err != nil {
		return nil, err
	}
	const q = `INSERT INTO account_sessions (
			id, account_id, identity_id, access_token_hash, access_expires_at, refresh_token_hash,
			device_id, ip_hash, user_agent_hash,
			expires_at, revoked_at, created_at, updated_at
		) VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, COALESCE($12, now()), COALESCE($13, COALESCE($12, now()))
		)
		RETURNING ` + accountSessionColumns
	var out domain.AccountSession
	err := scanAccountSession(r.db.QueryRow(ctx, q,
		nullableUUIDOrNil(session.ID),
		session.AccountID,
		nullableUUIDPtr(session.IdentityID),
		nullableString(session.AccessTokenHash),
		nullableTimePtr(session.AccessExpiresAt),
		session.RefreshTokenHash,
		session.DeviceID,
		session.IPHash,
		session.UserAgentHash,
		session.ExpiresAt,
		nullableTimePtr(session.RevokedAt),
		nullableTime(session.CreatedAt),
		nullableTime(session.UpdatedAt),
	), &out)
	if err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSessionRepository) FindSessionByAccessHash(ctx context.Context, accessTokenHash string) (*domain.AccountSession, error) {
	const q = `SELECT ` + accountSessionColumns + `
		FROM account_sessions
		WHERE access_token_hash = $1`
	var out domain.AccountSession
	if err := scanAccountSession(r.db.QueryRow(ctx, q, accessTokenHash), &out); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSessionRepository) FindSessionByRefreshHash(ctx context.Context, refreshTokenHash string) (*domain.AccountSession, error) {
	const q = `SELECT ` + accountSessionColumns + `
		FROM account_sessions
		WHERE refresh_token_hash = $1`
	var out domain.AccountSession
	if err := scanAccountSession(r.db.QueryRow(ctx, q, refreshTokenHash), &out); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSessionRepository) ListActiveSessionsByAccount(ctx context.Context, accountID uuid.UUID, now time.Time, limit int) ([]*domain.AccountSession, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	const q = `SELECT ` + accountSessionColumns + `
		FROM account_sessions
		WHERE account_id = $1
		  AND revoked_at IS NULL
		  AND expires_at > $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, accountID, now, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := make([]*domain.AccountSession, 0)
	for rows.Next() {
		var session domain.AccountSession
		if err := scanAccountSession(rows, &session); err != nil {
			return nil, mapError(err)
		}
		out = append(out, &session)
	}
	return out, mapError(rows.Err())
}

func (r *AccountSessionRepository) RevokeSession(ctx context.Context, accountID, sessionID uuid.UUID, revokedAt time.Time) (*domain.AccountSession, error) {
	const q = `UPDATE account_sessions
		SET revoked_at = COALESCE(revoked_at, $3),
		    updated_at = $3
		WHERE account_id = $1 AND id = $2
		RETURNING ` + accountSessionColumns
	var out domain.AccountSession
	if err := scanAccountSession(r.db.QueryRow(ctx, q, accountID, sessionID, revokedAt), &out); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSessionRepository) RevokeSessionByRefreshHash(ctx context.Context, refreshTokenHash string, revokedAt time.Time) (*domain.AccountSession, error) {
	const q = `UPDATE account_sessions
		SET revoked_at = $2,
		    updated_at = $2
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
		RETURNING ` + accountSessionColumns
	var out domain.AccountSession
	if err := scanAccountSession(r.db.QueryRow(ctx, q, refreshTokenHash, revokedAt), &out); err != nil {
		return nil, mapError(err)
	}
	return &out, nil
}

func (r *AccountSessionRepository) RevokeAllSessions(ctx context.Context, accountID uuid.UUID, revokedAt time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `UPDATE account_sessions
		SET revoked_at = $2, updated_at = $2
		WHERE account_id = $1 AND revoked_at IS NULL`, accountID, revokedAt)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func scanAccountSession(row rowScanner, session *domain.AccountSession) error {
	var identityID *uuid.UUID
	var accessTokenHash *string
	var accessExpiresAt *time.Time
	var revokedAt *time.Time
	if err := row.Scan(
		&session.ID,
		&session.AccountID,
		&identityID,
		&accessTokenHash,
		&accessExpiresAt,
		&session.RefreshTokenHash,
		&session.DeviceID,
		&session.IPHash,
		&session.UserAgentHash,
		&session.ExpiresAt,
		&revokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return err
	}
	session.IdentityID = identityID
	if accessTokenHash != nil {
		session.AccessTokenHash = *accessTokenHash
	}
	session.AccessExpiresAt = accessExpiresAt
	session.RevokedAt = revokedAt
	return nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableUUIDPtr(id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return id
}

func nullableUUIDOrNil(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func nullableTimePtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	return t
}
