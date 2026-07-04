package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountIdentityRepository is the PostgreSQL implementation of
// domain.AccountIdentityRepository.
type AccountIdentityRepository struct {
	db Querier
}

// NewAccountIdentityRepository builds an AccountIdentityRepository over db.
func NewAccountIdentityRepository(db Querier) *AccountIdentityRepository {
	return &AccountIdentityRepository{db: db}
}

var _ domain.AccountIdentityRepository = (*AccountIdentityRepository)(nil)

const accountIdentityColumns = `id, account_id, provider, external_id, normalized_id,
	verified_at, last_used_at, created_at, updated_at`

// ResolveIdentity fetches an identity by provider and normalized id.
func (r *AccountIdentityRepository) ResolveIdentity(ctx context.Context, provider domain.IdentityProvider, normalizedID string) (*domain.AccountIdentity, error) {
	const q = `SELECT ` + accountIdentityColumns + `
		FROM account_identities
		WHERE provider = $1 AND normalized_id = $2`
	var identity domain.AccountIdentity
	if err := mapError(scanAccountIdentity(r.db.QueryRow(ctx, q, provider, normalizedID), &identity)); err != nil {
		return nil, err
	}
	return &identity, nil
}

// EnsureIdentityForUser binds a legacy user to an account identity.
func (r *AccountIdentityRepository) EnsureIdentityForUser(ctx context.Context, user *domain.User, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	if user == nil || user.ID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	const q = `
		WITH existing AS (
			SELECT account_id
			FROM account_identities
			WHERE provider = $1 AND normalized_id = $2
		),
		user_account AS (
			SELECT account_id
			FROM users
			WHERE id = $6 AND account_id IS NOT NULL
		),
		inserted_account AS (
			INSERT INTO accounts (
				id, status, role, account_type, locale, timezone, risk_level,
				created_at, updated_at
			)
			SELECT
				$3,
				CASE WHEN $8 IN ('active', 'blocked', 'deleted') THEN $8 ELSE 'active' END,
				CASE WHEN $9 IN ('user', 'moderator', 'admin', 'operator') THEN $9 ELSE 'user' END,
				'personal',
				COALESCE(NULLIF($10, ''), 'ru'),
				COALESCE(NULLIF($11, ''), 'Europe/Moscow'),
				GREATEST($12, 0),
				now(),
				now()
			WHERE NOT EXISTS (SELECT 1 FROM existing)
			  AND NOT EXISTS (SELECT 1 FROM user_account)
			RETURNING id AS account_id
		),
		chosen AS (
			SELECT account_id FROM existing
			UNION ALL
			SELECT account_id FROM user_account
			UNION ALL
			SELECT account_id FROM inserted_account
			LIMIT 1
		),
		upserted_identity AS (
			INSERT INTO account_identities (
				id, account_id, provider, external_id, normalized_id,
				verified_at, last_used_at, created_at, updated_at
			)
			SELECT $4, account_id, $1, $5, $2, now(), now(), now(), now()
			FROM chosen
			ON CONFLICT (provider, normalized_id) DO UPDATE
			SET last_used_at = now(), updated_at = now()
			RETURNING ` + accountIdentityColumns + `
		),
		linked_user AS (
			UPDATE users
			SET account_id = (SELECT account_id FROM upserted_identity),
			    updated_at = now()
			WHERE id = $6
			  AND (account_id IS NULL OR account_id <> (SELECT account_id FROM upserted_identity))
			RETURNING id
		),
		audit AS (
			INSERT INTO account_links_audit (
				id, account_id, actor_account_id, action, provider, identity_id, created_at
			)
			SELECT $7, account_id, NULL, 'linked', provider, id, now()
			FROM upserted_identity
			WHERE NOT EXISTS (
				SELECT 1
				FROM account_links_audit a
				WHERE a.identity_id = upserted_identity.id
				  AND a.action = 'linked'
				  AND a.provider = upserted_identity.provider
			)
			RETURNING id
		)
		SELECT ` + accountIdentityColumns + ` FROM upserted_identity`
	var identity domain.AccountIdentity
	err := scanAccountIdentity(r.db.QueryRow(ctx, q,
		provider,
		normalizedID,
		uuid.New(),
		uuid.New(),
		externalID,
		user.ID,
		uuid.New(),
		string(user.Status),
		string(user.Role),
		user.Locale,
		user.Timezone,
		user.RiskLevel,
	), &identity)
	if err != nil {
		return nil, mapError(err)
	}
	user.AccountID = identity.AccountID
	return &identity, nil
}

