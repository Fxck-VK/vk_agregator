# Channel-Neutral Result Delivery Implementation Plan

> **For agentic workers:** implement task by task; use red → green tests, preserve VK Bot and Mini App compatibility, and leave every reviewed change uncommitted.

**Goal:** Let a canonical Account activate a prepared web Job, receive a safe result through account history, and be charged only after the result is durably available—without a synthetic VK user/peer or a second backend.

**Architecture:** Transport is explicit data, not authorization. `AccountID` remains the sole owner. A persisted `ChannelContext` records bounded, opaque origin metadata, and `ResultMode` decides finalization: `external_push` writes a delivery attempt and calls a channel publisher; `account_history` makes the result usable only through an owner-checked result boundary. Existing `UserID`, `VKPeerID`, and VK delivery columns remain nullable compatibility provenance during rollout.

**Tech Stack:** Go, PostgreSQL/pgx, Redis streams, transactional outbox, in-memory repositories, Go tests.

## Non-Negotiable Invariants

- No new web flow fabricates a legacy User, VK peer, or payment/billing owner.
- `AccountID` is the authorization and business owner. Channel references are never authorization input.
- A prepared job reserves nothing and enters no worker queue. Activation reserves and queues in one UOW.
- Browser polling, download, or acknowledgement never controls capture.
- An account-history result is visible only after its job is succeeded, its output artifact is ready, linked, owned by the caller, and moderation-safe.
- A VK push is captured only after its publisher confirms delivery. A publish failure captures nothing.
- Existing VK Bot and Mini App contracts remain functional throughout migration; unknown legacy routing fails closed rather than falling through to VK.
- Migrations are additive and forward-safe. Do not drop legacy columns, rewrite money data, or run a destructive down migration.

## Contract

### Channel provenance

`domain.ChannelContext` contains a bounded `Channel` plus opaque `RecipientRef` and `ThreadRef`. It is provenance only. Supported values are initially `vk_bot`, `vk_miniapp`, and `web`; no client-provided value is trusted by service code.

### Result mode and delivery target

`domain.ResultMode` is one of:

- `external_push`: requires a valid `DeliveryTarget`; VK Bot is the initial publisher.
- `account_history`: requires a canonical `AccountID` and intentionally has no target. This is used by Web and Mini App.
- `legacy_unknown`: compatibility-only, never used by a new write and fails closed in finalization until reconciled.

`DeliveryTarget` uses the same channel/ref shape but is separate from origin context. It prevents an absent target from being mistaken for an uninitialized VK job.

## Task 1: Persist Channel Context and Result Mode

**Files:**

- Create: `migrations/000044_channel_context_and_result_mode.up.sql`
- Create: `migrations/000044_channel_context_and_result_mode.down.sql`
- Create: `internal/domain/channel.go`
- Modify: `internal/domain/job.go`, `internal/domain/delivery.go`, `internal/domain/repositories.go`
- Modify: `internal/adapter/storage/postgres/{job,delivery}.go`
- Modify: `internal/adapter/storage/memory/{job,delivery}.go`
- Test: focused domain, memory, PostgreSQL binding/scan and migration-chain tests.

- [ ] Add `Channel`, `ChannelContext`, `DeliveryTarget`, and `ResultMode` with validation helpers. A new account-history job must have a non-nil AccountID and no target; a push job must have a valid target.
- [ ] Add nullable `jobs` channel/target fields and a non-null result-mode field with `legacy_unknown` default. Backfill only known legacy source families (`vk*` → `external_push`, `miniapp` → `account_history`, `web` → `account_history`); preserve all other rows as `legacy_unknown`.
- [ ] Add `deliveries.account_id`, channel/target fields; make legacy `user_id`, `vk_peer_id`, and `vk_random_id` nullable. Existing VK rows remain valid; account-history finalization creates no delivery row.
- [ ] Add `NOT VALID` shape checks and partial indexes. The down migration removes only new constraints/indexes, never canonical ownership data or legacy data.
- [ ] Make PostgreSQL scans/bindings use zero values only for absent legacy provenance. Memory repositories mirror exact idempotency and channel shapes.
- [ ] Run focused tests plus full migration-chain compile/integration test (skip only when `TEST_DATABASE_URL` is absent).

## Task 2: Explicit Adapter Contract and Atomic Prepared-Job Activation

**Files:**

