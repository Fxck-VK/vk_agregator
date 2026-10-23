package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/service/legalacceptance"
	legalcatalog "vk-ai-aggregator/web/platform/legal"
)

func legalTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("LEGAL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LEGAL_TEST_DATABASE_URL required for isolated PostgreSQL tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid isolated test database configuration")
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "epifera_legal_") {
		t.Fatal("refusing non-test database")
	}
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	schema := "legal_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	if _, err = pool.Exec(ctx, `CREATE TABLE accounts(id uuid PRIMARY KEY,status text NOT NULL DEFAULT 'active'); CREATE TABLE account_identities(id uuid PRIMARY KEY,account_id uuid REFERENCES accounts(id),provider text,normalized_id text,verified_at timestamptz,created_at timestamptz DEFAULT now());`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../../migrations/000062_account_legal_consents.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestLegalPostgresEvidenceReplayAndRecipientChanges(t *testing.T) {
	ctx := context.Background()
	pool := legalTestPool(t)
	account, identity := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts(id) VALUES($1);`, account); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO account_identities(id,account_id,provider,normalized_id,verified_at) VALUES($1,$2,'email','synthetic@example.test',now())`, identity, account); err != nil {
		t.Fatal(err)
	}
	catalog := legalcatalog.New("Synthetic Test Operator", "123456789012", true)
	meta := legalacceptance.Metadata{Published: catalog.Published, Version: catalog.Version, Checksum: catalog.Checksum, MarketingVersion: catalog.MarketingVersion, MarketingChecksum: catalog.MarketingChecksum, Snapshot: catalog.Snapshot}
	repo := postgres.NewLegalConsentRepository(pool)
	svc := legalacceptance.New(repo, meta)
	status, err := svc.Status(ctx, account)
	if err != nil || status.Accepted || status.MarketingOptIn || !status.MarketingAvailable || status.RecipientVersion == "" {
		t.Fatalf("unexpected initial state: %v", err)
	}
	record := legalacceptance.Record{AccountID: account, RequestID: uuid.New(), Kind: "accept", Metadata: meta, Locale: "ru"}
	if err = repo.Record(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err = repo.Record(ctx, record); err != nil {
		t.Fatal("replay failed", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM account_legal_events WHERE account_id=$1`, account).Scan(&count); err != nil || count != 1 {
		t.Fatal("replay created duplicate evidence", count, err)
	}
	record.Locale = "en"
	if err = repo.Record(ctx, record); !errors.Is(err, legalacceptance.ErrReplay) {
		t.Fatal("changed payload reused request ID", err)
	}
	input := legalacceptance.MarketingInput{RequestID: uuid.New(), Version: meta.MarketingVersion, Checksum: meta.MarketingChecksum, Locale: "ru", OptIn: true, RecipientVersion: status.RecipientVersion}
	status, err = svc.UpdateMarketing(ctx, account, input)
	if err != nil || !status.MarketingOptIn || !status.Accepted {
		t.Fatal("explicit consent not effective", err)
	}
	// Changing the primary address invalidates its consent. A stale tab cannot
	// silently enroll the new address, even through the storage boundary.
	if _, err = pool.Exec(ctx, `UPDATE account_identities SET normalized_id='replacement@example.test' WHERE id=$1`, identity); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(ctx, account)
	if err != nil || status.MarketingOptIn {
		t.Fatal("new address inherited consent", err)
	}
	record.RequestID = uuid.New()
	record.Kind = "marketing_grant"
	record.MarketingOptIn = true
	record.RecipientVersion = input.RecipientVersion
	if err = repo.Record(ctx, record); !errors.Is(err, legalacceptance.ErrEmail) {
		t.Fatal("stale recipient accepted in transaction", err)
	}
	input.RequestID = uuid.New()
	input.RecipientVersion = status.RecipientVersion
	if status, err = svc.UpdateMarketing(ctx, account, input); err != nil || !status.MarketingOptIn {
		t.Fatal("current recipient cannot opt in", err)
	}
	meta.Published = false
	paused := legalacceptance.New(repo, meta)
	withdraw := legalacceptance.MarketingInput{RequestID: uuid.New(), OptIn: false, Version: "old", Checksum: "old", Locale: "ru"}
	if _, err = paused.UpdateMarketing(ctx, account, withdraw); err != nil {
		t.Fatal("withdrawal blocked while paused", err)
	}
	// A deployment between a successful revoke and its lost-response retry must
	// not turn the same withdrawal into an idempotency conflict.
	newRevision := legalcatalog.New("Updated Synthetic Test Operator", "123456789012", false)
	updated := meta
	updated.Checksum, updated.Snapshot = newRevision.Checksum, newRevision.Snapshot
	if _, err = legalacceptance.New(repo, updated).UpdateMarketing(ctx, account, withdraw); err != nil {
		t.Fatal("withdrawal retry blocked by rollout", err)
	}
	meta.Published = true
	if status, err = svc.Status(ctx, account); err != nil || status.MarketingOptIn {
		t.Fatal("withdrawal did not persist", err)
	}
	var exact bool
	if err = pool.QueryRow(ctx, `SELECT snapshot->>'checksum'=$1 FROM account_legal_revisions WHERE checksum=$1`, meta.Checksum).Scan(&exact); err != nil || !exact {
		t.Fatal("exact legal snapshot missing", err)
	}
	var rawContact bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_legal_events WHERE recipient_hash LIKE '%@%')`).Scan(&rawContact); err != nil || rawContact {
		t.Fatal("raw recipient leaked to consent evidence", err)
	}
}
