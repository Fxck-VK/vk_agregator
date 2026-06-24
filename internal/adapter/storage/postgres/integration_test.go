package postgres_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

// These tests exercise the real PostgreSQL repository implementations. They run
// only when TEST_DATABASE_URL points at a reachable Postgres instance (for
// example the one started by docker compose):
//
//	export TEST_DATABASE_URL=postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable
//	go test ./...
//
// When the variable is unset the tests skip, so the default `go test ./...`
// stays green without external infrastructure.

var (
	schemaOnce sync.Once
	schemaErr  error
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	schemaOnce.Do(func() { schemaErr = applySchema(ctx, pool) })
	if schemaErr != nil {
		t.Fatalf("apply schema: %v", schemaErr)
	}
	t.Cleanup(pool.Close)
	return pool
}

// applySchema rebuilds the schema by running the initial migration's down then
// up scripts, so each test run starts from a known clean state.
func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	scripts := []string{
		// Drop in reverse dependency order, then recreate in forward order.
		"000002_inbound_events.down.sql",
		"000001_init_schema.down.sql",
		"000001_init_schema.up.sql",
		"000002_inbound_events.up.sql",
		"000016_video_media_metadata.up.sql",
		"000018_media_lifecycle.up.sql",
	}
	for _, name := range scripts {
		raw, err := os.ReadFile(filepath.Join(root, "migrations", name))
		if err != nil {
			return err
		}
		for _, stmt := range splitStatements(string(raw)) {
			if _, err := pool.Exec(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func repoRoot() (string, error) {
	// Test working directory is the package directory:
	// internal/adapter/storage/postgres -> repo root is four levels up.
	return filepath.Abs(filepath.Join("..", "..", "..", ".."))
}

// splitStatements breaks a migration script into individual executable
// statements, stripping SQL comments and the BEGIN/COMMIT wrappers (each
// statement runs in its own autocommit on a pooled connection).
func splitStatements(script string) []string {
	var out []string
	for _, chunk := range strings.Split(script, ";") {
		var lines []string
		for _, line := range strings.Split(chunk, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "--") {
				continue
			}
			lines = append(lines, line)
		}
		stmt := strings.TrimSpace(strings.Join(lines, "\n"))
		if stmt == "" || strings.EqualFold(stmt, "BEGIN") || strings.EqualFold(stmt, "COMMIT") {
			continue
		}
		out = append(out, stmt)
	}
	return out
}

// uniqueVKID derives a stable-but-unique VK id from a fresh UUID so parallel and
// repeated runs do not collide on the users.vk_user_id unique constraint.
func uniqueVKID() int64 {
	b := uuid.New()
	return int64(binary.BigEndian.Uint64(b[:8]) >> 1)
}

func ensureRetentionColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	classes := map[string]domain.DataClass{
		"commands":          domain.DataClassUserContent,
		"jobs":              domain.DataClassOperational,
		"provider_tasks":    domain.DataClassProviderPayload,
		"inbound_events":    domain.DataClassUserContent,
		"artifacts":         domain.DataClassArtifactMetadata,
		"artifact_variants": domain.DataClassArtifactMetadata,
	}
	for table, class := range classes {
		stmt := "ALTER TABLE " + table + " " +
			"ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ, " +
			"ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ, " +
			"ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ, " +
			"ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT '" + string(class) + "'"
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("ensure retention columns for %s: %v", table, err)
		}
		if _, err := pool.Exec(ctx, "UPDATE "+table+" SET retention_class = $1 WHERE retention_class = ''", string(class)); err != nil {
			t.Fatalf("ensure retention class for %s: %v", table, err)
		}
	}
}

