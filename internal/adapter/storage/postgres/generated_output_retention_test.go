package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactdeletion"
)

func retentionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := deletionTestPool(t)
	_, err := pool.Exec(context.Background(), `DROP TRIGGER guard_deleted_job_inputs ON jobs; DROP TRIGGER guard_deleted_artifact_variant ON artifact_variants; DROP TRIGGER guard_artifact_tombstone ON artifacts; ALTER TABLE artifacts ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(), ADD COLUMN artifact_tier text DEFAULT 'standard', ADD COLUMN retention_class text; ALTER TABLE artifact_variants ADD COLUMN artifact_tier text DEFAULT 'standard', ADD COLUMN retention_class text;`)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func applyOutputRetention(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	raw, err := os.ReadFile("../../../../migrations/000061_generated_output_retention.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
}

func outputExpiry(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var expiry time.Time
	if err := pool.QueryRow(context.Background(), `SELECT expires_at FROM artifacts WHERE id=$1`, id).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	return expiry
}

func TestGeneratedOutputRetentionCalendarAndPreviewInheritance(t *testing.T) {
	ctx := context.Background()
	pool := retentionTestPool(t)
	applyOutputRetention(t, pool)
	// Calendar months clamp at the end of February and stay anchored in UTC even
	// when the database connection uses a different session time zone.
	_, err := pool.Exec(ctx, `SET TIME ZONE 'Pacific/Auckland'`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ created, want string }{
		{"2025-12-31T20:30:00Z", "2026-02-28T20:30:00Z"},
		{"2023-12-31T20:30:00Z", "2024-02-29T20:30:00Z"},
		{"2026-07-01T11:00:00Z", "2026-09-01T11:00:00Z"},
	} {
		id := uuid.New()
		created, _ := time.Parse(time.RFC3339, tc.created)
		want, _ := time.Parse(time.RFC3339, tc.want)
		if _, err = pool.Exec(ctx, `INSERT INTO artifacts(id,created_at) VALUES($1,$2)`, id, created); err != nil {
			t.Fatal(err)
		}
		if got := outputExpiry(t, pool, id); !got.Equal(want) {
			t.Fatalf("%s: got %s want %s", tc.created, got, want)
		}
		variant := uuid.New()
		if _, err = pool.Exec(ctx, `INSERT INTO artifact_variants(id,artifact_id,variant_type,created_at)VALUES($1,$2,'thumbnail',$3)`, variant, id, want.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		var got time.Time
		if err = pool.QueryRow(ctx, `SELECT expires_at FROM artifact_variants WHERE id=$1`, variant).Scan(&got); err != nil || !got.Equal(want) {
			t.Fatalf("preview extended parent lifetime: %s %v", got, err)
		}
		if _, err = pool.Exec(ctx, `UPDATE artifacts SET updated_at=now(),expires_at=NULL WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if got = outputExpiry(t, pool, id); !got.Equal(want) {
			t.Fatal("ordinary update prolonged storage")
		}
		earlier := want.Add(-time.Hour)
		if _, err = pool.Exec(ctx, `UPDATE artifacts SET expires_at=$2 WHERE id=$1`, id, earlier); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT expires_at FROM artifact_variants WHERE id=$1`, variant).Scan(&got); err != nil || !got.Equal(earlier) {
			t.Fatal("existing preview did not follow earlier parent expiry", got, err)
		}
	}
}

func TestGeneratedOutputRetentionBackfillExclusionsAndReplay(t *testing.T) {
	ctx := context.Background()
	pool := retentionTestPool(t)
	recent := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	earlier := recent.Add(time.Hour)
	for _, media := range []string{"image", "video", "audio", "document"} {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,media_type,created_at,expires_at) VALUES($1,$2,$3,$4)`, id, media, recent, earlier); err != nil {
			t.Fatal(err)
		}
		applyOutputRetention(t, pool)
		if got := outputExpiry(t, pool, id); !got.Equal(earlier) {
			t.Fatal("earlier manual expiry overwritten")
		}
	}
	for _, kind := range []string{"input", "intermediate"} {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,kind,created_at)VALUES($1,$2,$3)`, id, kind, recent); err != nil {
			t.Fatal(err)
		}
		var nilExpiry bool
		_ = pool.QueryRow(ctx, `SELECT expires_at IS NULL FROM artifacts WHERE id=$1`, id).Scan(&nilExpiry)
		if !nilExpiry {
			t.Fatal("input/intermediate file policy changed")
		}
	}
	text := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,media_type)VALUES($1,'text')`, text); err != nil {
		t.Fatal(err)
	}
	var nilExpiry bool
	_ = pool.QueryRow(ctx, `SELECT expires_at IS NULL FROM artifacts WHERE id=$1`, text).Scan(&nilExpiry)
	if !nilExpiry {
		t.Fatal("chat text policy changed")
	}
	// A genuinely historical row and its existing preview receive one shared deadline.
	_, err := pool.Exec(ctx, `DROP TRIGGER generated_output_expiry ON artifacts; DROP TRIGGER generated_output_variant_expiry ON artifact_variants`)
	if err != nil {
		t.Fatal(err)
	}
	id, variant := uuid.New(), uuid.New()
	created := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if _, err = pool.Exec(ctx, `INSERT INTO artifacts(id,created_at)VALUES($1,$2)`, id, created); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO artifact_variants(id,artifact_id,variant_type,storage_bucket,storage_key,size_bytes,created_at)VALUES($1,$2,'thumbnail','private','preview',4,$3)`, variant, id, created.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	applyOutputRetention(t, pool)
	applyOutputRetention(t, pool)
	deadline := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if got := outputExpiry(t, pool, id); !got.Equal(deadline) {
		t.Fatalf("historical deadline: %s", got)
	}
	var got time.Time
	_ = pool.QueryRow(ctx, `SELECT expires_at FROM artifact_variants WHERE id=$1`, variant).Scan(&got)
	if !got.Equal(deadline) {
		t.Fatal("backfilled preview has another deadline")
	}
	repo := postgres.NewMaintenanceRepository(pool)
	before, err := repo.MediaCleanupCandidates(ctx, domain.MediaCleanupPolicy{ExpiredCutoff: deadline.Add(-time.Microsecond)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range before {
		if candidate.ArtifactID == id {
			t.Fatal("file eligible before its two-month deadline")
		}
	}
	after, err := repo.MediaCleanupCandidates(ctx, domain.MediaCleanupPolicy{ExpiredCutoff: deadline}, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, candidate := range after {
		if candidate.ArtifactID == id {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("existing maintenance cannot clean original and preview: %d", found)
	}
}

func TestGeneratedOutputRetentionOverridesTierFallbackAndDownKeepsDeadlines(t *testing.T) {
	ctx := context.Background()
	pool := retentionTestPool(t)
	applyOutputRetention(t, pool)
	created := time.Now().UTC().Truncate(time.Microsecond)
	for _, tier := range []string{"standard", "free", "paid"} {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,artifact_tier,created_at)VALUES($1,$2,$3)`, id, tier, created); err != nil {
			t.Fatal(err)
		}
		var due bool
		if err := pool.QueryRow(ctx, `SELECT expires_at = ((created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC' FROM artifacts WHERE id=$1`, id).Scan(&due); err != nil || !due {
			t.Fatal("tier changed generated-file deadline", tier, err)
		}
	}
	repo := postgres.NewMaintenanceRepository(pool)
	changed, err := repo.ExpireMediaArtifacts(ctx, domain.MediaCleanupPolicy{FreeArtifactCutoff: created.AddDate(1, 0, 0), PaidArtifactCutoff: created.AddDate(1, 0, 0)}, created, 100)
	if err != nil || changed != 0 {
		t.Fatal("legacy tier cutoff overrode explicit deadlines", changed, err)
	}
	down, err := os.ReadFile("../../../../migrations/000061_generated_output_retention.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var kept int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM artifacts WHERE expires_at IS NOT NULL`).Scan(&kept); err != nil || kept != 3 {
		t.Fatal("down silently changed stored deadlines", kept, err)
	}
}

func TestGeneratedOutputRetentionManualDeletionCompatibility(t *testing.T) {
	ctx := context.Background()
	pool := deletionTestPool(t) // Keep migration 000060's guards installed.
	if _, err := pool.Exec(ctx, `ALTER TABLE artifacts ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(), ADD COLUMN artifact_tier text DEFAULT 'standard', ADD COLUMN retention_class text; ALTER TABLE artifact_variants ADD COLUMN artifact_tier text DEFAULT 'standard', ADD COLUMN retention_class text;`); err != nil {
		t.Fatal(err)
	}
	applyOutputRetention(t, pool)
	owner, id, _ := seedDeletionOutput(t, pool)
	repo := postgres.NewArtifactRepository(pool)
	variant := &domain.ArtifactVariant{ID: uuid.New(), ArtifactID: id, VariantType: domain.VariantThumbnail, StorageBucket: "private", StorageKey: "thumbnail", MimeType: "image/webp", SizeBytes: 4}
	if err := repo.AddVariant(ctx, variant); err != nil {
		t.Fatal(err)
	}
	objects := memory.NewObjectStore()
	_ = objects.Put(ctx, "private", "original", []byte("data"), "image/png")
	_ = objects.Put(ctx, "private", "thumbnail", []byte("data"), "image/webp")
	service := artifactdeletion.New(repo, postgres.NewMaintenanceRepository(pool), objects)
	if err := service.Delete(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	if objects.Len() != 0 {
		t.Fatal("calendar policy prevented immediate physical deletion")
	}
	if err := service.Delete(ctx, owner, id); err != nil {
		t.Fatal("deletion replay failed", err)
	}
}
