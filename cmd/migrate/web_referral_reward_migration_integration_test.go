package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWebReferralRewardMigrationRetainsHistoricalProofAndRejectsDown(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	dir := t.TempDir()
	if err := ensureTable(ctx, pool); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		name := filepath.Base(path)
		if name[:6] > "000065" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = up(ctx, pool, dir); err != nil {
		t.Fatal(err)
	}
	owner, intent, wallet, entry := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO accounts(id) VALUES($1)`, []any{owner}},
		{`INSERT INTO credit_accounts(id,owner_account_id,currency,balance_cached) VALUES($1,$2,'credits',100)`, []any{wallet, owner}},
		{`INSERT INTO payment_intents(id,account_id,status,amount,currency,credits,price_version,provider,provider_payment_id,idempotency_key,receipt_email) VALUES($1,$2,'succeeded',19900,'rub',100,1,'yookassa',$3,$3,'test@example.invalid')`, []any{intent, owner, intent.String()}},
		{`INSERT INTO ledger_entries(id,account_id,owner_account_id,type,amount,status,idempotency_key) VALUES($1,$2,$3,'topup',100,'committed',$4)`, []any{entry, wallet, owner, "topup:yookassa:" + intent.String()}},
	} {
		if _, err = pool.Exec(ctx, query.sql, query.args...); err != nil {
			t.Fatal(err)
		}
	}
	const version = "000066_web_referral_rewards"
	for _, suffix := range []string{"up", "down"} {
		name := version + "." + suffix + ".sql"
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = up(ctx, pool, dir); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err = pool.QueryRow(ctx, `SELECT mode FROM web_referral_payment_evidence WHERE intent_id=$1`, intent).Scan(&mode); err != nil || mode != "historical" {
		t.Fatalf("history %s %v", mode, err)
	}
	if err = down(ctx, pool, dir); err == nil || !strings.Contains(err.Error(), "irreversible") {
		t.Fatalf("destructive rollback not rejected: %v", err)
	}
	var recorded bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&recorded); err != nil || !recorded {
		t.Fatal("migration lost", err)
	}
	if err = up(ctx, pool, dir); err != nil {
		t.Fatal("resume", err)
	}
}
