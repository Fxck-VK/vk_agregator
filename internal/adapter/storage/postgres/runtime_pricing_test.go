package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestRuntimePricingRepositoryActiveLookup(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureRuntimePricingTables(t, ctx, pool)
	cleanupRuntimePricing(t, ctx, pool)
	t.Cleanup(func() { cleanupRuntimePricing(t, ctx, pool) })

	versionID := uuid.New()
	priceVersion := 1701
	if _, err := pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, $2, 'active', $3, 'test', 'test')`, versionID, priceVersion, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert runtime pricing version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    internal_credit_cap, floor_amount_cap, enabled, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, 5000000, $6, 3, 1, 15, 5000000, true, 'test', 'test'
)`,
		versionID,
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		pricingcatalog.PublicImageNanoBanana2,
		pricingcatalog.ImageQuality1K,
		string(pricingcatalog.FloorUnitPoYoCredits),
	); err != nil {
		t.Fatalf("insert runtime price: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    internal_credit_cap, floor_amount_cap, enabled, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, 60000, $6, 3, 1, 4, 60000, true, 'test', 'test'
)`,
		versionID,
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		pricingcatalog.PublicImageGPTImage2,
		pricingcatalog.ImageQuality1K,
		string(pricingcatalog.FloorUnitAPIMartCredits),
	); err != nil {
		t.Fatalf("insert fractional runtime price: %v", err)
	}

	repo := postgres.NewRuntimePricingRepository(pool)
	set, err := repo.ListActivePrices(ctx)
	if err != nil {
		t.Fatalf("ListActivePrices: %v", err)
	}
	if set.Version.PriceVersion != priceVersion || len(set.Prices) != 2 {
		t.Fatalf("unexpected active price set: %+v", set)
	}
	if set.Prices[0].Source != pricingcatalog.RuntimeDBSource {
		t.Fatalf("source = %q, want %q", set.Prices[0].Source, pricingcatalog.RuntimeDBSource)
	}

	key := pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageNanoBanana2,
		Quality:      pricingcatalog.ImageQuality1K,
	}
	price, err := repo.GetActivePrice(ctx, key)
	if err != nil {
		t.Fatalf("GetActivePrice: %v", err)
	}
	credits, err := price.InternalCredits()
	if err != nil {
		t.Fatalf("InternalCredits: %v", err)
	}
	if credits != 15 {
		t.Fatalf("credits = %d, want 15", credits)
	}

	price, err = repo.GetActivePrice(ctx, pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageGPTImage2,
		Quality:      pricingcatalog.ImageQuality1K,
	})
	if err != nil {
		t.Fatalf("GetActivePrice fractional price: %v", err)
	}
	credits, err = price.InternalCredits()
	if err != nil {
		t.Fatalf("fractional InternalCredits: %v", err)
	}
	if credits != 4 {
		t.Fatalf("fractional credits = %d, want exact ceil 4", credits)
	}

	key.Quality = pricingcatalog.ImageQuality2K
	_, err = repo.GetActivePrice(ctx, key)
	if !errors.Is(err, pricingcatalog.ErrPriceNotFound) {
		t.Fatalf("missing runtime price error = %v, want ErrPriceNotFound", err)
	}
}

func TestRuntimePricingDBConstraintsRejectInvalidActivePricing(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureRuntimePricingTables(t, ctx, pool)
	cleanupRuntimePricing(t, ctx, pool)
	t.Cleanup(func() { cleanupRuntimePricing(t, ctx, pool) })

	versionID := uuid.New()
	if _, err := pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 1801, 'active', $2, 'test', 'test')`, versionID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert runtime pricing version: %v", err)
	}

	_, err := pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    enabled, created_by, updated_by
) VALUES ($1, $2, $3, $4, $5, 0, $6, 3, 1, true, 'test', 'test')`,
		versionID,
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		pricingcatalog.PublicImageNanoBanana2,
		pricingcatalog.ImageQuality1K,
		string(pricingcatalog.FloorUnitPoYoCredits),
	)
	if err == nil {
		t.Fatal("zero floor insert succeeded, want DB constraint rejection")
	}

	_, err = pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    enabled, created_by, updated_by
) VALUES ($1, $2, $3, $4, $5, 1, 'credits', 3, 1, true, 'test', 'test')`,
		versionID,
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		pricingcatalog.PublicImageNanoBanana2,
		pricingcatalog.ImageQuality1K,
	)
	if err == nil {
		t.Fatal("unknown floor unit insert succeeded, want DB constraint rejection")
	}

	_, err = pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 1802, 'active', $2, 'test', 'test')`, uuid.New(), time.Now().Add(-time.Minute))
	if err == nil {
		t.Fatal("second active version insert succeeded, want active-version conflict")
	}
}

func TestRuntimePricingRepositoryFailsClosedForMissingDisabledAndFuturePrices(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureRuntimePricingTables(t, ctx, pool)
	repo := postgres.NewRuntimePricingRepository(pool)

	t.Run("active version without enabled prices", func(t *testing.T) {
		cleanupRuntimePricing(t, ctx, pool)
		versionID := uuid.New()
		if _, err := pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 1901, 'active', $2, 'test', 'test')`, versionID, time.Now().Add(-time.Hour)); err != nil {
			t.Fatalf("insert empty runtime pricing version: %v", err)
		}
		_, err := repo.ListActivePrices(ctx)
		if !errors.Is(err, pricingcatalog.ErrInvalidRuntimePrice) {
			t.Fatalf("empty active version error = %v, want ErrInvalidRuntimePrice", err)
		}
	})

	t.Run("active version with only disabled price", func(t *testing.T) {
		cleanupRuntimePricing(t, ctx, pool)
		versionID := uuid.New()
		if _, err := pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 1902, 'active', $2, 'test', 'test')`, versionID, time.Now().Add(-time.Hour)); err != nil {
			t.Fatalf("insert disabled runtime pricing version: %v", err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    enabled, created_by, updated_by
) VALUES ($1, $2, $3, $4, $5, 5000000, $6, 3, 1, false, 'test', 'test')`,
			versionID,
			string(domain.OperationImageGenerate),
			string(domain.ModalityImage),
			pricingcatalog.PublicImageNanoBanana2,
			pricingcatalog.ImageQuality1K,
			string(pricingcatalog.FloorUnitPoYoCredits),
		); err != nil {
			t.Fatalf("insert disabled runtime price: %v", err)
		}
		_, err := repo.ListActivePrices(ctx)
		if !errors.Is(err, pricingcatalog.ErrInvalidRuntimePrice) {
			t.Fatalf("disabled-only active version error = %v, want ErrInvalidRuntimePrice", err)
		}
	})

	t.Run("future active version is not active yet", func(t *testing.T) {
		cleanupRuntimePricing(t, ctx, pool)
		versionID := uuid.New()
		if _, err := pool.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 1903, 'active', $2, 'test', 'test')`, versionID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("insert future runtime pricing version: %v", err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    enabled, created_by, updated_by
) VALUES ($1, $2, $3, $4, $5, 5000000, $6, 3, 1, true, 'test', 'test')`,
			versionID,
			string(domain.OperationImageGenerate),
			string(domain.ModalityImage),
			pricingcatalog.PublicImageNanoBanana2,
			pricingcatalog.ImageQuality1K,
			string(pricingcatalog.FloorUnitPoYoCredits),
		); err != nil {
			t.Fatalf("insert future runtime price: %v", err)
		}
		_, err := repo.ListActivePrices(ctx)
		if !errors.Is(err, pricingcatalog.ErrNoActiveVersion) {
			t.Fatalf("future active version error = %v, want ErrNoActiveVersion", err)
		}
	})
}

