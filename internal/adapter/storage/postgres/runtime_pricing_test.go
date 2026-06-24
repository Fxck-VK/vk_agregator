package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

	repo := postgres.NewRuntimePricingRepository(pool)
	set, err := repo.ListActivePrices(ctx)
	if err != nil {
		t.Fatalf("ListActivePrices: %v", err)
	}
	if set.Version.PriceVersion != priceVersion || len(set.Prices) != 1 {
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

func cleanupRuntimePricing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
TRUNCATE runtime_pricing_audit_events, runtime_generation_prices, runtime_pricing_catalog_versions
RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("cleanup runtime pricing: %v", err)
	}
}
