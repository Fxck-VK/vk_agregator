package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"vk-ai-aggregator/internal/domain"
)

type scopedQueryRecorder struct {
	query string
	args  []any
}

func (r *scopedQueryRecorder) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (r *scopedQueryRecorder) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	r.query = query
	r.args = args
	return nil, pgx.ErrNoRows
}

func (r *scopedQueryRecorder) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	r.query = query
	r.args = args
	return scopedNoRows{}
}

type scopedNoRows struct{}

func (scopedNoRows) Scan(...any) error { return pgx.ErrNoRows }

func TestAccountNativePreparedRepositoryQueriesUseExactCanonicalOwners(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	id := uuid.New()

	t.Run("job by id", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewJobRepository(recorder).GetByIDForAccount(ctx, accountID, id)
		assertStrictOwnerQuery(t, err, recorder, "account_id = $2")
	})
	t.Run("job key", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewJobRepository(recorder).GetByIdempotencyKeyForAccount(ctx, accountID, "prepared-key")
		assertStrictOwnerQuery(t, err, recorder, "account_id = $2")
	})
	t.Run("job list", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewJobRepository(recorder).ListByAccount(ctx, accountID, 10, 0)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("list error = %v, want ErrNotFound", err)
		}
		if !strings.Contains(recorder.query, "WHERE account_id = $1") || strings.Contains(recorder.query, "OR user_id") {
			t.Fatalf("list query must scope exact canonical owner: %s", recorder.query)
		}
	})
	t.Run("active capacity", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewJobRepository(recorder).CountActiveByAccountOperation(ctx, accountID, domain.OperationVideoGenerate)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("active capacity error = %v, want ErrNotFound", err)
		}
		if !strings.Contains(recorder.query, "account_id = $1") || strings.Contains(recorder.query, "user_id") {
			t.Fatalf("active capacity query must use exact canonical account only: %s", recorder.query)
		}
	})
	t.Run("artifact by id", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewArtifactRepository(recorder).GetByIDForAccount(ctx, accountID, id)
		assertStrictOwnerQuery(t, err, recorder, "owner_account_id = $2")
	})
	t.Run("artifact hash", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewArtifactRepository(recorder).GetBySHA256ForAccount(ctx, accountID, "sha")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("hash error = %v, want ErrNotFound", err)
		}
		if !strings.Contains(recorder.query, "owner_account_id = $1") || strings.Contains(recorder.query, "OR owner_user_id") {
			t.Fatalf("hash query must be exact account owner only: %s", recorder.query)
		}
	})
	t.Run("artifact reuse", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewArtifactRepository(recorder).FindReusableInputReferenceForAccount(ctx, accountID, "sha", "policy", "image/png")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("reuse error = %v, want ErrNotFound", err)
		}
		if !strings.Contains(recorder.query, "WHERE owner_account_id = $1") || strings.Contains(recorder.query, "OR owner_user_id") || strings.Contains(recorder.query, "COALESCE(owner_account_id") {
			t.Fatalf("reuse query must be exact account owner only: %s", recorder.query)
		}
	})
}

func TestJobRepositoryListCursorScopesWebImageHistoryBySource(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	recorder := &scopedQueryRecorder{}

	_, err := NewJobRepository(recorder).ListCursor(ctx, domain.JobFilter{
		AccountID: &accountID,
		Source:    "web",
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
	}, 21, nil)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("list cursor error = %v, want ErrNotFound", err)
	}
	for _, clause := range []string{
		"account_id = $1",
		"source = $2",
		"operation_type = $3",
		"modality = $4",
		"ORDER BY created_at DESC, id DESC LIMIT $5",
	} {
		if !strings.Contains(recorder.query, clause) {
			t.Fatalf("query missing %q: %s", clause, recorder.query)
		}
	}
	if len(recorder.args) != 5 || recorder.args[0] != accountID || recorder.args[1] != "web" || recorder.args[2] != domain.OperationImageGenerate || recorder.args[3] != domain.ModalityImage || recorder.args[4] != 21 {
		t.Fatalf("cursor args = %#v", recorder.args)
	}
}

func assertStrictOwnerQuery(t *testing.T, err error, recorder *scopedQueryRecorder, ownerClause string) {
	t.Helper()
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("query error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(recorder.query, ownerClause) || strings.Contains(recorder.query, "OR owner_user_id") || strings.Contains(recorder.query, "COALESCE(account_id") {
		t.Fatalf("query must scope exact canonical owner: %s", recorder.query)
	}
}