func TestRuntimePricingMigrationUpDownPreservesReadableJobSnapshots(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL migration test")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	schema := "runtime_pricing_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := conn.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, `RESET search_path`)
		_, _ = conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	}()
	if _, err := conn.Exec(ctx, `SET search_path TO `+schema+`, public`); err != nil {
		t.Fatalf("set search path: %v", err)
	}

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000001_init_schema.up.sql"))
	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000027_job_pricing_snapshot.up.sql"))
	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000028_runtime_pricing_catalog.up.sql"))

	var userID uuid.UUID
	if err := conn.QueryRow(ctx, `INSERT INTO users (vk_user_id) VALUES ($1) RETURNING id`, uniqueVKID()).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	key := pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageNanoBanana2,
		Quality:      pricingcatalog.ImageQuality1K,
	}

	legacyJobID := insertRuntimePricingMigrationJob(t, ctx, conn, userID, nil, 7)

	staticCatalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("static catalog: %v", err)
	}
	staticSnapshot, err := staticCatalog.Snapshot(key)
	if err != nil {
		t.Fatalf("static snapshot: %v", err)
	}
	staticSnapshotRaw, err := json.Marshal(staticSnapshot)
	if err != nil {
		t.Fatalf("marshal static snapshot: %v", err)
	}
	staticJobID := insertRuntimePricingMigrationJob(t, ctx, conn, userID, staticSnapshotRaw, 999)

	versionID := uuid.New()
	if _, err := conn.Exec(ctx, `
INSERT INTO runtime_pricing_catalog_versions (id, price_version, status, effective_from, created_by, updated_by)
VALUES ($1, 2001, 'active', $2, 'test', 'test')`, versionID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert runtime pricing version: %v", err)
	}
	if _, err := conn.Exec(ctx, `
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, quality,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator,
    internal_credit_cap, floor_amount_cap, enabled, created_by, updated_by
) VALUES ($1, $2, $3, $4, $5, 8000000, $6, 3, 1, 100, 8000000, true, 'test', 'test')`,
		versionID,
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		pricingcatalog.PublicImageNanoBanana2,
		pricingcatalog.ImageQuality1K,
		string(pricingcatalog.FloorUnitPoYoCredits),
	); err != nil {
		t.Fatalf("insert runtime price: %v", err)
	}
	runtimePrice, err := postgres.NewRuntimePricingRepository(conn).GetActivePrice(ctx, key)
	if err != nil {
		t.Fatalf("get runtime price: %v", err)
	}
	runtimeSnapshot, err := runtimePrice.Snapshot()
	if err != nil {
		t.Fatalf("runtime snapshot: %v", err)
	}
	runtimeSnapshotRaw, err := json.Marshal(runtimeSnapshot)
	if err != nil {
		t.Fatalf("marshal runtime snapshot: %v", err)
	}
	runtimeJobID := insertRuntimePricingMigrationJob(t, ctx, conn, userID, runtimeSnapshotRaw, 999)

	assertJobCharge(t, ctx, conn, legacyJobID, 7, false)
	assertJobCharge(t, ctx, conn, staticJobID, staticSnapshot.InternalCredits, true)
	assertJobCharge(t, ctx, conn, runtimeJobID, runtimeSnapshot.InternalCredits, true)

	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000028_runtime_pricing_catalog.down.sql"))
	assertRuntimePricingTablesDropped(t, ctx, conn)

	assertJobCharge(t, ctx, conn, legacyJobID, 7, false)
	assertJobCharge(t, ctx, conn, staticJobID, staticSnapshot.InternalCredits, true)
	assertJobCharge(t, ctx, conn, runtimeJobID, runtimeSnapshot.InternalCredits, true)
}

func ensureRuntimePricingTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "000028_runtime_pricing_catalog.up.sql"))
	if err != nil {
		t.Fatalf("read runtime pricing migration: %v", err)
	}
	for _, stmt := range splitStatements(string(raw)) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("apply runtime pricing migration statement %q: %v", stmt, err)
		}
	}
}

func insertRuntimePricingMigrationJob(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, snapshot []byte, costReserved int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := execer.QueryRow(ctx, `
INSERT INTO jobs (
    user_id, vk_peer_id, operation_type, modality, idempotency_key,
    pricing_snapshot, cost_reserved
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id`,
		userID,
		uniqueVKID(),
		string(domain.OperationImageGenerate),
		string(domain.ModalityImage),
		"runtime-pricing-migration:"+uuid.NewString(),
		nullableBytes(snapshot),
		costReserved,
	).Scan(&id); err != nil {
		t.Fatalf("insert migration job: %v", err)
	}
	return id
}

func cleanupRuntimePricing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
TRUNCATE runtime_pricing_audit_events, runtime_generation_prices, runtime_pricing_catalog_versions
RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("cleanup runtime pricing: %v", err)
	}
}

func assertJobCharge(t *testing.T, ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id uuid.UUID, want int64, wantSnapshot bool) {
	t.Helper()
	var rawSnapshot []byte
	var costReserved int64
	if err := queryer.QueryRow(ctx, `
SELECT pricing_snapshot, cost_reserved
FROM jobs
WHERE id = $1`, id).Scan(&rawSnapshot, &costReserved); err != nil {
		t.Fatalf("get job %s charge fields: %v", id, err)
	}
	job := domain.Job{PricingSnapshot: rawSnapshot, CostReserved: costReserved}
	credits, hasSnapshot := job.PricingSnapshotCredits()
	if hasSnapshot != wantSnapshot {
		t.Fatalf("job %s snapshot presence = %v, want %v; snapshot credits=%d", id, hasSnapshot, wantSnapshot, credits)
	}
	if charge := job.ChargeAmountCredits(); charge != want {
		t.Fatalf("job %s charge = %d, want %d", id, charge, want)
	}
}

func assertRuntimePricingTablesDropped(t *testing.T, ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) {
	t.Helper()
	for _, table := range []string{
		"runtime_pricing_audit_events",
		"runtime_generation_prices",
		"runtime_pricing_catalog_versions",
	} {
		var exists bool
		if err := queryer.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if exists {
			t.Fatalf("runtime pricing table %s still exists after down migration", table)
		}
	}
}

func nullableBytes(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
