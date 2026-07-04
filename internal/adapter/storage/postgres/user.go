package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"vk-ai-aggregator/internal/domain"
)

// UserRepository is the PostgreSQL implementation of domain.UserRepository.
type UserRepository struct {
	db Querier
}

// NewUserRepository builds a UserRepository over the given querier.
func NewUserRepository(db Querier) *UserRepository {
	return &UserRepository{db: db}
}

var _ domain.UserRepository = (*UserRepository)(nil)

const userColumns = `id, account_id, vk_user_id, role, status, locale, timezone, risk_level,
	vk_first_name, vk_last_name, vk_profile_synced_at, welcome_name_sent_at,
	first_seen_at, last_seen_at, created_at, updated_at`

// Create inserts a new user, letting the database fill id and timestamps when
// they are zero so callers may pre-set them or rely on defaults.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	const q = `
		INSERT INTO users (
			id, account_id, vk_user_id, role, status, locale, timezone, risk_level,
			vk_first_name, vk_last_name, vk_profile_synced_at, welcome_name_sent_at,
			first_seen_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, now()), COALESCE($14, now()))
		RETURNING ` + userColumns
	row := r.db.QueryRow(ctx, q,
		user.ID, nullableUUID(user.AccountID), user.VKUserID, user.Role, user.Status, user.Locale, user.Timezone, user.RiskLevel,
		user.VKFirstName, user.VKLastName, nullableTime(user.VKProfileSyncedAt), nullableTime(user.WelcomeNameSentAt),
		nullableTime(user.FirstSeenAt), nullableTime(user.LastSeenAt),
	)
	return mapError(scanUser(row, user))
}

// Update persists mutable fields of an existing user and refreshes updated_at.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	const q = `
		UPDATE users
		SET role = $2, status = $3, locale = $4, timezone = $5, risk_level = $6,
		    vk_first_name = $7, vk_last_name = $8, vk_profile_synced_at = $9,
		    welcome_name_sent_at = $10, last_seen_at = $11, updated_at = now()
		WHERE id = $1
		RETURNING ` + userColumns
	row := r.db.QueryRow(ctx, q,
		user.ID, user.Role, user.Status, user.Locale, user.Timezone, user.RiskLevel,
		user.VKFirstName, user.VKLastName, nullableTime(user.VKProfileSyncedAt),
		nullableTime(user.WelcomeNameSentAt), user.LastSeenAt,
	)
	return mapError(scanUser(row, user))
}

// GetByID fetches a user by internal id.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	var user domain.User
	if err := mapError(scanUser(r.db.QueryRow(ctx, q, id), &user)); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByVKUserID fetches a user by external VK id.
func (r *UserRepository) GetByVKUserID(ctx context.Context, vkUserID int64) (*domain.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE vk_user_id = $1`
	var user domain.User
	if err := mapError(scanUser(r.db.QueryRow(ctx, q, vkUserID), &user)); err != nil {
		return nil, err
	}
	return &user, nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner, user *domain.User) error {
	var vkProfileSyncedAt, welcomeNameSentAt *time.Time
	var accountID pgtype.UUID
	if err := row.Scan(
		&user.ID, &accountID, &user.VKUserID, &user.Role, &user.Status, &user.Locale, &user.Timezone,
		&user.RiskLevel, &user.VKFirstName, &user.VKLastName, &vkProfileSyncedAt,
		&welcomeNameSentAt, &user.FirstSeenAt, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		return err
	}
	if accountID.Valid {
		user.AccountID = uuid.UUID(accountID.Bytes)
	} else {
		user.AccountID = uuid.Nil
	}
	if vkProfileSyncedAt != nil {
		user.VKProfileSyncedAt = *vkProfileSyncedAt
	}
	if welcomeNameSentAt != nil {
		user.WelcomeNameSentAt = *welcomeNameSentAt
	}
	return nil
}
