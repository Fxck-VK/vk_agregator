package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type maintenanceLotExpirer interface {
	ExpireCreditLots(context.Context, time.Time, int) (int64, error)
}
type maintenanceExpiryTx struct {
	pgx.Tx
	nested                     *maintenanceExpiryTx
	begins, queries, rollbacks int
	err                        error
}

func (q *maintenanceExpiryTx) Begin(context.Context) (pgx.Tx, error) {
	q.begins++
	return q.nested, nil
}
func (q *maintenanceExpiryTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	q.queries++
	return nil, q.err
}
func (q *maintenanceExpiryTx) Rollback(context.Context) error { q.rollbacks++; return nil }

func TestMaintenanceCreditLotExpiryUsesTransactionBoundBilling(t *testing.T) {
	sentinel := errors.New("query sentinel")
	nested := &maintenanceExpiryTx{err: sentinel}
	outer := &maintenanceExpiryTx{nested: nested}
	capability, ok := any(NewMaintenanceRepository(outer)).(maintenanceLotExpirer)
	if !ok {
		t.Fatal("maintenance repository lacks credit lot expiry capability")
	}
	if _, err := capability.ExpireCreditLots(context.Background(), time.Now(), 10); !errors.Is(err, sentinel) {
		t.Fatalf("expiry error %v", err)
	}
	if outer.begins != 1 || nested.queries != 1 || nested.rollbacks != 1 {
		t.Fatalf("transaction delegation begin=%d query=%d rollback=%d", outer.begins, nested.queries, nested.rollbacks)
	}
}
