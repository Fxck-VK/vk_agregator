package postgres

import (
	"context"

	"github.com/google/uuid"

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

const userColumns = `id, vk_user_id, role, status, locale, timezone, risk_level,
	first_seen_at, last_seen_at, created_at, updated_at`

// Create inserts a new user, letting the database fill id and timestamps when
// they are zero so callers may pre-set them or rely on defaults.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	const q = `
		INSERT INTO users (id, vk_user_id, role, status, locale, timezone, risk_level, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()), COALESCE($9, now()))
		RETURNING ` + userColumns
	row := r.db.QueryRow(ctx, q,
		user.ID, user.VKUserID, user.Role, user.Status, user.Locale, user.Timezone, user.RiskLevel,
		nullableTime(user.FirstSeenAt), nullableTime(user.LastSeenAt),
	)
	return mapError(scanUser(row, user))
}

// Update persists mutable fields of an existing user and refreshes updated_at.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	const q = `
		UPDATE users
		SET role = $2, status = $3, locale = $4, timezone = $5, risk_level = $6,
		    last_seen_at = $7, updated_at = now()
		WHERE id = $1
		RETURNING ` + userColumns
	row := r.db.QueryRow(ctx, q,
		user.ID, user.Role, user.Status, user.Locale, user.Timezone, user.RiskLevel, user.LastSeenAt,
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
	return row.Scan(
		&user.ID, &user.VKUserID, &user.Role, &user.Status, &user.Locale, &user.Timezone,
		&user.RiskLevel, &user.FirstSeenAt, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt,
	)
}