- Modify: `internal/service/joborchestrator/orchestrator.go`
- Modify: `internal/adapter/inbound/vk/handler.go`
- Modify: `internal/adapter/inbound/miniapp/handler.go`
- Modify: job storage if needed for exact owner/status replay
- Test: `internal/service/joborchestrator/*account*activation*_test.go` and adapter regression tests.

- [ ] Extend trusted create input with explicit channel context/result mode/target. VK Bot dual-writes a VK push target; Mini App writes account-history; Web preparation writes account-history. Existing compatibility callers may use a bounded server-side inference only while migration is enabled.
- [ ] Add `ActivatePreparedAccountJob(ctx, accountID, jobID)`. Inside one UOW, read by exact AccountID, lock state through expected transitions, recheck capacity from immutable persisted facts, reserve through a new transaction-bound `ReserveForAccountWith(ctx, repos.Billing, accountID, jobID, amount)` contract, transition to queued, and write exactly one `event.job.queued` outbox record.
- [ ] Treat queued/downstream activation replay as a stable read. A foreign account returns `ErrNotFound`. Insufficient funds persists `awaiting_payment`, writes no queue event, and keeps the job resumable. A prepared job still cannot enter workers before activation.
- [ ] Prove concurrent activation has one reservation and one queue event, including a PostgreSQL transaction race when a test database is available.

## Task 3: Durable Result-Ready Scheduling

**Files:**

- Modify: `internal/worker/{worker,generation,poll}.go`
- Modify: `internal/service/outboxrelay/*`
- Modify: queue publisher/relay contracts and their memory/Redis tests
- Test: worker and relay recovery tests.

- [ ] Persist `event.job.result_ready` in the same durable state transition that records `result_ready`, using a required worker UOW; do not rely solely on an in-memory stream publish after the status update.
- [ ] Make relay replay schedule finalization idempotently through a small `PublishTo(stream, task)` capability: queued events use normal generation enqueue, result-ready events use the existing delivery stream. Keep that stream name during rollout to avoid stranding production messages.
- [ ] Preserve `event.job.created` as audit-only and `event.job.queued` as the only generation enqueue event.
- [ ] Prove a post-commit crash/replay cannot lose a finalization task or perform capture twice.

## Task 4: Neutral Finalizer and VK Publisher Port

**Files:**

- Modify: `internal/worker/delivery.go`
- Create/modify: `internal/adapter/delivery/vk/publisher.go`
- Modify: worker wiring in `cmd/worker/main.go`
- Test: `internal/worker/delivery_test.go`, `internal/adapter/delivery/vk/*_test.go`.

- [ ] Switch finalization on persisted `ResultMode`, never `Source` or `VKPeerID` alone.
- [ ] For `account_history`, verify the ready output artifact, capture idempotently, then transition `result_ready → succeeded`; make no VK call and create no delivery row.
- [ ] For `external_push`, route through a publisher port. Move VK target validation, deterministic random id, upload, formatting, and replay validation behind the VK adapter. Capture only after confirmed publication.
- [ ] A capture retry after a successful push must not send twice. An invalid/unknown target fails closed and captures nothing. Preserve existing bounded failure/DLQ/release behavior for VK.
- [ ] Allow `result_ready → succeeded` in the state machine while preserving `result_ready → delivering → succeeded` for push.

## Task 5: Owner-Safe Result Retrieval Boundary

**Files:**

- Create: `internal/service/resultservice/service.go`
- Test: `internal/service/resultservice/service_test.go`
- Later integration only: web-session handler route DTOs.

- [ ] Expose account-scoped job result/history reads with no raw storage fields or provider payloads.
- [ ] Require exact owner, `JobStatusSucceeded`, output linkage, ready artifact status, supported output kind, and safe moderation before returning a result DTO; foreign/unready/unlinked/failed data returns `ErrNotFound`.
- [ ] Do not add a public `/web/v1` endpoint in this task unless the cookie principal and safe DTO policy are wired in the same reviewed change.

## Task 6: Compatibility, Rollout, and Verification

- [ ] Run domain, joborchestrator, outbox relay, resultservice, worker, VK delivery, storage, Mini App, and VK handler regression suites; then `go test ./... -count=1`, `go vet ./...`, and `git diff --check`.
- [ ] Deployment order: migrate → deploy dual-read/write with web activation disabled → reconcile unknown modes/targets → replace/drain old workers → enable neutral finalization → canary VK/Mini App → enable web activation.
- [ ] An old worker must never coexist with activated targetless web jobs; it would interpret them as VK deliveries. Keep the feature disabled until worker replacement is verified.