// CreateAccountWithIdentity creates a standalone account and identity.
func (r *AccountIdentityRepository) CreateAccountWithIdentity(ctx context.Context, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	const q = `
		WITH inserted_account AS (
			INSERT INTO accounts (id, status, role, account_type, locale, timezone, risk_level, created_at, updated_at)
			VALUES ($3, 'active', 'user', 'personal', 'ru', 'Europe/Moscow', 0, now(), now())
			RETURNING id
		),
		inserted_identity AS (
			INSERT INTO account_identities (
				id, account_id, provider, external_id, normalized_id,
				verified_at, last_used_at, created_at, updated_at
			)
			SELECT $4, id, $1, $5, $2, now(), now(), now(), now()
			FROM inserted_account
			RETURNING ` + accountIdentityColumns + `
		),
		audit AS (
			INSERT INTO account_links_audit (id, account_id, actor_account_id, action, provider, identity_id, created_at)
			SELECT $6, account_id, NULL, 'linked', provider, id, now()
			FROM inserted_identity
			RETURNING id
		)
		SELECT ` + accountIdentityColumns + ` FROM inserted_identity`
	var identity domain.AccountIdentity
	err := scanAccountIdentity(r.db.QueryRow(ctx, q, provider, normalizedID, uuid.New(), uuid.New(), externalID, uuid.New()), &identity)
	if err != nil {
		return nil, mapError(err)
	}
	return &identity, nil
}

func (r *AccountIdentityRepository) ListIdentitiesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.AccountIdentity, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	const q = `SELECT ` + accountIdentityColumns + `
		FROM account_identities
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, accountID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	identities := make([]*domain.AccountIdentity, 0)
	for rows.Next() {
		var identity domain.AccountIdentity
		if err := scanAccountIdentity(rows, &identity); err != nil {
			return nil, mapError(err)
		}
		identities = append(identities, &identity)
	}
	return identities, mapError(rows.Err())
}

// LinkIdentity attaches an identity to an existing account.
func (r *AccountIdentityRepository) LinkIdentity(ctx context.Context, accountID uuid.UUID, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	existing, err := r.ResolveIdentity(ctx, provider, normalizedID)
	if err == nil {
		if existing.AccountID == accountID {
			return existing, nil
		}
		return nil, domain.ErrConflict
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	const q = `
		WITH inserted_identity AS (
			INSERT INTO account_identities (
				id, account_id, provider, external_id, normalized_id,
				verified_at, last_used_at, created_at, updated_at
			)
			VALUES ($4, $3, $1, $5, $2, now(), now(), now(), now())
			RETURNING ` + accountIdentityColumns + `
		),
		audit AS (
			INSERT INTO account_links_audit (id, account_id, actor_account_id, action, provider, identity_id, created_at)
			SELECT $6, account_id, NULL, 'linked', provider, id, now()
			FROM inserted_identity
			RETURNING id
		)
		SELECT ` + accountIdentityColumns + ` FROM inserted_identity`
	var identity domain.AccountIdentity
	err = scanAccountIdentity(r.db.QueryRow(ctx, q, provider, normalizedID, accountID, uuid.New(), externalID, uuid.New()), &identity)
	if err != nil {
		return nil, mapError(err)
	}
	return &identity, nil
}

// UnlinkIdentity removes one identity from an account.
func (r *AccountIdentityRepository) UnlinkIdentity(ctx context.Context, accountID, identityID uuid.UUID) error {
	const q = `
		WITH target AS (
			SELECT id, account_id, provider
			FROM account_identities
			WHERE id = $2 AND account_id = $1
		),
		stats AS (
			SELECT
				(SELECT count(*) FROM target) AS target_count,
				(SELECT count(*) FROM account_identities WHERE account_id = $1) AS account_count
		),
		guarded AS (
			SELECT target.*
			FROM target, stats
			WHERE stats.account_count > 1
		),
		audit AS (
			INSERT INTO account_links_audit (id, account_id, actor_account_id, action, provider, identity_id, created_at)
			SELECT $3, account_id, NULL, 'unlinked', provider, id, now()
			FROM guarded
			RETURNING id
		),
		deleted AS (
			DELETE FROM account_identities ai
			USING guarded
			WHERE ai.id = guarded.id
			RETURNING ai.id
		)
		SELECT COALESCE(
			(SELECT 'deleted' FROM deleted LIMIT 1),
			CASE
				WHEN (SELECT target_count FROM stats) = 0 THEN 'not_found'
				WHEN (SELECT account_count FROM stats) <= 1 THEN 'last_identity'
				ELSE 'not_found'
			END
		)`
	var status string
	if err := r.db.QueryRow(ctx, q, accountID, identityID, uuid.New()).Scan(&status); err != nil {
		return mapError(err)
	}
	switch status {
	case "deleted":
		return nil
	case "last_identity":
		return domain.ErrAccountLastIdentity
	default:
		return domain.ErrNotFound
	}
}

func scanAccountIdentity(row rowScanner, identity *domain.AccountIdentity) error {
	var verifiedAt, lastUsedAt *time.Time
	if err := row.Scan(
		&identity.ID,
		&identity.AccountID,
		&identity.Provider,
		&identity.ExternalID,
		&identity.NormalizedID,
		&verifiedAt,
		&lastUsedAt,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	); err != nil {
		return err
	}
	if verifiedAt != nil {
		identity.VerifiedAt = *verifiedAt
	}
	if lastUsedAt != nil {
		identity.LastUsedAt = *lastUsedAt
	}
	return nil
}
