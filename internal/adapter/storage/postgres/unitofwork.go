package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/platform/uow"
)

// UnitOfWork is the PostgreSQL implementation of uow.Manager. Each call opens a
// transaction and builds repositories bound to it, so all writes commit or roll
// back together.
type UnitOfWork struct {
	pool *pgxpool.Pool
}

// NewUnitOfWork builds a UnitOfWork over the given pool.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

var _ uow.Manager = (*UnitOfWork)(nil)

// Within runs fn inside a single transaction with transaction-scoped
// repositories.
func (u *UnitOfWork) Within(ctx context.Context, fn func(ctx context.Context, repos uow.Repositories) error) error {
	return RunInTx(ctx, u.pool, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, uow.Repositories{
			Jobs:   NewJobRepository(tx),
			Outbox: NewOutboxRepository(tx),
		})
	})
}
