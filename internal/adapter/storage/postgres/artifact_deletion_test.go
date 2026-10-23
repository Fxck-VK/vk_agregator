package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactdeletion"
)

func deletionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ARTIFACT_DELETION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated artifact-deletion PostgreSQL not configured")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "epifera_delete_") {
		t.Fatal("refusing a non-disposable test database")
	}
	schema := "artifact_delete_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `CREATE TABLE jobs(id uuid PRIMARY KEY,account_id uuid,source text DEFAULT 'web',status text DEFAULT 'succeeded',input_artifact_ids uuid[] DEFAULT '{}',output_artifact_ids uuid[] DEFAULT '{}',params jsonb DEFAULT '{}');
 CREATE TABLE deliveries(artifact_id uuid);
 CREATE TABLE artifacts(id uuid PRIMARY KEY,owner_account_id uuid,job_id uuid,kind text DEFAULT 'output',media_type text DEFAULT 'image',mime_type text DEFAULT 'image/png',status text DEFAULT 'ready',lifecycle_class text DEFAULT 'provider_original',storage_bucket text DEFAULT 'private',storage_key text DEFAULT 'original',size_bytes bigint DEFAULT 4,public_url text DEFAULT '',expires_at timestamptz,deleted_at timestamptz,updated_at timestamptz DEFAULT now());
 CREATE TABLE artifact_variants(id uuid PRIMARY KEY,artifact_id uuid,variant_type text,storage_bucket text,storage_key text,mime_type text,size_bytes bigint,width int,height int,duration_ms bigint,codec text,container text,bitrate_bps bigint,probe_status text,lifecycle_class text DEFAULT 'delivery_variant',created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),expires_at timestamptz,deleted_at timestamptz);`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../../migrations/000060_artifact_deletion_guards.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	return pool
}

func seedDeletionOutput(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	owner, id, job := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,output_artifact_ids)VALUES($1,$2,ARRAY[$3::uuid])`, job, owner, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,owner_account_id,job_id)VALUES($3,$2,$1)`, job, owner, id); err != nil {
		t.Fatal(err)
	}
	return owner, id, job
}

func TestArtifactDeletionPostgresOwnerBusyReplayAndPhysicalCleanup(t *testing.T) {
	ctx := context.Background()
	pool := deletionTestPool(t)
	owner, id, job := seedDeletionOutput(t, pool)
	repo := postgres.NewArtifactRepository(pool)
	cleaner := postgres.NewMaintenanceRepository(pool)
	variant := &domain.ArtifactVariant{ID: uuid.New(), ArtifactID: id, VariantType: domain.VariantThumbnail, StorageBucket: "private", StorageKey: "thumbnail", MimeType: "image/webp", SizeBytes: 4}
	if err := repo.AddVariant(ctx, variant); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RequestOutputDeletionForAccount(ctx, uuid.New(), id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign file allowed", err)
	}
	consumer := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,status,input_artifact_ids)VALUES($1,$2,'queued',ARRAY[$3::uuid])`, consumer, owner, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RequestOutputDeletionForAccount(ctx, owner, id); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("unfinished consumer lost bytes", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status='succeeded' WHERE id=$1`, consumer); err != nil {
		t.Fatal(err)
	}
	objects := memory.NewObjectStore()
	_ = objects.Put(ctx, "private", "original", []byte("data"), "image/png")
	_ = objects.Put(ctx, "private", "thumbnail", []byte("data"), "image/webp")
	service := artifactdeletion.New(repo, cleaner, objects)
	if err := service.Delete(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	if objects.Len() != 0 {
		t.Fatal("objects not physically removed")
	}
	if err := service.Delete(ctx, owner, id); err != nil {
		t.Fatal("repeat deletion failed", err)
	}
	var status string
	var size int64
	if err := pool.QueryRow(ctx, `SELECT status,size_bytes FROM artifacts WHERE id=$1`, id).Scan(&status, &size); err != nil || status != "deleted" || size != 0 {
		t.Fatal("tombstone not retained", err, status, size)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,input_artifact_ids)VALUES($1,$2,ARRAY[$3::uuid])`, uuid.New(), owner, id); err == nil {
		t.Fatal("deleted file reused")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,account_id,params)VALUES($1,$2,jsonb_build_object('source_job_id',$3::text))`, uuid.New(), owner, job.String()); err == nil {
		t.Fatal("deleted job output reused by continuation")
	}
	if err := repo.AddVariant(ctx, &domain.ArtifactVariant{ID: uuid.New(), ArtifactID: id, VariantType: domain.VariantPreview}); err == nil {
		t.Fatal("late variant resurrected deleted output")
	}
}

func TestArtifactDeletionPostgresWaitsForVariantWrite(t *testing.T) {
	ctx := context.Background()
	pool := deletionTestPool(t)
	owner, id, _ := seedDeletionOutput(t, pool)
	repo := postgres.NewArtifactRepository(pool)
	locked, release := make(chan struct{}), make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- repo.WithArtifactWrite(ctx, id, func(domain.ArtifactRepository) error { close(locked); <-release; return nil })
	}()
	<-locked
	deleted := make(chan error, 1)
	go func() { _, err := repo.RequestOutputDeletionForAccount(ctx, owner, id); deleted <- err }()
	select {
	case err := <-deleted:
		t.Fatal("deletion raced variant storage", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	called := false
	if err := repo.WithArtifactWrite(ctx, id, func(domain.ArtifactRepository) error { called = true; return nil }); !errors.Is(err, domain.ErrNotFound) || called {
		t.Fatal("late storage callback ran", err)
	}
}

func TestArtifactDeletionPostgresWaitsForConsumerAdmission(t *testing.T) {
	ctx := context.Background()
	pool := deletionTestPool(t)
	owner, id, _ := seedDeletionOutput(t, pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO jobs(id,account_id,status,input_artifact_ids)VALUES($1,$2,'queued',ARRAY[$3::uuid])`, uuid.New(), owner, id); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewArtifactRepository(pool).RequestOutputDeletionForAccount(ctx, owner, id)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatal("deletion passed uncommitted consumer", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, domain.ErrConflict) {
		t.Fatal("active consumer not protected", err)
	}
}

