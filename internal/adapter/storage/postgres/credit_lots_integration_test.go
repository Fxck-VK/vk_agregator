package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

func TestPostgresCreditLotsExpiryTerminalAndRollback(t *testing.T) {
	for _, capture := range []bool{false, true} {
		t.Run(map[bool]string{false: "release", true: "capture"}[capture], func(t *testing.T) {
			ctx := context.Background()
			pool := testPool(t)
			owner := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
				t.Fatal(err)
			}
			r := postgres.NewBillingRepository(pool)
			a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
			if err := r.CreateAccount(ctx, a); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour)
			e := &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 60, IdempotencyKey: uuid.NewString(), LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry}
			if err := r.AppendEntry(ctx, e); err != nil {
				t.Fatal(err)
			}
			jobID := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, jobID, owner, uuid.NewString()); err != nil {
				t.Fatal(err)
			}
			res := &domain.CreditReservation{AccountID: a.ID, JobID: jobID, Amount: 40, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
			if err := r.ReserveForOwner(ctx, owner, res); err != nil {
				t.Fatal(err)
			}
			if err := r.ReserveForOwner(ctx, owner, res); err != nil {
				t.Fatal(err)
			}
			lots, err := r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range lots {
				if l.Kind == domain.CreditLotSubscription && (l.AvailableCredits != 20 || l.ReservedCredits != 40) {
					t.Fatalf("FEFO %+v", l)
				}
			}
			if n, err := r.ExpireCreditLots(ctx, expiry.Add(time.Second), 10); err != nil || n != 1 {
				t.Fatalf("expire %d %v", n, err)
			}
			if _, err := pool.Exec(ctx, `UPDATE credit_lots SET expires_at=now()-interval '1 second' WHERE id=$1`, e.ID); err != nil {
				t.Fatal(err)
			}
			key := uuid.NewString()
			var terminal func() error
			if capture {
				terminal = func() error { return r.CaptureForOwner(ctx, owner, res.ID, 40, key) }
			} else {
				terminal = func() error { return r.ReleaseForOwner(ctx, owner, res.ID, key) }
			}
			if err := terminal(); err != nil {
				t.Fatal(err)
			}
			if err := terminal(); err != nil {
				t.Fatal(err)
			}
			got, _ := r.GetAccount(ctx, a.ID)
			if got.BalanceCached != 100 {
				t.Fatalf("preserved balance %d", got.BalanceCached)
			}
			assertPostgresLotProjection(t, ctx, pool, a.ID)
			sentinel := errors.New("outer rollback")
			err = postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				if err := postgres.NewBillingRepositoryTx(tx).AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerRefund, Amount: 30, IdempotencyKey: uuid.NewString()}); err != nil {
					return err
				}
				return sentinel
			})
			if !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
			got, _ = r.GetAccount(ctx, a.ID)
			if got.BalanceCached != 100 {
				t.Fatalf("rollback balance %d", got.BalanceCached)
			}
			assertPostgresLotProjection(t, ctx, pool, a.ID)
		})
	}
}

func TestPostgresCreditLotsSignedAdjustmentsConcurrentReservations(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
		t.Fatal(err)
	}
	r := postgres.NewBillingRepository(pool)
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := r.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	for _, amount := range []int64{-120, 45, -5, 30} {
		if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: amount, IdempotencyKey: uuid.NewString()}); err != nil {
			t.Fatal(err)
		}
		assertPostgresLotProjection(t, ctx, pool, a.ID)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobID := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, jobID, owner, uuid.NewString()); err != nil {
				t.Error(err)
				return
			}
			res := &domain.CreditReservation{AccountID: a.ID, JobID: jobID, Amount: 5, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
			err := r.ReserveForOwner(ctx, owner, res)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			} else if !errors.Is(err, domain.ErrInsufficientCredits) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if success != 10 {
		t.Fatalf("reservations %d want 10", success)
	}
	assertPostgresLotProjection(t, ctx, pool, a.ID)
}

