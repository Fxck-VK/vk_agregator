// Package postgres provides PostgreSQL implementations of the domain
// repository interfaces, built on top of jackc/pgx v5.
//
// Every repository accepts a Querier, which is satisfied by both *pgxpool.Pool
// and pgx.Tx. This lets the same repository code run either on its own
// connection pool or inside a caller-managed transaction (for example, to
// write an outbox event in the same transaction as the state change that
// produced it).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/domain"
)

// Querier is the subset of pgx methods shared by *pgxpool.Pool and pgx.Tx. All
// repositories depend on this interface rather than a concrete connection so
// they can transparently participate in transactions.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pgUniqueViolation is the PostgreSQL SQLSTATE for a unique_violation.
const pgUniqueViolation = "23505"

// NewPool opens a pgx connection pool against dsn and verifies connectivity.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// mapError normalizes a raw pgx error into a domain-level error. It translates
// "no rows" into domain.ErrNotFound and unique violations into
// domain.ErrConflict, leaving other errors wrapped for context.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.ErrConflict
	}
	return err
}

// nullableTime returns nil for a zero time so the database default (now()) can
// apply via COALESCE, otherwise it returns a pointer to the value.
func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// rawOrNil returns nil for an empty JSON payload so the column is stored as
// SQL NULL rather than an invalid empty JSONB value.
func rawOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

// uuidArray coerces a nil UUID slice into a non-nil empty slice. pgx encodes a
// nil slice as SQL NULL, which violates the NOT NULL UUID[] columns; an empty
// non-nil slice encodes as the empty array '{}' the schema expects.
func uuidArray(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// TxFunc is a unit of work executed inside a database transaction.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// RunInTx runs fn inside a transaction, committing on success and rolling back
// on error or panic. It is the entry point for multi-repository atomic work.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}