type unavailableDeletionStore struct{}

func (unavailableDeletionStore) DeleteObject(context.Context, string, string) error {
	return errors.New("storage unavailable")
}
func TestArtifactDeletionPostgresDurableCleanupRetry(t *testing.T) {
	ctx := context.Background()
	pool := deletionTestPool(t)
	owner, id, _ := seedDeletionOutput(t, pool)
	repo, cleanup := postgres.NewArtifactRepository(pool), postgres.NewMaintenanceRepository(pool)
	if err := artifactdeletion.New(repo, cleanup, unavailableDeletionStore{}).Delete(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	// Expiry is assigned by PostgreSQL. Use its clock so a Windows Go test
	// talking to WSL is not sensitive to host/guest clock skew.
	var cutoff time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&cutoff); err != nil {
		t.Fatal(err)
	}
	candidates, err := cleanup.MediaCleanupCandidates(ctx, domain.MediaCleanupPolicy{ExpiredCutoff: cutoff}, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatal("cleanup task lost after storage failure", err, len(candidates))
	}
	objects := memory.NewObjectStore()
	_ = objects.Put(ctx, "private", "original", []byte("data"), "image/png")
	for _, candidate := range candidates {
		if err = objects.DeleteObject(ctx, candidate.StorageBucket, candidate.StorageKey); err != nil {
			t.Fatal(err)
		}
		if err = cleanup.MarkMediaCleanupDeleted(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err = cleanup.MediaCleanupCandidates(ctx, domain.MediaCleanupPolicy{ExpiredCutoff: cutoff}, 10)
	if err != nil || len(candidates) != 0 || objects.Len() != 0 {
		t.Fatal("retried purge incomplete", err)
	}
}
