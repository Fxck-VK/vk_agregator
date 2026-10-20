package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestPostgresYandexMigrationAndLastLogin(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	if _, err := pool.Exec(ctx, `ALTER TABLE accounts
	 ADD COLUMN status text NOT NULL DEFAULT 'active', ADD COLUMN role text NOT NULL DEFAULT 'user',
	 ADD COLUMN account_type text NOT NULL DEFAULT 'personal', ADD COLUMN locale text NOT NULL DEFAULT 'ru',
	 ADD COLUMN timezone text NOT NULL DEFAULT 'Europe/Moscow', ADD COLUMN risk_level integer NOT NULL DEFAULT 0,
	 ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(), ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now()`); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `ALTER TABLE account_identities ADD CONSTRAINT account_identities_provider_check CHECK(provider IN ('vk','telegram','google','apple','email','phone','password'));
	ALTER TABLE account_links_audit ADD CONSTRAINT account_links_audit_provider_check CHECK(provider='' OR provider IN ('vk','telegram','google','apple','email','phone','password'))`)
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../../../migrations/000058_yandex_identity.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	accountID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool, postgres.WithUsableLoginProviders(domain.UsableLoginProviders{domain.IdentityProviderEmail: true, domain.IdentityProviderYandex: true}))
	auth := accountauth.New(identityresolver.New(nil, repo, nil))
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginYandex, ExternalID: "New-Yandex-Subject", Verified: true}
	newAccount, err := auth.ResolveOrCreate(ctx, login)
	if err != nil || newAccount.AccountID == uuid.Nil {
		t.Fatal("new Yandex account not created", err)
	}
	if again, err := auth.ResolveOrCreate(ctx, login); err != nil || again.AccountID != newAccount.AccountID {
		t.Fatal("Yandex login did not reuse its account", err)
	}
	email, err := repo.LinkIdentity(ctx, accountID, domain.IdentityProviderEmail, "synthetic@example.test", "synthetic@example.test")
	if err != nil {
		t.Fatal(err)
	}
	yandex, err := repo.LinkIdentity(ctx, accountID, domain.IdentityProviderYandex, "Stable-Subject", "Stable-Subject")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := repo.LinkIdentity(ctx, accountID, domain.IdentityProviderYandex, "Stable-Subject", "Stable-Subject"); err != nil || again.ID != yandex.ID {
		t.Fatal("non-idempotent Yandex binding", err)
	}
	disabled := postgres.NewAccountIdentityRepository(pool)
	if err := disabled.UnlinkIdentity(ctx, accountID, email.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatal("disabled Yandex allowed removal of last login", err)
	}
	if err := repo.UnlinkIdentity(ctx, accountID, email.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.UnlinkIdentity(ctx, accountID, yandex.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatal("last Yandex sign-in removed", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_links_audit WHERE provider='yandex' AND account_id=$1", accountID).Scan(&count); err != nil || count == 0 {
		t.Fatal("Yandex audit missing", err)
	}
	down, err := os.ReadFile("../../../../migrations/000058_yandex_identity.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	// A schema downgrade must never erase existing Yandex bindings to succeed.
	if _, err := pool.Exec(ctx, string(down)); err == nil {
		t.Fatal("unsafe downgrade accepted existing Yandex data")
	}
	if got, err := repo.ResolveIdentity(ctx, domain.IdentityProviderYandex, "Stable-Subject"); err != nil || got.AccountID != accountID {
		t.Fatal("downgrade damaged Yandex binding", err)
	}
}
