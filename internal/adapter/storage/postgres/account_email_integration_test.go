package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestPostgresEmailLimitAndReplacementConcurrency(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE account_identities (
 id uuid PRIMARY KEY, account_id uuid NOT NULL REFERENCES accounts(id), provider text NOT NULL,
 external_id text NOT NULL, normalized_id text NOT NULL, verified_at timestamptz, last_used_at timestamptz,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, UNIQUE(provider, normalized_id));
 CREATE TABLE account_links_audit (id uuid PRIMARY KEY, account_id uuid NOT NULL, actor_account_id uuid, action text NOT NULL, provider text NOT NULL, identity_id uuid, created_at timestamptz NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	owner, foreign := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1),($2)", owner, foreign); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	primary, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, email := range []string{"a@example.test", "b@example.test"} {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			<-start
			_, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, email, email)
			results <- err
		}(email)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, limited := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrAccountEmailLimit) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || limited != 1 {
		t.Fatalf("success=%d limited=%d", successes, limited)
	}
	rows, err := repo.ListIdentitiesByAccount(ctx, owner, 100, 0)
	if err != nil || len(rows) != 2 {
		t.Fatal("email limit failed")
	}
	roles := domain.AccountEmailRoles(rows)
	var backup *domain.AccountIdentity
	for _, row := range rows {
		if roles[row.ID] == domain.AccountEmailBackup {
			backup = row
		}
	}
	if backup == nil || roles[primary.ID] != domain.AccountEmailPrimary {
		t.Fatal("email role order invalid")
	}
	_, err = repo.LinkIdentity(ctx, foreign, domain.IdentityProviderEmail, "occupied@example.test", "occupied@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ReplaceBackupEmailIdentity(ctx, owner, backup.ID, "occupied@example.test", "occupied@example.test", backup.UpdatedAt); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign conflict: %v", err)
	}
	if _, err = repo.ReplaceBackupEmailIdentity(ctx, owner, primary.ID, "new@example.test", "new@example.test", primary.UpdatedAt); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatal("primary replaced")
	}
	start = make(chan struct{})
	results = make(chan error, 2)
	for _, email := range []string{"new-a@example.test", "new-b@example.test"} {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			<-start
			_, err := repo.ReplaceBackupEmailIdentity(ctx, owner, backup.ID, email, email, backup.UpdatedAt)
			results <- err
		}(email)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, stale := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrAccountBackupEmailChanged) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || stale != 1 {
		t.Fatalf("success=%d stale=%d", successes, stale)
	}
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, backup.NormalizedID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("old backup still resolves")
	}
	rows, _ = repo.ListIdentitiesByAccount(ctx, owner, 100, 0)
	if len(rows) != 2 || domain.AccountEmailRoles(rows)[primary.ID] != domain.AccountEmailPrimary {
		t.Fatal("replacement changed primary/count")
	}
	// Audit failure must roll back the binding update.
	current := rows[0]
	if current.ID != backup.ID {
		current = rows[1]
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE account_links_audit ADD CONSTRAINT reject_unlink CHECK(action <> 'unlinked') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceBackupEmailIdentity(ctx, owner, current.ID, "rollback@example.test", "rollback@example.test", current.UpdatedAt); err == nil {
		t.Fatal("audit failure ignored")
	}
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, current.NormalizedID); err != nil {
		t.Fatal("audit failure removed old backup")
	}
}
