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

func TestPostgresVKProfileNameOwnershipAndExistingName(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	if _, err := pool.Exec(ctx, "ALTER TABLE accounts ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now()"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../../migrations/000057_account_display_name.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	owner, foreign := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1),($2)", owner, foreign); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	vk, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderVK, "12345", "12345")
	if err != nil {
		t.Fatal(err)
	}
	email, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "synthetic@example.test", "synthetic@example.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ owner, binding uuid.UUID }{{foreign, vk.ID}, {owner, uuid.New()}, {owner, email.ID}} {
		if err := repo.SetAccountDisplayNameIfEmpty(ctx, input.owner, input.binding, "Rejected Name"); !errors.Is(err, domain.ErrUnverifiedLogin) {
			t.Fatal("unowned or non-VK proof accepted", err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE account_identities SET verified_at=NULL WHERE id=$1", vk.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAccountDisplayNameIfEmpty(ctx, owner, vk.ID, "Unverified Name"); !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatal("unverified binding accepted")
	}
	if _, err := pool.Exec(ctx, "UPDATE account_identities SET verified_at=now() WHERE id=$1", vk.ID); err != nil {
		t.Fatal(err)
	}
	// Concurrent first logins must store one complete value without overwriting it.
	var group sync.WaitGroup
	failures := make(chan error, 2)
	for _, name := range []string{"First Name", "Second Name"} {
		group.Add(1)
		go func(name string) {
			defer group.Done()
			failures <- repo.SetAccountDisplayNameIfEmpty(ctx, owner, vk.ID, name)
		}(name)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := repo.GetAccountDisplayName(ctx, owner)
	if err != nil || (stored != "First Name" && stored != "Second Name") {
		t.Fatal("name was not persisted atomically", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE accounts SET display_name='Manual Name' WHERE id=$1", owner); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAccountDisplayNameIfEmpty(ctx, owner, vk.ID, "Provider Name"); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetAccountDisplayName(ctx, owner); err != nil || got != "Manual Name" {
		t.Fatal("manual name overwritten", err)
	}
	if got, err := repo.GetAccountDisplayName(ctx, foreign); err != nil || got != "" {
		t.Fatal("another profile changed", err)
	}
}
