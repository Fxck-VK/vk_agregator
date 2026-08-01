package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

func TestPrepareAccountJobPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	artifacts := postgres.NewArtifactRepository(pool)
	uowManager := postgres.NewUnitOfWork(pool)

	owner := uuid.New()
	foreign := uuid.New()
	for _, accountID := range []uuid.UUID{owner, foreign} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
			t.Fatalf("insert account %s: %v", accountID, err)
		}
	}
	input := joborchestrator.PrepareAccountJobInput{
		AccountID:           owner,
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      "prepared-postgres-" + uuid.NewString(),
		CorrelationID:       "prepared-postgres",
		CostEstimateCredits: 1,
	}
	lookupBarrier := newPreparedAccountLookupBarrier(jobs, input.AccountID, input.IdempotencyKey)
	orch := joborchestrator.New(
		lookupBarrier,
		uowManager,
		billingservice.New(postgres.NewBillingRepository(pool)),
		0,
		joborchestrator.WithArtifactRepository(artifacts),
	)

	type result struct {
		job *domain.Job
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := orch.PrepareAccountJob(ctx, input)
			results <- result{job: job, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var prepared *domain.Job
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent prepare: %v", result.err)
		}
		if result.job == nil {
			t.Fatal("concurrent prepare returned nil job")
		}
		if prepared == nil {
			prepared = result.job
		} else if result.job.ID != prepared.ID {
			t.Fatalf("concurrent prepare IDs = %s and %s, want one job", prepared.ID, result.job.ID)
		}
	}

	var jobsCount, createdCount, queuedCount, reservationsCount, accountsCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&jobsCount); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type = 'event.job.created'`).Scan(&createdCount); err != nil {
		t.Fatalf("count created events: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type = 'event.job.queued'`).Scan(&queuedCount); err != nil {
		t.Fatalf("count queued events: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_reservations`).Scan(&reservationsCount); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_accounts`).Scan(&accountsCount); err != nil {
		t.Fatalf("count credit accounts: %v", err)
	}
	if jobsCount != 1 || createdCount != 1 || queuedCount != 0 || reservationsCount != 0 || accountsCount != 0 {
		t.Fatalf("prepared transaction counts jobs/created/queued/reservations/accounts = %d/%d/%d/%d/%d, want 1/1/0/0/0", jobsCount, createdCount, queuedCount, reservationsCount, accountsCount)
	}
	if got := lookupBarrier.InitialMisses(); got != 2 {
		t.Fatalf("initial scoped idempotency lookup misses = %d, want 2", got)
	}
	if got := lookupBarrier.FallbackLookups(); got != 1 {
		t.Fatalf("fallback idempotency lookups after concurrent create = %d, want 1", got)
	}

	var legacyUser, legacyCommand *uuid.UUID
	var legacyPeer *int64
	if err := pool.QueryRow(ctx, `SELECT user_id, command_id, vk_peer_id FROM jobs WHERE id = $1`, prepared.ID).Scan(&legacyUser, &legacyCommand, &legacyPeer); err != nil {
		t.Fatalf("read nullable legacy job fields: %v", err)
	}
	if legacyUser != nil || legacyCommand != nil || legacyPeer != nil {
		t.Fatalf("account-native legacy fields must be SQL NULL: user=%v command=%v peer=%v", legacyUser, legacyCommand, legacyPeer)
	}

	if got, err := jobs.GetByIDForAccount(ctx, owner, prepared.ID); err != nil || got.ID != prepared.ID || got.AccountID != owner {
		t.Fatalf("owner job read = %+v, %v", got, err)
	}
	if got, err := jobs.GetByIdempotencyKeyForAccount(ctx, owner, input.IdempotencyKey); err != nil || got.ID != prepared.ID {
		t.Fatalf("owner job key read = %+v, %v", got, err)
	}
	for _, accountID := range []uuid.UUID{foreign, uuid.Nil} {
		if _, err := jobs.GetByIDForAccount(ctx, accountID, prepared.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("non-owner job read for %s = %v, want ErrNotFound", accountID, err)
		}
	}

	artifact := &domain.Artifact{
		ID:                      uuid.New(),
		OwnerAccountID:          owner,
		Kind:                    domain.ArtifactKindInput,
		MediaType:               domain.MediaTypeImage,
		MimeType:                "image/png",
		StorageBucket:           "artifacts",
		StorageKey:              "inputs/owned.png",
		SHA256:                  "owned-" + uuid.NewString(),
		ValidationPolicyVersion: "test-policy",
		LifecycleClass:          domain.ArtifactLifecycleInputReference,
		Status:                  domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, artifact); err != nil {
		t.Fatalf("create account artifact: %v", err)
	}
	var artifactLegacyOwner *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT owner_user_id FROM artifacts WHERE id = $1`, artifact.ID).Scan(&artifactLegacyOwner); err != nil {
		t.Fatalf("read nullable legacy artifact owner: %v", err)
	}
	if artifactLegacyOwner != nil {
		t.Fatalf("account-native artifact owner_user_id must be SQL NULL, got %v", artifactLegacyOwner)
	}
	ownerless := &domain.Artifact{
		ID:                      uuid.New(),
		Kind:                    domain.ArtifactKindInput,
		MediaType:               domain.MediaTypeImage,
		MimeType:                "image/png",
		StorageBucket:           "artifacts",
		StorageKey:              "inputs/ownerless.png",
		SHA256:                  "ownerless-" + uuid.NewString(),
		ValidationPolicyVersion: "test-policy",
		LifecycleClass:          domain.ArtifactLifecycleInputReference,
		Status:                  domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, ownerless); err != nil {
		t.Fatalf("create ownerless artifact: %v", err)
	}
	if got, err := artifacts.GetByIDForAccount(ctx, owner, artifact.ID); err != nil || got.ID != artifact.ID || got.OwnerUserID != uuid.Nil {
		t.Fatalf("owner artifact read = %+v, %v", got, err)
	}
	for _, id := range []uuid.UUID{artifact.ID, ownerless.ID} {
		if _, err := artifacts.GetByIDForAccount(ctx, foreign, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("foreign artifact read %s = %v, want ErrNotFound", id, err)
		}
	}
	if _, err := artifacts.GetByIDForAccount(ctx, owner, ownerless.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ownerless artifact read = %v, want ErrNotFound", err)
	}

	if got, err := orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           owner,
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      input.IdempotencyKey,
		CostEstimateCredits: 1,
	}); err != nil || got.ID != prepared.ID {
		t.Fatalf("same-account replay = %+v, %v", got, err)
	}
	if got, err := orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           foreign,
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      input.IdempotencyKey,
		CostEstimateCredits: 1,
	}); !errors.Is(err, domain.ErrConflict) || got != nil {
		t.Fatalf("foreign prepared key collision = %+v, %v; want nil, ErrConflict", got, err)
	}
	if got, err := orch.CreateJob(ctx, joborchestrator.CreateJobInput{
		UserID: uuid.New(), Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, IdempotencyKey: input.IdempotencyKey,
	}); !errors.Is(err, domain.ErrConflict) || got != nil {
		t.Fatalf("legacy key collision = %+v, %v; want nil, ErrConflict", got, err)
	}

	publisher := queue.NewMemoryPublisher()
	drained, err := outboxrelay.New(uowManager, publisher).Drain(ctx)
	if err != nil {
		t.Fatalf("drain outbox: %v", err)
	}
	if drained != 1 || publisher.Len() != 0 {
		t.Fatalf("created-only outbox drain = %d events, %d worker tasks; want 1, 0", drained, publisher.Len())
	}
}

func TestActivatePreparedAccountJobPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	uowManager := postgres.NewUnitOfWork(pool)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, owner); err != nil {
		t.Fatalf("insert owner account: %v", err)
	}
	guard := &activationTransactionGuard{entered: make(chan struct{}), release: make(chan struct{})}
	firstOrchestrator := joborchestrator.New(
		jobs,
		uowManager,
		billingservice.New(postgres.NewBillingRepository(pool)),
		0,
		joborchestrator.WithCapacityGuard(guard),
	)
	secondOrchestrator := joborchestrator.New(
		postgres.NewJobRepository(pool),
		postgres.NewUnitOfWork(pool),
		billingservice.New(postgres.NewBillingRepository(pool)),
		0,
		joborchestrator.WithCapacityGuard(guard),
	)
	prepared, err := firstOrchestrator.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           owner,
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      "activate-prepared-postgres-" + uuid.NewString(),
		CorrelationID:       "activate-prepared-postgres",
		CostEstimateCredits: 25,
	})
	if err != nil {
		t.Fatalf("prepare job: %v", err)
	}
	if prepared.ChannelContext == nil || prepared.ChannelContext.Channel != domain.ChannelWeb || prepared.ResultMode != domain.ResultModeAccountHistory || prepared.DeliveryTarget != nil {
		t.Fatalf("prepared result contract = %+v", prepared)
	}

	type result struct {
		job *domain.Job
		err error
	}
	firstResult := make(chan result, 1)
	go func() {
		job, err := firstOrchestrator.ActivatePreparedAccountJob(ctx, owner, prepared.ID)
		firstResult <- result{job: job, err: err}
	}()
	<-guard.entered // first UOW holds the account row while capacity is checked
	secondStarted := make(chan struct{})
	secondResult := make(chan result, 1)
	go func() {
		close(secondStarted)
		job, err := secondOrchestrator.ActivatePreparedAccountJob(ctx, owner, prepared.ID)
		secondResult <- result{job: job, err: err}
	}()
	<-secondStarted
	close(guard.release)
	for _, result := range []result{<-firstResult, <-secondResult} {
		if result.err != nil || result.job == nil || result.job.ID != prepared.ID || result.job.Status != domain.JobStatusQueued {
			t.Fatalf("concurrent activation = %+v, %v; want queued job", result.job, result.err)
		}
	}
	if calls := guard.Calls(); calls != 1 {
		t.Fatalf("capacity guard calls = %d, want 1 because replay reloads after account lock", calls)
	}

	var reservations, queuedEvents, accounts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_reservations WHERE job_id = $1`, prepared.ID).Scan(&reservations); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = 'event.job.queued'`, prepared.ID).Scan(&queuedEvents); err != nil {
		t.Fatalf("count queue events: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_accounts WHERE owner_account_id = $1`, owner).Scan(&accounts); err != nil {
		t.Fatalf("count native credit accounts: %v", err)
	}
	if reservations != 1 || queuedEvents != 1 || accounts != 1 {
		t.Fatalf("activation counts reservations/queued/accounts = %d/%d/%d, want 1/1/1", reservations, queuedEvents, accounts)
	}

	var storedStatus domain.JobStatus
	var reserved int64
	var legacyUser *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT status, cost_reserved, user_id FROM jobs WHERE id = $1`, prepared.ID).Scan(&storedStatus, &reserved, &legacyUser); err != nil {
		t.Fatalf("read activated job: %v", err)
	}
	if storedStatus != domain.JobStatusQueued || reserved != 25 || legacyUser != nil {
		t.Fatalf("activated row status/reserved/user = %s/%d/%v, want queued/25/NULL", storedStatus, reserved, legacyUser)
	}
}

// Regression: an expired confirmation must be committed before the caller sees
// its conflict. Returning ErrConflict from inside the UOW would roll back this
// audit transition and leave the row indefinitely reusable.
func TestActivatePreparedAccountJobExpiredConfirmationCommitsPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, owner); err != nil {
		t.Fatalf("insert owner account: %v", err)
	}

	orch := joborchestrator.New(
		jobs,
		postgres.NewUnitOfWork(pool),
		billingservice.New(postgres.NewBillingRepository(pool)),
		0,
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(1, time.Hour),
	)
	prepared, err := orch.PrepareAccountJob(ctx, joborchestrator.PrepareAccountJobInput{
		AccountID:           owner,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "expired-confirmation-postgres-" + uuid.NewString(),
		CorrelationID:       "expired-confirmation-postgres",
		CostEstimateCredits: 1,
	})
	if err != nil {
		t.Fatalf("prepare image job: %v", err)
	}
	past := time.Now().Add(-time.Second)
	prepared.ExpiresAt = &past
	if err := jobs.Update(ctx, prepared); err != nil {
		t.Fatalf("make confirmation expired: %v", err)
	}

	activated, err := orch.ActivatePreparedAccountJob(ctx, owner, prepared.ID)
	if !errors.Is(err, domain.ErrConflict) || activated != nil {
		t.Fatalf("activate expired confirmation = %+v, %v; want nil, ErrConflict", activated, err)
	}
	stored, err := jobs.GetByIDForAccount(ctx, owner, prepared.ID)
	if err != nil {
		t.Fatalf("read expired job: %v", err)
	}
	if stored.Status != domain.JobStatusExpired || stored.ErrorCode != "prepared_confirmation_expired" {
		t.Fatalf("stored confirmation = status:%s code:%q, want expired/prepared_confirmation_expired", stored.Status, stored.ErrorCode)
	}
	var reservations, queuedEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_reservations WHERE job_id = $1`, prepared.ID).Scan(&reservations); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = 'event.job.queued'`, prepared.ID).Scan(&queuedEvents); err != nil {
		t.Fatalf("count queue events: %v", err)
	}
	if reservations != 0 || queuedEvents != 0 {
		t.Fatalf("expired confirmation side effects reservations/queued = %d/%d, want 0/0", reservations, queuedEvents)
	}
}

func preparedAccountIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping prepared-account PostgreSQL integration test")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	schema := "prepared_account_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create temporary schema: %v", err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		t.Fatalf("parse pool config: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		t.Fatalf("connect temporary schema pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		admin.Close()
	})

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	defer conn.Release()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	for _, name := range preparedAccountMigrationFiles {
		preparedRunMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", name))
	}
	return pool
}

var preparedAccountMigrationFiles = []string{
	"000001_init_schema.up.sql",
	"000002_inbound_events.up.sql",
	"000003_moderation_results.up.sql",
	"000004_backfill_opening_grants.up.sql",
	"000005_user_vk_profile.up.sql",
	"000006_conversation_context.up.sql",
	"000007_referrals.up.sql",
	"000008_conversation_sources.up.sql",
	"000009_payments.up.sql",
	"000010_payment_product_catalog.up.sql",
	"000011_payment_intent_receipt_snapshot.up.sql",
	"000012_neirohub_crystal_catalog.up.sql",
	"000013_referral_activation_status.up.sql",
	"000014_referral_events.up.sql",
	"000015_product_observability_indexes.up.sql",
	"000016_video_media_metadata.up.sql",
	"000017_media_cleanup_indexes.up.sql",
	"000018_media_lifecycle.up.sql",
	"000019_operator_audit_entries.up.sql",
	"000020_jobs_source.up.sql",
	"000021_retention_schema.up.sql",
	"000022_job_log_retention.up.sql",
	"000023_daily_analytics_aggregates.up.sql",
	"000024_command_raw_text_retention.up.sql",
	"000025_postgres_diagnostics_indexes.up.sql",
	"000026_dev_payment_test_product.up.sql",
	"000027_job_pricing_snapshot.up.sql",
	"000028_runtime_pricing_catalog.up.sql",
	"000029_reduce_opening_balance_to_30.up.sql",
	"000030_miniapp_single_default_chat.up.sql",
	"000031_operator_jobs_console_indexes.up.sql",
	"000032_operator_payments_console_indexes.up.sql",
	"000033_operator_provider_health_indexes.up.sql",
	"000034_daily_dlq_stats.up.sql",
	"000035_account_identity.up.sql",
	"000036_backfill_vk_accounts.up.sql",
	"000037_account_business_dual_write.up.sql",
	"000038_account_sessions_hardening.up.sql",
	"000039_account_password_login.up.sql",
	"000040_provider_task_payload_redaction.up.sql",
	"000041_star_denomination.up.sql",
	"000042_account_session_access_tokens.up.sql",
	"000043_account_native_legacy_nullable.up.sql",
	"000044_channel_context_and_result_mode.up.sql",
	"000045_outbox_claim_lease.up.sql",
}

type preparedAccountLookupBarrier struct {
	domain.JobRepository

	mu              sync.Mutex
	accountID       uuid.UUID
	key             string
	initialMisses   int
	fallbackLookups int
	release         chan struct{}
}

type activationTransactionGuard struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

func (g *activationTransactionGuard) CheckCapacity(_ context.Context, _ joborchestrator.CapacityCheckInput) error {
	g.mu.Lock()
	g.calls++
	first := g.calls == 1
	g.mu.Unlock()
	if first {
		close(g.entered)
		<-g.release
	}
	return nil
}

func (g *activationTransactionGuard) Calls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func newPreparedAccountLookupBarrier(jobs domain.JobRepository, accountID uuid.UUID, key string) *preparedAccountLookupBarrier {
	return &preparedAccountLookupBarrier{
		JobRepository: jobs,
		accountID:     accountID,
		key:           key,
		release:       make(chan struct{}),
	}
}

func (b *preparedAccountLookupBarrier) GetByIdempotencyKeyForAccount(ctx context.Context, accountID uuid.UUID, key string) (*domain.Job, error) {
	job, err := b.JobRepository.GetByIdempotencyKeyForAccount(ctx, accountID, key)
	if accountID != b.accountID || key != b.key || !errors.Is(err, domain.ErrNotFound) {
		return job, err
	}

	b.mu.Lock()
	if b.initialMisses >= 2 {
		b.mu.Unlock()
		return job, err
	}
	b.initialMisses++
	if b.initialMisses == 2 {
		close(b.release)
	}
	b.mu.Unlock()

	<-b.release
	return job, err
}

func (b *preparedAccountLookupBarrier) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Job, error) {
	if key == b.key {
		b.mu.Lock()
		b.fallbackLookups++
		b.mu.Unlock()
	}
	return b.JobRepository.GetByIdempotencyKey(ctx, key)
}

func (b *preparedAccountLookupBarrier) InitialMisses() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.initialMisses
}

func (b *preparedAccountLookupBarrier) FallbackLookups() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fallbackLookups
}

func preparedRunMigrationFile(t *testing.T, ctx context.Context, conn *pgxpool.Conn, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	mrr := conn.Conn().PgConn().Exec(ctx, string(raw))
	for mrr.NextResult() {
		if _, err := mrr.ResultReader().Close(); err != nil {
			t.Fatalf("execute migration %s: %v", path, err)
		}
	}
	if err := mrr.Close(); err != nil {
		t.Fatalf("execute migration %s: %v", path, err)
	}
}
