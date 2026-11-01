package accounthistory_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accounthistory"
)

type historyRepo struct {
	rows  []domain.AccountHistoryRow
	owner uuid.UUID
	limit int
	after *domain.AccountHistoryCursor
	err   error
}

func (r *historyRepo) ListAccountHistory(_ context.Context, owner uuid.UUID, limit int, after *domain.AccountHistoryCursor) ([]domain.AccountHistoryRow, error) {
	r.owner = owner
	r.limit = limit
	r.after = after
	return r.rows, r.err
}
func historyRow() domain.AccountHistoryRow {
	return domain.AccountHistoryRow{ID: uuid.New(), Source: domain.AccountHistoryLedger, Kind: "generation", CreatedAt: time.Now().UTC(), StarsDelta: -10, CreditDenominationVersion: 1, Status: "committed", Operation: "image_generate", Modality: "image"}
}
func TestAccountHistoryConvertsSafeRowsAndBindsCursor(t *testing.T) {
	owner := uuid.New()
	a, b := historyRow(), historyRow()
	r := &historyRepo{rows: []domain.AccountHistoryRow{a, b}}
	s := accounthistory.New(r)
	page, err := s.List(context.Background(), owner, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.owner != owner || r.limit != 2 || len(page.Items) != 1 || page.Items[0].StarsDelta != -20 || page.NextCursor == nil {
		t.Fatalf("bad page %+v", page)
	}
	if _, err = s.List(context.Background(), uuid.New(), 1, *page.NextCursor); !errors.Is(err, accounthistory.ErrInvalidQuery) {
		t.Fatalf("foreign cursor accepted: %v", err)
	}
	if _, err = s.List(context.Background(), owner, 1, *page.NextCursor); err != nil || r.after == nil || r.after.ID != a.ID || !r.after.CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("cursor not applied %v %+v", err, r.after)
	}
	data, _ := json.Marshal(page)
	for _, forbidden := range []string{"account_id", "owner_id", "reason", "idempotency", "provider", "denomination"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("private field %s", forbidden)
		}
	}
}
func TestAccountHistoryEmptyBoundsAndFailure(t *testing.T) {
	r := &historyRepo{}
	s := accounthistory.New(r)
	owner := uuid.New()
	page, err := s.List(context.Background(), owner, 0, "")
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != nil || r.limit != 21 {
		t.Fatalf("empty/default %+v %v", page, err)
	}
	for _, limit := range []int{-1, 51, 1000000} {
		if _, err = s.List(context.Background(), owner, limit, ""); !errors.Is(err, accounthistory.ErrInvalidQuery) {
			t.Fatalf("bad limit accepted %d %v", limit, err)
		}
	}
	for _, cursor := range []string{"invalid", strings.Repeat("x", 1000), "e30"} {
		if _, err = s.List(context.Background(), owner, 20, cursor); !errors.Is(err, accounthistory.ErrInvalidQuery) {
			t.Fatalf("bad cursor accepted %v", err)
		}
	}
	if _, err = s.List(context.Background(), uuid.Nil, 20, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	r.err = errors.New("unavailable")
	if _, err = s.List(context.Background(), owner, 20, ""); err == nil {
		t.Fatal("storage failure hidden")
	}
}
func TestAccountHistoryNormalizesPaymentsAndRejectsCorruptRows(t *testing.T) {
	r := &historyRepo{}
	s := accounthistory.New(r)
	owner := uuid.New()
	amount := int64(54900)
	row := historyRow()
	row.Source = domain.AccountHistoryPayment
	row.Kind = "subscription"
	row.StarsDelta = 0
	row.Status = "waiting_for_user"
	row.AmountMinor = &amount
	row.Currency = domain.CurrencyRUB
	row.PlanCode = "start"
	r.rows = []domain.AccountHistoryRow{row}
	page, err := s.List(context.Background(), owner, 20, "")
	if err != nil || page.Items[0].Status != "pending" || page.Items[0].StarsDelta != 0 || page.Items[0].Money == nil || page.Items[0].Money.AmountMinor != 54900 {
		t.Fatalf("pending purchase %+v %v", page, err)
	}
	row.Status = "refunded"
	r.rows = []domain.AccountHistoryRow{row}
	page, err = s.List(context.Background(), owner, 20, "")
	if err != nil || page.Items[0].Status != "refunded" {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.AccountHistoryRow){func(r *domain.AccountHistoryRow) { r.Kind = "private provider reason" }, func(r *domain.AccountHistoryRow) { r.Source = 99 }, func(r *domain.AccountHistoryRow) { r.StarsDelta = 1 << 54 }, func(r *domain.AccountHistoryRow) { r.CreatedAt = time.Time{} }, func(r *domain.AccountHistoryRow) { r.Currency = "usd" }, func(r *domain.AccountHistoryRow) { r.CreditDenominationVersion = 99 }} {
		bad := row
		mutate(&bad)
		r.rows = []domain.AccountHistoryRow{bad}
		if _, err = s.List(context.Background(), owner, 20, ""); err == nil {
			t.Fatal("corrupt row accepted")
		}
	}
}
