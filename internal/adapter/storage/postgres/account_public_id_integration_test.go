package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestPostgresAccountPublicIDBackfillPersistenceAndUniqueness(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	_, err := pool.Exec(ctx, `ALTER TABLE accounts
	 ADD COLUMN status text NOT NULL DEFAULT 'active', ADD COLUMN role text NOT NULL DEFAULT 'user',
	 ADD COLUMN account_type text NOT NULL DEFAULT 'personal', ADD COLUMN locale text NOT NULL DEFAULT 'ru',
	 ADD COLUMN timezone text NOT NULL DEFAULT 'Europe/Moscow', ADD COLUMN risk_level integer NOT NULL DEFAULT 0,
	 ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(), ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now()`)
	if err != nil {
		t.Fatal(err)
	}
	oldA, oldB := uuid.New(), uuid.New()
	if _, err = pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1),($2)", oldA, oldB); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../../migrations/000065_account_public_id.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool, postgres.WithUsableLoginProviders(domain.UsableLoginProviders{domain.IdentityProviderEmail: true, domain.IdentityProviderYandex: true}))
	a, err := repo.GetAccountPublicNumber(ctx, oldA)
	if err != nil || a <= 0 {
		t.Fatalf("backfill missing: %d/%v", a, err)
	}
	b, err := repo.GetAccountPublicNumber(ctx, oldB)
	if err != nil || b <= 0 || b == a {
		t.Fatalf("backfill collision: %d/%v", b, err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO accounts(id,public_number) VALUES($1,$2)", uuid.New(), a+1000); err == nil {
		t.Fatal("explicit public number bypassed server assignment")
	}
	for _, value := range []int64{a, 0, -1} {
		if _, err = pool.Exec(ctx, "INSERT INTO accounts(id,public_number) OVERRIDING SYSTEM VALUE VALUES($1,$2)", uuid.New(), value); err == nil {
			t.Fatalf("duplicate or nonpositive number accepted: %d", value)
		}
	}
	account, err := repo.GetAccountByPublicNumber(ctx, a)
	if err != nil || account.ID != oldA || account.Status != "active" {
		t.Fatalf("lookup mismatch: %v", err)
	}
	if _, err = repo.GetAccountPublicNumber(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unknown account assigned a number", err)
	}
	if _, err = repo.GetAccountByPublicNumber(ctx, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("invalid number accepted", err)
	}
	// Existing identity changes must never allocate or rotate the account number.
	first, err := repo.LinkIdentity(ctx, oldA, domain.IdentityProviderEmail, "first@example.test", "first@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.LinkIdentity(ctx, oldA, domain.IdentityProviderYandex, "synthetic-yandex", "synthetic-yandex"); err != nil {
		t.Fatal(err)
	}
	if err = repo.UnlinkIdentity(ctx, oldA, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE accounts SET locale='ru', updated_at=now() WHERE id=$1", oldA); err != nil {
		t.Fatal(err)
	}
	// This trigger is defense in depth against accidental account updates.
	if _, err = pool.Exec(ctx, "UPDATE accounts SET public_number=public_number+100 WHERE id=$1", oldA); err == nil {
		t.Fatal("permanent number was mutable")
	}
	if _, err = pool.Exec(ctx, "UPDATE accounts SET public_number=DEFAULT WHERE id=$1", oldA); err == nil {
		t.Fatal("identity default rotated permanent number")
	}
	if got, err := postgres.NewAccountIdentityRepository(pool).GetAccountPublicNumber(ctx, oldA); err != nil || got != a {
		t.Fatal("number rotated across repository restart/identity changes", err)
	}
	const count = 16
	var group sync.WaitGroup
	results := make(chan int64, count)
	for i := 0; i < count; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			key := uuid.NewString()
			identity, err := repo.CreateAccountWithIdentity(ctx, domain.IdentityProviderYandex, key, key)
			if err != nil {
				t.Error(err)
				return
			}
			n, err := repo.GetAccountPublicNumber(ctx, identity.AccountID)
			if err != nil {
				t.Error(err)
				return
			}
			results <- n
		}()
	}
	group.Wait()
	close(results)
	seen := map[int64]bool{a: true, b: true}
	for number := range results {
		if number <= 0 || seen[number] {
			t.Fatal("duplicate or invalid number", number)
		}
		seen[number] = true
	}
	if len(seen) != count+2 {
		t.Fatalf("assigned %d accounts, expected %d", len(seen), count+2)
	}
	// Retiring an account never allows a later account to reuse its public ID.
	var deleted int64
	if err = pool.QueryRow(ctx, "INSERT INTO accounts(id) VALUES($1) RETURNING public_number", uuid.New()).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM accounts WHERE public_number=$1", deleted); err != nil {
		t.Fatal(err)
	}
	var next int64
	if err = pool.QueryRow(ctx, "INSERT INTO accounts(id) VALUES($1) RETURNING public_number", uuid.New()).Scan(&next); err != nil || next <= deleted {
		t.Fatal("retired number reused", err)
	}
	rollback, err := os.ReadFile("../../../../migrations/000065_account_public_id.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(rollback)); err == nil {
		t.Fatal("rollback destroyed permanent public identifiers")
	}
	if got, err := repo.GetAccountPublicNumber(ctx, oldA); err != nil || got != a {
		t.Fatal("failed rollback changed assigned public ID", err)
	}
}
