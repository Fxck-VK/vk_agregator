package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPreparedWebImageExpiryReconciliationMigrationUsesNarrowConcurrentIndex(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "000049_web_image_prepared_expiry_reconciliation.up.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read reconciliation migration: %v", err)
	}
	migration := string(raw)
	for _, clause := range []string{
		"-- migrate: no-transaction",
		"DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_prepared_expiry_reconciliation_idx",
		"CREATE INDEX CONCURRENTLY jobs_web_image_prepared_expiry_reconciliation_idx",
		"ON jobs (expires_at ASC, id ASC)",
		"account_id IS NOT NULL",
		"source = 'web'",
		"operation_type = 'image_generate'",
		"modality = 'image'",
		"status = 'prepared'",
		"expires_at IS NOT NULL",
	} {
		if !strings.Contains(migration, clause) {
			t.Fatalf("reconciliation migration missing %q: %s", clause, migration)
		}
	}
}

func TestPreparedWebImageExpiryRepositoryUsesBoundedSkipLockedClaims(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	recorder := &preparedExpiryQueryRecorder{}
	repository := NewPreparedWebImageExpiryRepository(recorder)

	expired, hasMore, err := repository.ExpireDuePreparedWebImages(context.Background(), nil, now, 100)
	if err != nil {
		t.Fatalf("expire global page: %v", err)
	}
	if expired != 0 || hasMore {
		t.Fatalf("empty global page = %d/%t", expired, hasMore)
	}
	for _, clause := range []string{
		"UPDATE jobs",
		"account_id IS NOT NULL",
		"source = 'web'",
		"operation_type = 'image_generate'",
		"modality = 'image'",
		"status = 'prepared'",
		"expires_at IS NOT NULL",
		"FOR UPDATE SKIP LOCKED",
		"ORDER BY expires_at ASC, id ASC",
		"RETURNING j.id",
	} {
		if !strings.Contains(recorder.query, clause) {
			t.Fatalf("global expiry query missing %q: %s", clause, recorder.query)
		}
	}
	if !containsArg(recorder.args, now) || !containsArg(recorder.args, 101) {
		t.Fatalf("global expiry args must include now and limit+1: %#v", recorder.args)
	}
}

func TestPreparedWebImageExpiryRepositoryScopesAccountAndTargetedClaimsExactly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	accountID := uuid.New()
	jobID := uuid.New()

	t.Run("account page", func(t *testing.T) {
		recorder := &preparedExpiryQueryRecorder{}
		_, _, err := NewPreparedWebImageExpiryRepository(recorder).ExpireDuePreparedWebImages(ctx, &accountID, now, 10)
		if err != nil {
			t.Fatalf("expire account page: %v", err)
		}
		if !strings.Contains(recorder.query, "account_id = $1") || !containsArg(recorder.args, accountID) {
			t.Fatalf("account expiry query must use exact owner: %s / %#v", recorder.query, recorder.args)
		}
		if strings.Contains(recorder.query, "user_id") {
			t.Fatalf("account expiry query must not fall back to legacy user: %s", recorder.query)
		}
	})

	t.Run("exact job", func(t *testing.T) {
		recorder := &preparedExpiryQueryRecorder{}
		changed, err := NewPreparedWebImageExpiryRepository(recorder).ExpireDuePreparedWebImage(ctx, accountID, jobID, now)
		if err != nil {
			t.Fatalf("expire exact job: %v", err)
		}
		if changed {
			t.Fatal("empty exact update must not report a change")
		}
		for _, clause := range []string{"UPDATE jobs", "id = $1", "account_id = $2", "status = 'prepared'", "RETURNING id"} {
			if !strings.Contains(recorder.query, clause) {
				t.Fatalf("targeted expiry query missing %q: %s", clause, recorder.query)
			}
		}
		if len(recorder.args) < 3 || recorder.args[0] != jobID || recorder.args[1] != accountID || !containsArg(recorder.args, now) {
			t.Fatalf("targeted expiry args = %#v", recorder.args)
		}
	})
}

type preparedExpiryQueryRecorder struct {
	query string
	args  []any
}

func (r *preparedExpiryQueryRecorder) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (r *preparedExpiryQueryRecorder) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	r.query = query
	r.args = args
	return preparedExpiryEmptyRows{}, nil
}

func (r *preparedExpiryQueryRecorder) QueryRow(context.Context, string, ...any) pgx.Row {
	return scopedNoRows{}
}

type preparedExpiryEmptyRows struct{}

func (preparedExpiryEmptyRows) Close()                                       {}
func (preparedExpiryEmptyRows) Err() error                                   { return nil }
func (preparedExpiryEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (preparedExpiryEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (preparedExpiryEmptyRows) Next() bool                                   { return false }
func (preparedExpiryEmptyRows) Scan(...any) error                            { return nil }
func (preparedExpiryEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (preparedExpiryEmptyRows) RawValues() [][]byte                          { return nil }
func (preparedExpiryEmptyRows) Conn() *pgx.Conn                              { return nil }

func containsArg(args []any, want any) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
