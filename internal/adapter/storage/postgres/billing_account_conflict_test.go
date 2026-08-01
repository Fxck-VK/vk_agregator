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

func TestCreateNativeAccountUsesNonAbortingOwnerCurrencyConflictTarget(t *testing.T) {
	recorder := &nativeAccountConflictRecorder{}
	err := NewBillingRepositoryTx(recorder).CreateAccount(context.Background(), &domain.CreditAccount{
		OwnerAccountID: uuid.New(),
		Currency:       domain.CurrencyCredits,
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("native owner/currency conflict error = %v, want ErrConflict", err)
	}
	if !strings.Contains(recorder.query, "ON CONFLICT (owner_account_id, currency) WHERE owner_account_id IS NOT NULL DO NOTHING") {
		t.Fatalf("native account insert must use the exact partial owner/currency conflict target: %s", recorder.query)
	}
}

type nativeAccountConflictRecorder struct {
	query string
}

func (r *nativeAccountConflictRecorder) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (r *nativeAccountConflictRecorder) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (r *nativeAccountConflictRecorder) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	r.query = query
	return nativeAccountConflictRow{}
}

type nativeAccountConflictRow struct{}

func (nativeAccountConflictRow) Scan(...any) error { return pgx.ErrNoRows }
