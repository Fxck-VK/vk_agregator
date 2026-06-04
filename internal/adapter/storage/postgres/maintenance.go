package postgres

import (
	"context"
	"time"

	"vk-ai-aggregator/internal/domain"
)

// MaintenanceRepository contains operational cleanup and audit queries that do
// not belong to business repositories.
type MaintenanceRepository struct {
	db Querier
}

// NewMaintenanceRepository builds a MaintenanceRepository over db.
func NewMaintenanceRepository(db Querier) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// CleanupExpiredIdempotencyKeys deletes expired idempotency records.
func (r *MaintenanceRepository) CleanupExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// CleanupOutboxEvents deletes terminal outbox events older than cutoff.
func (r *MaintenanceRepository) CleanupOutboxEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM outbox_events
		WHERE status IN ($1, $2) AND created_at <= $3`,
		domain.OutboxPublished, domain.OutboxFailed, cutoff,
	)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// BalanceMismatches returns accounts whose cached balance differs from the sum
// of committed ledger entries.
func (r *MaintenanceRepository) BalanceMismatches(ctx context.Context, limit int) ([]domain.BalanceMismatch, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT c.id, c.user_id, c.currency, c.balance_cached,
		       COALESCE(SUM(l.amount) FILTER (WHERE l.status = $1), 0)::bigint AS ledger_balance
		FROM credit_accounts c
		LEFT JOIN ledger_entries l ON l.account_id = c.id
		GROUP BY c.id, c.user_id, c.currency, c.balance_cached
		HAVING c.balance_cached <> COALESCE(SUM(l.amount) FILTER (WHERE l.status = $1), 0)
		ORDER BY c.updated_at ASC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, domain.LedgerStatusCommitted, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []domain.BalanceMismatch
	for rows.Next() {
		var m domain.BalanceMismatch
		if err := rows.Scan(&m.AccountID, &m.UserID, &m.Currency, &m.BalanceCached, &m.LedgerBalance); err != nil {
			return nil, mapError(err)
		}
		m.Difference = m.BalanceCached - m.LedgerBalance
		out = append(out, m)
	}
	return out, mapError(rows.Err())
}