func newTestUser(t *testing.T, ctx context.Context, repo *postgres.UserRepository) *domain.User {
	t.Helper()
	u := &domain.User{
		VKUserID: uniqueVKID(),
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := postgres.NewUserRepository(pool)

	u := newTestUser(t, ctx, repo)
	if u.ID == uuid.Nil {
		t.Fatal("expected generated id")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.VKUserID != u.VKUserID {
		t.Fatalf("vk id mismatch: %d != %d", got.VKUserID, u.VKUserID)
	}

	byVK, err := repo.GetByVKUserID(ctx, u.VKUserID)
	if err != nil {
		t.Fatalf("get by vk id: %v", err)
	}
	if byVK.ID != u.ID {
		t.Fatal("id mismatch on get by vk id")
	}

	u.Status = domain.StatusBlocked
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}
	if reread, _ := repo.GetByID(ctx, u.ID); reread.Status != domain.StatusBlocked {
		t.Fatal("status not persisted")
	}

	if _, err := repo.GetByID(ctx, uuid.New()); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMaintenanceRedactsExpiredVKInboundPayloads(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureRetentionColumns(t, ctx, pool)
	inboundRepo := postgres.NewInboundEventRepository(pool)
	maintenanceRepo := postgres.NewMaintenanceRepository(pool)
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	key := "vk_event:1:" + uuid.NewString()
	event := &domain.InboundEvent{
		Source:         "vk",
		EventType:      "message_new",
		GroupID:        1,
		VKEventID:      "evt-" + uuid.NewString(),
		PeerID:         uniqueVKID(),
		VKUserID:       uniqueVKID(),
		Payload:        []byte(`{"message":"SENSITIVE_TEXT_MARKER","payload":"SENSITIVE_PAYLOAD_MARKER"}`),
		Status:         domain.InboundReceived,
		IdempotencyKey: key,
	}
	if err := inboundRepo.Create(ctx, event); err != nil {
		t.Fatalf("create inbound event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE inbound_events
		SET created_at = $2, updated_at = $2, retention_class = $3
		WHERE id = $1`, event.ID, old, string(domain.DataClassUserContent)); err != nil {
		t.Fatalf("age inbound event: %v", err)
	}

	expired, err := maintenanceRepo.ExpireInboundEvents(ctx, now.Add(-24*time.Hour), now, 10)
	if err != nil {
		t.Fatalf("expire inbound events: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired rows = %d, want 1", expired)
	}
	expired, err = maintenanceRepo.ExpireInboundEvents(ctx, now.Add(-24*time.Hour), now, 10)
	if err != nil {
		t.Fatalf("repeat expire inbound events: %v", err)
	}
	if expired != 0 {
		t.Fatalf("repeat expired rows = %d, want 0", expired)
	}

	beforePayload, beforeRedacted := inboundPayloadState(t, ctx, pool, event.ID)
	dryRun, err := maintenanceRepo.RetentionDryRun(ctx, now, 20)
	if err != nil {
		t.Fatalf("retention dry-run: %v", err)
	}
	var inboundDryRun *domain.RetentionDryRunItem
	for i := range dryRun.Items {
		if dryRun.Items[i].TableName == "inbound_events" {
			inboundDryRun = &dryRun.Items[i]
			break
		}
	}
	if inboundDryRun == nil || inboundDryRun.Count < 1 {
		t.Fatalf("dry-run inbound item = %+v, want count >= 1", inboundDryRun)
	}
	afterDryPayload, afterDryRedacted := inboundPayloadState(t, ctx, pool, event.ID)
	if afterDryPayload != beforePayload || afterDryRedacted != beforeRedacted {
		t.Fatalf("dry-run mutated inbound row: before payload=%s redacted=%v after payload=%s redacted=%v",
			beforePayload, beforeRedacted, afterDryPayload, afterDryRedacted)
	}

	redacted, err := maintenanceRepo.RedactExpiredInboundEvents(ctx, now, 10)
	if err != nil {
		t.Fatalf("redact inbound events: %v", err)
	}
	if redacted != 1 {
		t.Fatalf("redacted rows = %d, want 1", redacted)
	}
	redacted, err = maintenanceRepo.RedactExpiredInboundEvents(ctx, now, 10)
	if err != nil {
		t.Fatalf("repeat redact inbound events: %v", err)
	}
	if redacted != 0 {
		t.Fatalf("repeat redacted rows = %d, want 0", redacted)
	}

	payload, isRedacted := inboundPayloadState(t, ctx, pool, event.ID)
	if !isRedacted {
		t.Fatal("redacted_at is null after redaction")
	}
	if strings.Contains(payload, "SENSITIVE_TEXT_MARKER") || strings.Contains(payload, "SENSITIVE_PAYLOAD_MARKER") {
		t.Fatalf("redacted payload still contains raw marker: %s", payload)
	}
	if !strings.Contains(payload, `"payload_class": "vk_callback_metadata"`) && !strings.Contains(payload, `"payload_class":"vk_callback_metadata"`) {
		t.Fatalf("redacted payload = %s, want metadata marker", payload)
	}
	if _, err := inboundRepo.GetByIdempotencyKey(ctx, key); err != nil {
		t.Fatalf("idempotency lookup after redaction: %v", err)
	}
}

func inboundPayloadState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (string, bool) {
	t.Helper()
	var payload string
	var redacted bool
	if err := pool.QueryRow(ctx, `
		SELECT payload::text, redacted_at IS NOT NULL
		FROM inbound_events
		WHERE id = $1`, id).Scan(&payload, &redacted); err != nil {
		t.Fatalf("load inbound payload state: %v", err)
	}
	return payload, redacted
}

func TestMaintenanceRedactsExpiredCommandRawText(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureRetentionColumns(t, ctx, pool)
	users := postgres.NewUserRepository(pool)
	commands := postgres.NewCommandRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	maintenanceRepo := postgres.NewMaintenanceRepository(pool)
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)

	user := newTestUser(t, ctx, users)
	completed := &domain.Command{
		UserID:         user.ID,
		VKPeerID:       user.VKUserID,
		InboundEventID: uuid.New(),
		Type:           domain.CommandTextAsk,
		RawText:        "SENSITIVE_COMMAND_MARKER completed",
		Args:           []byte(`{"kind":"completed"}`),
		IdempotencyKey: "cmd:completed:" + uuid.NewString(),
	}
	if err := commands.Create(ctx, completed); err != nil {
		t.Fatalf("create completed command: %v", err)
	}
	active := &domain.Command{
		UserID:         user.ID,
		VKPeerID:       user.VKUserID,
		InboundEventID: uuid.New(),
		Type:           domain.CommandTextAsk,
		RawText:        "SENSITIVE_COMMAND_MARKER active",
		Args:           []byte(`{"kind":"active"}`),
		IdempotencyKey: "cmd:active:" + uuid.NewString(),
	}
	if err := commands.Create(ctx, active); err != nil {
		t.Fatalf("create active command: %v", err)
	}
	completedJob := &domain.Job{
		UserID:         user.ID,
		VKPeerID:       user.VKUserID,
		CommandID:      completed.ID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusSucceeded,
		Params:         []byte(`{"prompt":"durable completed prompt"}`),
		IdempotencyKey: "job:completed:" + uuid.NewString(),
	}
	if err := jobs.Create(ctx, completedJob); err != nil {
		t.Fatalf("create completed job: %v", err)
	}
	activeJob := &domain.Job{
		UserID:         user.ID,
		VKPeerID:       user.VKUserID,
		CommandID:      active.ID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusQueued,
		Params:         []byte(`{"prompt":"durable active prompt"}`),
		IdempotencyKey: "job:active:" + uuid.NewString(),
	}
	if err := jobs.Create(ctx, activeJob); err != nil {
		t.Fatalf("create active job: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE commands
		SET created_at = $2, updated_at = $2, retention_class = $3
		WHERE id = ANY($1::uuid[])`, []uuid.UUID{completed.ID, active.ID}, old, string(domain.DataClassUserContent)); err != nil {
		t.Fatalf("age commands: %v", err)
	}

	expired, err := maintenanceRepo.ExpireCommandRawText(ctx, now.Add(-24*time.Hour), now, 10)
	if err != nil {
		t.Fatalf("expire command raw text: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired rows = %d, want 1", expired)
	}
	expired, err = maintenanceRepo.ExpireCommandRawText(ctx, now.Add(-24*time.Hour), now, 10)
	if err != nil {
		t.Fatalf("repeat expire command raw text: %v", err)
	}
	if expired != 0 {
		t.Fatalf("repeat expired rows = %d, want 0", expired)
	}

	beforeText, beforeRedacted, beforeExpired := commandRawTextState(t, ctx, pool, completed.ID)
	dryRun, err := maintenanceRepo.RetentionDryRun(ctx, now, 20)
	if err != nil {
		t.Fatalf("retention dry-run: %v", err)
	}
	var commandDryRun *domain.RetentionDryRunItem
	for i := range dryRun.Items {
		if dryRun.Items[i].TableName == "commands" {
			commandDryRun = &dryRun.Items[i]
			break
		}
	}
	if commandDryRun == nil || commandDryRun.Count < 1 {
		t.Fatalf("dry-run commands item = %+v, want count >= 1", commandDryRun)
	}
	afterDryText, afterDryRedacted, afterDryExpired := commandRawTextState(t, ctx, pool, completed.ID)
	if afterDryText != beforeText || afterDryRedacted != beforeRedacted || afterDryExpired != beforeExpired {
		t.Fatalf("dry-run mutated command row: before text=%q redacted=%v expired=%v after text=%q redacted=%v expired=%v",
			beforeText, beforeRedacted, beforeExpired, afterDryText, afterDryRedacted, afterDryExpired)
	}

	redacted, err := maintenanceRepo.RedactExpiredCommandRawText(ctx, now, 10)
	if err != nil {
		t.Fatalf("redact command raw text: %v", err)
	}
	if redacted != 1 {
		t.Fatalf("redacted rows = %d, want 1", redacted)
	}
	redacted, err = maintenanceRepo.RedactExpiredCommandRawText(ctx, now, 10)
	if err != nil {
		t.Fatalf("repeat redact command raw text: %v", err)
	}
	if redacted != 0 {
		t.Fatalf("repeat redacted rows = %d, want 0", redacted)
	}

	rawText, isRedacted, _ := commandRawTextState(t, ctx, pool, completed.ID)
	if rawText != "" || !isRedacted {
		t.Fatalf("completed command raw text = %q redacted=%v, want empty/redacted", rawText, isRedacted)
	}
	activeText, activeRedacted, activeExpired := commandRawTextState(t, ctx, pool, active.ID)
	if !strings.Contains(activeText, "SENSITIVE_COMMAND_MARKER active") || activeRedacted || activeExpired {
		t.Fatalf("active command state text=%q redacted=%v expired=%v, want still hot", activeText, activeRedacted, activeExpired)
	}
	gotCommand, err := commands.GetByIdempotencyKey(ctx, completed.IdempotencyKey)
	if err != nil {
		t.Fatalf("command idempotency lookup after redaction: %v", err)
	}
	if gotCommand.ID != completed.ID || gotCommand.RawText != "" {
		t.Fatalf("redacted command lookup = %+v, want same id and empty raw text", gotCommand)
	}
	gotJob, err := jobs.GetByID(ctx, completedJob.ID)
	if err != nil {
		t.Fatalf("job lookup after command redaction: %v", err)
	}
	if gotJob.CommandID != completed.ID || !strings.Contains(string(gotJob.Params), "durable completed prompt") {
		t.Fatalf("job relationship/params changed after command redaction: %+v params=%s", gotJob, string(gotJob.Params))
	}
}

func commandRawTextState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (string, bool, bool) {
	t.Helper()
	var rawText string
	var redacted bool
	var expired bool
	if err := pool.QueryRow(ctx, `
		SELECT raw_text, redacted_at IS NOT NULL, expires_at IS NOT NULL
		FROM commands
		WHERE id = $1`, id).Scan(&rawText, &redacted, &expired); err != nil {
		t.Fatalf("load command raw text state: %v", err)
	}
	return rawText, redacted, expired
}

func TestJobAndCommandRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users := postgres.NewUserRepository(pool)
	commands := postgres.NewCommandRepository(pool)
	jobs := postgres.NewJobRepository(pool)

	u := newTestUser(t, ctx, users)

	cmd := &domain.Command{
		UserID:         u.ID,
		VKPeerID:       u.VKUserID,
		InboundEventID: uuid.New(),
		Type:           domain.CommandImageGenerate,
		RawText:        "/image cat",
		IdempotencyKey: "cmd:" + uuid.NewString(),
	}
	if err := commands.Create(ctx, cmd); err != nil {
		t.Fatalf("create command: %v", err)
	}

	job := &domain.Job{
		UserID:         u.ID,
		VKPeerID:       u.VKUserID,
		CommandID:      cmd.ID,
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusReceived,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostEstimate:   80,
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := jobs.UpdateStatus(ctx, job.ID, domain.JobStatusReceived, domain.JobStatusValidated, "", ""); err != nil {
		t.Fatalf("update status: %v", err)
	}
	// A stale "from" status must be rejected.
	if err := jobs.UpdateStatus(ctx, job.ID, domain.JobStatusReceived, domain.JobStatusQueued, "", ""); err != domain.ErrConflict {
		t.Fatalf("expected ErrConflict on stale transition, got %v", err)
	}

	got, err := jobs.GetByIdempotencyKey(ctx, job.IdempotencyKey)
	if err != nil {
		t.Fatalf("get by idempotency: %v", err)
	}
	if got.Status != domain.JobStatusValidated {
		t.Fatalf("status = %q, want validated", got.Status)
	}
	if len(got.PricingSnapshot) != 0 {
		t.Fatalf("legacy job pricing snapshot = %s, want empty", string(got.PricingSnapshot))
	}

	list, err := jobs.ListByUser(ctx, u.ID, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 job, got %d", len(list))
	}

	snapshotJob := &domain.Job{
		UserID:          u.ID,
		VKPeerID:        u.VKUserID,
		CommandID:       cmd.ID,
		OperationType:   domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		Status:          domain.JobStatusReceived,
		IdempotencyKey:  "job:" + uuid.NewString(),
		PricingSnapshot: []byte(`{"internal_credits":33}`),
		CostEstimate:    33,
		CostReserved:    33,
	}
	if err := jobs.Create(ctx, snapshotJob); err != nil {
		t.Fatalf("create snapshot job: %v", err)
	}
	gotSnapshotJob, err := jobs.GetByID(ctx, snapshotJob.ID)
	if err != nil {
		t.Fatalf("get snapshot job: %v", err)
	}
	if credits, ok := gotSnapshotJob.PricingSnapshotCredits(); !ok || credits != 33 {
		t.Fatalf("snapshot job credits = %d/%v, want 33/true; snapshot=%s", credits, ok, string(gotSnapshotJob.PricingSnapshot))
	}

	if _, err := commands.GetByIdempotencyKey(ctx, cmd.IdempotencyKey); err != nil {
		t.Fatalf("command get by idempotency: %v", err)
	}
}

func TestProviderTaskRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users := postgres.NewUserRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	tasks := postgres.NewProviderTaskRepository(pool)

	u := newTestUser(t, ctx, users)

	// jobs.command_id references commands(id), so create the command first.
	commands := postgres.NewCommandRepository(pool)
	cmd := &domain.Command{UserID: u.ID, VKPeerID: u.VKUserID, InboundEventID: uuid.New(), Type: domain.CommandVideoGenerate, IdempotencyKey: "cmd:" + uuid.NewString()}
	if err := commands.Create(ctx, cmd); err != nil {
		t.Fatalf("create command: %v", err)
	}
	job := &domain.Job{
		UserID: u.ID, VKPeerID: u.VKUserID, CommandID: cmd.ID,
		OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo,
		Status: domain.JobStatusReceived, IdempotencyKey: "job:" + uuid.NewString(),
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	task := &domain.ProviderTask{
		JobID:          job.ID,
		Provider:       domain.ProviderKling,
		ModelCode:      "kling-v2",
		Status:         domain.ProviderTaskPending,
		Request:        []byte(`{"prompt":"tokyo"}`),
		IdempotencyKey: "ptask:" + uuid.NewString(),
	}
	if err := tasks.Create(ctx, task); err != nil {
		t.Fatalf("create provider task: %v", err)
	}

	task.ExternalID = "ext-123"
	task.Status = domain.ProviderTaskSucceeded
	now := time.Now()
	task.CompletedAt = &now
	task.Result = []byte(`{"output_urls":["https://x/y.mp4"]}`)
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatalf("update provider task: %v", err)
	}

	byExt, err := tasks.GetByExternalID(ctx, domain.ProviderKling, "ext-123")
	if err != nil {
		t.Fatalf("get by external id: %v", err)
	}
	if byExt.ID != task.ID {
		t.Fatal("provider task id mismatch")
	}

	all, err := tasks.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list by job: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 task, got %d", len(all))
	}
}

func TestBillingRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users := postgres.NewUserRepository(pool)
	commands := postgres.NewCommandRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	billing := postgres.NewBillingRepository(pool)

	u := newTestUser(t, ctx, users)
	acc := &domain.CreditAccount{UserID: u.ID, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := billing.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("create account: %v", err)
	}

	cmd := &domain.Command{UserID: u.ID, VKPeerID: u.VKUserID, InboundEventID: uuid.New(), Type: domain.CommandVideoGenerate, IdempotencyKey: "cmd:" + uuid.NewString()}
	if err := commands.Create(ctx, cmd); err != nil {
		t.Fatalf("create command: %v", err)
	}
	job := &domain.Job{UserID: u.ID, VKPeerID: u.VKUserID, CommandID: cmd.ID, OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Status: domain.JobStatusReceived, IdempotencyKey: "job:" + uuid.NewString()}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	res := &domain.CreditReservation{
		AccountID:      acc.ID,
		JobID:          job.ID,
		Amount:         80,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := billing.Reserve(ctx, res); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Available balance is now 20, so a 30-credit reservation must fail.
	tooMuch := &domain.CreditReservation{
		AccountID: acc.ID, JobID: job.ID, Amount: 30,
		IdempotencyKey: "resv:" + uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := billing.Reserve(ctx, tooMuch); err != domain.ErrInsufficientCredits {
		t.Fatalf("expected ErrInsufficientCredits, got %v", err)
	}

	if err := billing.Capture(ctx, res.ID, 80, "cap:"+uuid.NewString()); err != nil {
		t.Fatalf("capture: %v", err)
	}

	after, err := billing.GetAccount(ctx, acc.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if after.BalanceCached != 20 {
		t.Fatalf("balance = %d, want 20", after.BalanceCached)
	}

	captured, err := billing.GetReservation(ctx, res.ID)
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	if captured.Status != domain.ReservationCaptured {
		t.Fatalf("reservation status = %q, want captured", captured.Status)
	}

	entries, err := billing.ListEntries(ctx, acc.ID, 10, 0)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected at least reserve + capture entries, got %d", len(entries))
	}
}

func TestBillingRepositoryDuplicateAppendEntryDoesNotAbortTransaction(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users := postgres.NewUserRepository(pool)
	billing := postgres.NewBillingRepository(pool)

	u := newTestUser(t, ctx, users)
	acc := &domain.CreditAccount{UserID: u.ID, Currency: domain.CurrencyCredits}
	if err := billing.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("create account: %v", err)
	}

	duplicateKey := "topup:" + uuid.NewString()
	if err := billing.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      acc.ID,
		Type:           domain.LedgerTopup,
		Amount:         100,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: duplicateKey,
		Reason:         "initial topup",
	}); err != nil {
		t.Fatalf("append initial topup: %v", err)
	}

	if err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		txBilling := postgres.NewBillingRepositoryTx(tx)
		if err := txBilling.AppendEntry(ctx, &domain.LedgerEntry{
			AccountID:      acc.ID,
			Type:           domain.LedgerTopup,
			Amount:         100,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: duplicateKey,
			Reason:         "duplicate topup",
		}); err != nil {
			return err
		}
		return txBilling.AppendEntry(ctx, &domain.LedgerEntry{
			AccountID:      acc.ID,
			Type:           domain.LedgerAdjustment,
			Amount:         1,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: "adjust:" + uuid.NewString(),
			Reason:         "post-duplicate write",
		})
	}); err != nil {
		t.Fatalf("duplicate append in transaction: %v", err)
	}

	after, err := billing.GetAccount(ctx, acc.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if after.BalanceCached != 101 {
		t.Fatalf("balance = %d, want 101", after.BalanceCached)
	}
}

func TestOutboxRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	outbox := postgres.NewOutboxRepository(pool)

	e := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     "event.job.created",
		Payload:       []byte(`{"hello":"world"}`),
	}
	if err := outbox.Add(ctx, e); err != nil {
		t.Fatalf("add: %v", err)
	}

	pending, err := outbox.FetchPending(ctx, 50)
	if err != nil {
		t.Fatalf("fetch pending: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.ID == e.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected new event in pending set")
	}

	if err := outbox.MarkPublished(ctx, e.ID, time.Now()); err != nil {
		t.Fatalf("mark published: %v", err)
	}
}

func TestIdempotencyRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := postgres.NewIdempotencyRepository(pool)

	key := "vk_event:" + uuid.NewString()
	rec := &domain.IdempotencyRecord{Key: key, Scope: "inbound_event", ResourceType: "command"}

	first, created, err := repo.GetOrCreate(ctx, rec)
	if err != nil {
		t.Fatalf("get or create: %v", err)
	}
	if !created {
		t.Fatal("expected first call to create")
	}
	if first.Status != domain.IdempotencyStarted {
		t.Fatalf("status = %q, want started", first.Status)
	}

	again := &domain.IdempotencyRecord{Key: key, Scope: "inbound_event", ResourceType: "command"}
	_, created2, err := repo.GetOrCreate(ctx, again)
	if err != nil {
		t.Fatalf("get or create (2): %v", err)
	}
	if created2 {
		t.Fatal("expected second call to NOT create")
	}

	resourceID := uuid.New()
	if err := repo.MarkCompleted(ctx, key, resourceID); err != nil {
		t.Fatalf("mark completed: %v", err)
	}
	done, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if done.Status != domain.IdempotencyCompleted {
		t.Fatalf("status = %q, want completed", done.Status)
	}
	if done.ResourceID == nil || *done.ResourceID != resourceID {
		t.Fatal("resource id not persisted")
	}
}

func TestArtifactAndDeliveryRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users := postgres.NewUserRepository(pool)
	commands := postgres.NewCommandRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	artifacts := postgres.NewArtifactRepository(pool)
	deliveries := postgres.NewDeliveryRepository(pool)

	u := newTestUser(t, ctx, users)
	cmd := &domain.Command{UserID: u.ID, VKPeerID: u.VKUserID, InboundEventID: uuid.New(), Type: domain.CommandImageGenerate, IdempotencyKey: "cmd:" + uuid.NewString()}
	if err := commands.Create(ctx, cmd); err != nil {
		t.Fatalf("create command: %v", err)
	}
	job := &domain.Job{UserID: u.ID, VKPeerID: u.VKUserID, CommandID: cmd.ID, OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusReceived, IdempotencyKey: "job:" + uuid.NewString()}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	art := &domain.Artifact{
		OwnerUserID:   u.ID,
		JobID:         &job.ID,
		Kind:          domain.ArtifactKindOutput,
		MediaType:     domain.MediaTypeImage,
		MimeType:      "image/png",
		StorageBucket: "artifacts",
		StorageKey:    "u/" + uuid.NewString() + ".png",
		SHA256:        uuid.NewString(),
		SizeBytes:     2048,
		Width:         1024,
		Height:        1024,
		Codec:         " H.264 ",
		Container:     "PNG ",
		BitrateBPS:    1200000,
		ProbeStatus:   domain.MediaProbePassed,
		Status:        domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, art); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	variant := &domain.ArtifactVariant{
		ArtifactID:  art.ID,
		VariantType: domain.VariantVKPhoto,
		StorageKey:  "u/vk_" + uuid.NewString() + ".jpg",
		MimeType:    "image/jpeg",
		SizeBytes:   1024,
		Codec:       "MJPEG",
		Container:   "JPG",
		ProbeStatus: domain.MediaProbeSkipped,
	}
	if err := artifacts.AddVariant(ctx, variant); err != nil {
		t.Fatalf("add variant: %v", err)
	}
	vlist, err := artifacts.ListVariants(ctx, art.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(vlist) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(vlist))
	}
	if vlist[0].Codec != "mjpeg" || vlist[0].Container != "jpg" || vlist[0].ProbeStatus != domain.MediaProbeSkipped {
		t.Fatalf("variant metadata not stored: %+v", vlist[0])
	}

	bySHA, err := artifacts.GetBySHA256(ctx, u.ID, art.SHA256)
	if err != nil {
		t.Fatalf("get by sha256: %v", err)
	}
	if bySHA.ID != art.ID {
		t.Fatal("artifact id mismatch on sha lookup")
	}
	if bySHA.Codec != "h.264" || bySHA.Container != "png" || bySHA.BitrateBPS != 1200000 || bySHA.ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("artifact metadata not stored: %+v", bySHA)
	}

	ref := &domain.Artifact{
		OwnerUserID:             u.ID,
		Kind:                    domain.ArtifactKindInput,
		MediaType:               domain.MediaTypeImage,
		MimeType:                "image/png",
		StorageBucket:           "artifacts",
		StorageKey:              "u/ref_" + uuid.NewString() + ".png",
		SHA256:                  art.SHA256,
		ValidationPolicyVersion: "image_reference_v1",
		LifecycleClass:          domain.ArtifactLifecycleInputReference,
		SizeBytes:               2048,
		Status:                  domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, ref); err != nil {
		t.Fatalf("create reference artifact: %v", err)
	}
	reusable, err := artifacts.FindReusableInputReference(ctx, u.ID, art.SHA256, "image_reference_v1", "image/png")
	if err != nil {
		t.Fatalf("find reusable reference: %v", err)
	}
	if reusable.ID != ref.ID {
		t.Fatalf("reusable reference id = %s, want %s", reusable.ID, ref.ID)
	}

	del := &domain.Delivery{
		JobID:          job.ID,
		UserID:         u.ID,
		VKPeerID:       u.VKUserID,
		ArtifactID:     &art.ID,
		Type:           domain.DeliveryTypePhoto,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     uniqueVKID(),
		Attachment:     "photo1_2",
		IdempotencyKey: "del:" + uuid.NewString(),
	}
	if err := deliveries.Create(ctx, del); err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	msgID := int64(555)
	del.Status = domain.DeliveryStatusSent
	del.VKMessageID = &msgID
	if err := deliveries.Update(ctx, del); err != nil {
		t.Fatalf("update delivery: %v", err)
	}

	dlist, err := deliveries.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(dlist) != 1 || dlist[0].Status != domain.DeliveryStatusSent {
		t.Fatalf("unexpected deliveries: %+v", dlist)
	}
}
