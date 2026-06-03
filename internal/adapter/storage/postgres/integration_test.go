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
	for _, name := range []string{"000001_init_schema.down.sql", "000001_init_schema.up.sql"} {
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

	list, err := jobs.ListByUser(ctx, u.ID, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 job, got %d", len(list))
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

	bySHA, err := artifacts.GetBySHA256(ctx, u.ID, art.SHA256)
	if err != nil {
		t.Fatalf("get by sha256: %v", err)
	}
	if bySHA.ID != art.ID {
		t.Fatal("artifact id mismatch on sha lookup")
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
