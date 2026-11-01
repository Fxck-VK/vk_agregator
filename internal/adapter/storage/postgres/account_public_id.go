package postgres

import (
	"context"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.AccountPublicIDRepository = (*AccountIdentityRepository)(nil)

func (r *AccountIdentityRepository) GetAccountPublicNumber(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var number int64
	err := r.db.QueryRow(ctx, `SELECT public_number FROM accounts WHERE id=$1`, accountID).Scan(&number)
	return number, mapError(err)
}

func (r *AccountIdentityRepository) GetAccountByPublicNumber(ctx context.Context, number int64) (*domain.Account, error) {
	if number <= 0 {
		return nil, domain.ErrNotFound
	}
	var account domain.Account
	err := r.db.QueryRow(ctx, `SELECT id,status,role,account_type,locale,timezone,risk_level,created_at,updated_at
	 FROM accounts WHERE public_number=$1`, number).Scan(&account.ID, &account.Status, &account.Role, &account.AccountType, &account.Locale, &account.Timezone, &account.RiskLevel, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &account, nil
}