func TestCreditLotMigrationPreservesLegacyBalancesAndHolds(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	schema := "lot_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := tx.Exec(ctx, `CREATE SCHEMA `+schema+`; SET LOCAL search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE credit_accounts(id UUID PRIMARY KEY,balance_cached BIGINT,created_at TIMESTAMPTZ DEFAULT now());CREATE TABLE credit_reservations(id UUID PRIMARY KEY,account_id UUID,amount BIGINT,status TEXT);CREATE TABLE ledger_entries(id UUID PRIMARY KEY);`); err != nil {
		t.Fatal(err)
	}
	for _, balance := range []int64{100, 10, -25} {
		account := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO credit_accounts(id,balance_cached) VALUES($1,$2)`, account, balance); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO credit_reservations(id,account_id,amount,status) VALUES($1,$2,40,'reserved')`, uuid.New(), account); err != nil {
			t.Fatal(err)
		}
	}
	root, _ := repoRoot()
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "000063_credit_lots.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range splitStatements(string(raw)) {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	var mismatches int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM credit_accounts a JOIN credit_lots l ON l.account_id=a.id WHERE a.balance_cached<>l.available_credits+l.reserved_credits OR l.reserved_credits<>40 OR l.expires_at IS NOT NULL`).Scan(&mismatches); err != nil {
		t.Fatal(err)
	}
	if mismatches != 0 {
		t.Fatalf("migration mismatches %d", mismatches)
	}
	var allocations int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM credit_reservation_lots WHERE amount=40`).Scan(&allocations); err != nil {
		t.Fatal(err)
	}
	if allocations != 3 {
		t.Fatalf("migration allocations %d", allocations)
	}
}

func assertPostgresLotProjection(t *testing.T, ctx context.Context, q postgres.Querier, account uuid.UUID) {
	t.Helper()
	var cached, lots, ledger int64
	err := q.QueryRow(ctx, `SELECT balance_cached,COALESCE((SELECT SUM(available_credits+reserved_credits) FROM credit_lots WHERE account_id=$1),0),COALESCE((SELECT SUM(amount) FROM ledger_entries WHERE account_id=$1 AND status='committed'),0) FROM credit_accounts WHERE id=$1`, account).Scan(&cached, &lots, &ledger)
	if err != nil {
		t.Fatal(err)
	}
	if cached != lots || cached != ledger {
		t.Fatalf("projection cached=%d lots=%d ledger=%d", cached, lots, ledger)
	}
}

func applyCreditLotTestSchema(ctx context.Context, q postgres.Querier) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "000063_credit_lots.up.sql"))
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(string(raw)) {
		if strings.HasPrefix(strings.TrimSpace(stmt), "LOCK TABLE") {
			continue
		}
		if _, err := q.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func TestCreditLotExpiryLocksCanonicalOwnerBeforeCreditAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool := testPool(t)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
		t.Fatal(err)
	}
	r := postgres.NewBillingRepository(pool)
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := r.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(-time.Second)
	if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 20, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expires, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, owner); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := r.ExpireCreditLots(ctx, time.Now(), 10); done <- err }()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		var count int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM accounts WHERE id=%FOR KEY SHARE'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			waiting = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("expiry did not wait for canonical owner before acquiring credit lock")
	}
	if err := postgres.NewBillingRepositoryTx(tx).AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 10, LotKind: domain.CreditLotPurchased, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, _ := r.GetAccount(ctx, a.ID)
	if got.BalanceCached != 110 {
		t.Fatalf("balance %d", got.BalanceCached)
	}
	assertPostgresLotProjection(t, ctx, pool, a.ID)
}

type creditLotGrantRevoker interface {
	RevokeCreditLotForGrant(context.Context, uuid.UUID, string, time.Time) error
}

func TestCreditLotRevocationAttributionAndTransactionRollback(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	r := postgres.NewBillingRepository(pool)
	revoker, ok := any(r).(creditLotGrantRevoker)
	if !ok {
		t.Fatal("missing exact grant-lot revocation")
	}
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
		t.Fatal(err)
	}
	a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := r.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	purchasedKey := uuid.NewString()
	if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 80, LotKind: domain.CreditLotPurchased, IdempotencyKey: purchasedKey}); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	key := uuid.NewString()
	grant := &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 60, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expiry, IdempotencyKey: key}
	if err := r.AppendEntry(ctx, grant); err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, jobID, owner, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	hold := &domain.CreditReservation{AccountID: a.ID, JobID: jobID, Amount: 40, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := r.ReserveForOwner(ctx, owner, hold); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Second)
	sentinel := errors.New("outer revoke rollback")
	err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		capability := any(postgres.NewBillingRepositoryTx(tx)).(creditLotGrantRevoker)
		if err := capability.RevokeCreditLotForGrant(ctx, owner, key, now); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	got, _ := r.GetAccount(ctx, a.ID)
	if got.BalanceCached != 240 {
		t.Fatalf("rollback balance %d", got.BalanceCached)
	}
	if err := revoker.RevokeCreditLotForGrant(ctx, uuid.New(), key, now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-owner %v", err)
	}
	if err := revoker.RevokeCreditLotForGrant(ctx, owner, purchasedKey, now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("purchased revoke %v", err)
	}
	if err := revoker.RevokeCreditLotForGrant(ctx, owner, key, now); err != nil {
		t.Fatal(err)
	}
	if err := revoker.RevokeCreditLotForGrant(ctx, owner, key, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetAccount(ctx, a.ID)
	if got.BalanceCached != 220 {
		t.Fatalf("revoke balance %d want220", got.BalanceCached)
	}
	assertPostgresLotProjection(t, ctx, pool, a.ID)
	if err := r.CaptureForOwner(ctx, owner, hold.ID, 40, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetAccount(ctx, a.ID)
	if got.BalanceCached != 180 {
		t.Fatalf("late capture balance %d want180", got.BalanceCached)
	}
	assertPostgresLotProjection(t, ctx, pool, a.ID)
}

func TestCreditLotGenericDebitExpiresUnsweptLotsBeforeConsumption(t *testing.T) {
	for _, debit := range []int64{100, 1000} {
		t.Run(fmt.Sprint(debit), func(t *testing.T) {
			ctx := context.Background()
			pool := testPool(t)
			r := postgres.NewBillingRepository(pool)
			owner := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
				t.Fatal(err)
			}
			a := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits}
			if err := r.CreateAccount(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 1000, LotKind: domain.CreditLotPurchased, IdempotencyKey: uuid.NewString()}); err != nil {
				t.Fatal(err)
			}
			expired := time.Now().Add(-time.Hour)
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 400, LotKind: domain.CreditLotSubscription, LotExpiresAt: &expired, IdempotencyKey: uuid.NewString()}); err != nil {
				t.Fatal(err)
			}
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: -debit, IdempotencyKey: uuid.NewString(), Reason: "payment refund debit"}); err != nil {
				t.Fatal(err)
			}
			lots, err := r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			var spendable int64
			for _, lot := range lots {
				spendable += lot.AvailableCredits
			}
			account, _ := r.GetAccount(ctx, a.ID)
			if spendable != 1000-debit || account.BalanceCached != 1000-debit {
				t.Fatalf("debit consumed expired credits: spendable=%d cached=%d want=%d", spendable, account.BalanceCached, 1000-debit)
			}
			assertPostgresLotProjection(t, ctx, pool, a.ID)
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerAdjustment, Amount: debit, IdempotencyKey: uuid.NewString(), Reason: "payment refund provider failure compensation"}); err != nil {
				t.Fatal(err)
			}
			lots, err = r.CreditLotBalancesForOwner(ctx, owner, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			spendable = 0
			for _, lot := range lots {
				spendable += lot.AvailableCredits
				if lot.Kind == domain.CreditLotSubscription && lot.AvailableCredits != 0 {
					t.Fatal("compensation resurrected expired subscription")
				}
			}
			if spendable != 1000 {
				t.Fatalf("compensation restored expired credits: %d want1000", spendable)
			}
			assertPostgresLotProjection(t, ctx, pool, a.ID)
			if n, err := r.ExpireCreditLots(ctx, time.Now(), 10); err != nil || n != 0 {
				t.Fatalf("generic debit did not persist exact expiry: %d/%v", n, err)
			}
		})
	}
}

func TestCreditLotSignupOpeningBonusExactlyOnce(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := postgres.NewBillingRepository(pool)
	svc := billingservice.New(repo)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, owner); err != nil {
		t.Fatal(err)
	}
	a, err := svc.EnsureAccountForAccount(ctx, repo, owner, domain.CurrencyCredits)
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.EnsureAccountForAccount(ctx, repo, owner, domain.CurrencyCredits)
	if err != nil || again.ID != a.ID {
		t.Fatalf("opening account replay differs: %v", err)
	}
	lots, err := repo.CreditLotBalancesForOwner(ctx, owner, time.Now())
	if err != nil || len(lots) != 1 || lots[0].Kind != domain.CreditLotBonus || lots[0].AvailableCredits != 60 || lots[0].ExpiresAt != nil {
		t.Fatalf("persisted opening bonus differs: %v/%v", lots, err)
	}
	entries, err := repo.ListEntries(ctx, a.ID, 10, 0)
	if err != nil || len(entries) != 1 || entries[0].LotKind != domain.CreditLotBonus || entries[0].Amount != 60 {
		t.Fatalf("persisted grant metadata/count differs: %d/%v", len(entries), err)
	}
	assertPostgresLotProjection(t, ctx, pool, a.ID)
	legacyOwner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, legacyOwner); err != nil {
		t.Fatal(err)
	}
	legacy := &domain.CreditAccount{OwnerAccountID: legacyOwner, Currency: domain.CurrencyCredits, BalanceCached: 60}
	if err := repo.CreateAccount(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	lots, err = repo.CreditLotBalancesForOwner(ctx, legacyOwner, time.Now())
	if err != nil || len(lots) != 1 || lots[0].Kind != domain.CreditLotLegacy {
		t.Fatalf("legacy direct account reclassified: %v/%v", lots, err)
	}
}
