# DeepSeek Conversation Titles — Implementation Plan

> **For implementation:** Use the subagent-driven development workflow. Each task starts with a failing focused test, makes the smallest production change, runs the task's verification commands, and commits only its own files. Preserve the user's existing dirty planning/specification files.

**Goal:** Give each new active Web conversation a short semantic title generated from its first user message through the existing cheap DeepSeek V4 Flash route, without affecting chat latency, user billing, or manually chosen titles.

**Architecture:** The browser immediately displays a deterministic fallback. The normal Web chat job emits a second, independent transactional outbox event only when its conversation is still title-eligible. The relay publishes that event to a dedicated Redis stream. A title-only worker waits for the first persisted user message, asks DeepInfra/DeepSeek in the background, and conditionally writes the semantic title. The active page makes at most five exact-row GETs to reconcile that one entry in the local sidebar cache.

**Runtime / test baseline:** Go backend with PostgreSQL, Redis Streams, worker processes; Next.js/React/TypeScript web app; Go tests via `go test`; web checks via the existing `web/platform` scripts.

## Non-negotiable constraints

- No model call in an HTTP handler, React render, or user chat generation path.
- No prompt in outbox payloads, Redis tasks, logs, metrics, tracing attributes, or public DTOs.
- Title work creates no visible Job, artifact, payment, billing reservation, delivery, or user-star charge.
- Only `web`, active, account-owned conversations participate. VK Bot and Mini App keep their current title behavior.
- A manual rename is authoritative even if DeepSeek responds after it.
- The title event and Redis stream are independent from `event.job.queued` / `stream:jobs:text`: an outage must never delay or duplicate a normal chat response.
- Browser reconciliation is account-scoped, one request at a time, bounded to 1/2/4/8/15 seconds, and never calls `router.refresh()` or refetches the full conversation list.

## Task 1 — Durable title state, fallback, and safe exact conversation API

**Files:**
- Create: `migrations/000050_conversation_title_origin.up.sql`
- Create: `migrations/000050_conversation_title_origin.down.sql`
- Modify: `internal/domain/conversation.go`
- Modify: `internal/domain/repositories.go`
- Modify: `internal/adapter/storage/postgres/conversation.go`
- Modify: `internal/adapter/storage/memory/conversation.go`
- Modify: `internal/service/dialogcontext/service.go`
- Modify: `internal/service/dialogcontext/service_test.go`
- Modify: `internal/adapter/storage/postgres/conversation_management_test.go`
- Modify: `internal/adapter/storage/memory/conversation_test.go`
- Modify: `internal/adapter/inbound/websession/handler.go`
- Modify: `internal/adapter/inbound/websession/handler_test.go`

**Implementation:**

1. Add private `ConversationTitleOrigin` values `manual`, `auto_pending`, `auto_fallback`, and `auto_generated` to `domain.Conversation`; validate/default legacy sources to `manual`.
2. Add `title_origin` to `conversations`. Backfill only blank existing Web conversations to `auto_pending`; all other historical conversations become `manual`. The down migration removes the column and its constraint.
3. Extend repository scanning/insertion and the in-memory repository. New Web conversations created by `POST /web/v1/conversations` start as `auto_pending`.
4. Add narrowly scoped repository operations:
   - resolve the one user message for a job (`job_id`, `role=user`), using the existing unique key;
   - fetch the earliest user message for one conversation;
   - atomically persist a fallback only from `auto_pending` to `auto_fallback`;
   - atomically persist a generated title only for the exact active account-owned Web conversation whose origin is `auto_pending` or `auto_fallback`.
5. Keep `SetConversationTitleIfEmpty` intact for Mini App. Change Web handling in `dialogcontext.Prepare` to use the new fallback-only transition, so a late normal worker can never replace a generated title.
6. Make `RenameActiveConversationForAccount` set `title_origin=manual` in the same update.
7. Add `GET /web/v1/conversations/{conversationID}`. It must use `GetByIDForAccount`, require `web` + `active`, return only the existing strict `safeConversation` fields, and set `Cache-Control: no-store`.

**Tests first:**

```go
func TestSetGeneratedTitleDoesNotOverwriteManualRename(t *testing.T) { /* conditional update returns false */ }
func TestPrepareSetsWebFallbackButKeepsMiniAppTitleSemantics(t *testing.T) { /* web auto_fallback; Mini App unchanged */ }
func TestGetConversationRejectsForeignArchivedAndNonWebRows(t *testing.T) { /* all return 404 */ }
```

**Verification:**

```powershell
go test ./internal/service/dialogcontext ./internal/adapter/storage/memory ./internal/adapter/storage/postgres ./internal/adapter/inbound/websession
```

## Task 2 — Independent durable title outbox event and Redis stream

**Files:**
- Modify: `internal/service/joborchestrator/orchestrator.go`
- Modify: `internal/service/joborchestrator/orchestrator_test.go`
- Modify: `internal/adapter/inbound/websession/handler.go`
- Modify: `internal/adapter/inbound/websession/conversation_message_test.go`
- Modify: `internal/service/outboxrelay/relay.go`
- Modify: `internal/service/outboxrelay/relay_test.go`
- Modify: `internal/adapter/queue/redis/streams.go`
- Modify: `internal/adapter/queue/redis/streams_test.go`
- Modify: `internal/worker/engine.go`

**Implementation:**

1. Add an internal-only `ConversationTitleRequested` input flag to `joborchestrator.CreateJobInput`. It is valid only for queued Web `text_generate` / `text` work.
2. In the Web message handler, inspect the trusted active conversation before creating the job; set the flag only while its origin is `auto_pending`. This means normal conversations create one small title event, not one title task per message. Concurrent first sends may emit duplicates; downstream first-message and compare-and-set checks make that harmless.
3. In the same job/outbox transaction, append `event.conversation_title.queued` after the ordinary job event whenever the internal flag is accepted. Its payload has only job/operation/modality/account-safe source/correlation/trace metadata—never params or prompt.
4. Extend the relay classification and strict envelope validation. Publish the new event only to `stream:conversations:title`. It must not call `Enqueue` for a normal job, so a title-queue Redis failure retries only the title event.
5. Add the stream to `AllStreams` and `AllStreamsWithDLQ`, classify it as `conversation_title` in worker/queue metrics, and retain existing low-cardinality/no-payload metric behavior.

**Tests first:**

```go
func TestWebFirstMessageCreatesIndependentConversationTitleEvent(t *testing.T) { /* normal + title outbox rows */ }
func TestTitleEventPublishesOnlyToConversationTitleStream(t *testing.T) { /* no normal queue task */ }
func TestTitleStreamPublishFailureDoesNotRepublishNormalGeneration(t *testing.T) { /* separate outbox recovery */ }
func TestTitleEventPayloadContainsNoPromptOrParams(t *testing.T) { /* payload allowlist */ }
```

**Verification:**

```powershell
go test ./internal/service/joborchestrator ./internal/adapter/inbound/websession ./internal/service/outboxrelay ./internal/adapter/queue/redis ./internal/worker
```

## Task 3 — DeepSeek title generator and title-only worker

**Files:**
- Create: `internal/service/conversationtitle/service.go`
- Create: `internal/service/conversationtitle/service_test.go`
- Create: `internal/worker/conversation_title.go`
- Create: `internal/worker/conversation_title_test.go`
- Modify: `internal/adapter/provider/deepinfra/deepinfra.go`
- Modify: `internal/adapter/provider/deepinfra/deepinfra_test.go`
- Modify: `cmd/worker/main.go`
- Modify: `internal/worker/engine.go` (only if a title-specific retry/phase hook is needed)

**Implementation:**

1. Define `conversationtitle.Generator`, with no dependency on normal user job processing. Implement it on `deepinfra.Provider` using the configured `deepseek-ai/DeepSeek-V4-Flash` text model, a title-only trusted system message, first prompt as a user message, `max_tokens=32`, and a stable provider idempotency key `conversation-title:<conversation-id>`.
2. Normalize provider output: trim, collapse whitespace/newlines, strip wrapping quotes, reject an empty response, and cap at 80 Unicode runes. Limit the untrusted prompt supplied to the title model to a bounded size.
3. Create `conversationtitle.Service.Process(task)`:
   - validate its persisted job is account-owned Web text work;
   - wait only for the normal worker to persist the matching user message (return retryable error only for this short first-message race);
   - ensure this job is the earliest user message of the conversation;
   - preserve the fallback, invoke DeepSeek outside a DB transaction, and conditionally write `auto_generated`;
   - treat malformed/terminal/provider-failed title output as a successful no-op with the fallback left in place, so an external model outage cannot build an unbounded pending list;
   - never mutate the parent job's status, billing, artifacts, or deliveries.
4. Wire one dedicated `worker.Engine` to `StreamConversationTitle`, with a short title-only reclaim idle period for the initial persistence race. If no DeepInfra generator is configured, acknowledge title work and keep the fallback; do not block normal worker startup.

**Tests first:**

```go
func TestTitleWorkerWaitsForFirstMessageThenGeneratesOnce(t *testing.T) { /* initial retry, then semantic title */ }
func TestTitleWorkerIgnoresLaterMessageAndManualRename(t *testing.T) { /* no provider call / no overwrite */ }
func TestTitleWorkerLeavesFallbackOnProviderOrMalformedOutput(t *testing.T) { /* ACK, no billing/job mutation */ }
func TestDeepInfraTitleRequestUsesDedicatedPromptAndTokenCap(t *testing.T) { /* no normal assistant prompt */ }
```

**Verification:**

```powershell
go test ./internal/service/conversationtitle ./internal/worker ./internal/adapter/provider/deepinfra ./cmd/worker
```

## Task 4 — Immediate fallback and bounded sidebar reconciliation in React

**Files:**
- Create: `web/platform/src/features/conversations/ConversationTitleSync/ConversationTitleSync.tsx`
- Create: `web/platform/src/features/conversations/ConversationTitleSync/ConversationTitleSync.test.tsx`
- Create: `web/platform/src/features/conversations/conversation-title.ts`
- Modify: `web/platform/src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.tsx`
- Create or modify: `web/platform/src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/lib/web-api/contracts.ts`
- Modify: `web/platform/src/lib/web-api/contracts.test.ts`

**Implementation:**

1. Add one strict `parseConversationItem` parser for the new GET response.
2. Add a shared, Unicode-safe fallback title helper matching the backend truncation rules.
3. Extend the conversation-list context with an in-place `replaceConversation` / `updateConversationTitle` operation. It must preserve the array order and return the prior state if nothing changed—semantic replacement must not make the sidebar jump.
4. On a successful first prompt from the workspace or a blank new chat composer, immediately upsert the fallback title and store a small local title-sync marker. Do not wait for DeepSeek before navigating.
5. Mount an invisible `ConversationTitleSync` only for the active new conversation. It calls `GET /web/v1/conversations/{id}` sequentially after 1, 2, 4, 8, 15 seconds, one request at a time. It uses `AbortController` and cleanup; it stops on semantic/manual title, 401, 404, unmount, account change, or the fifth result.
6. The sync performs no `router.refresh`, no full conversation-list request, and no visible error state. It only replaces the one cached list item when the server title differs from the fallback.

**Tests first:**

```tsx
it("shows the fallback in the sidebar before navigation", async () => { /* empty create DTO */ });
it("replaces exactly one title without moving the row", () => { /* list order is stable */ });
it("uses at most five sequential exact-row requests and aborts on unmount", async () => { /* fake timers */ });
it("does not poll an unrelated or manually named conversation", async () => { /* no global refresh */ });
```

**Verification:**

```powershell
Set-Location web/platform
npm test -- --runInBand
npm run typecheck
npm run lint
```

## Task 5 — Operational observability, integration verification, and DEV rollout

**Files:**
- Modify as applicable after discovery: `docs/LOAD_TESTING.md`, `scripts/loadtest/redis-diagnostics.ps1`, `scripts/deploy/observe-prod.ps1`, `scripts/deploy/observe-prod.sh`
- Modify: `docs/superpowers/specs/2026-08-02-deepseek-conversation-title-design.md` only if implementation naming differs from the approved design.

**Implementation:**

1. Add the title stream to any manually enumerated safe queue diagnostics/observability tooling, retaining count-only/no-payload behavior.
2. Verify the full set of separate title-worker metrics and stream trimming uses the built-in `AllStreams` lists.
3. Run the complete backend/frontend quality gates, inspect the aggregate diff for prompt leakage and unrelated edits, commit only feature files, push `dev-deploy`, wait for CI/image success, dispatch the existing DEV deployment, and smoke the Basic Auth protected DEV host.

**Verification:**

```powershell
go test ./...
Set-Location web/platform
npm test -- --runInBand
npm run typecheck
npm run lint
npm run build
```

**Rollout acceptance:**

- Start a new Web chat: fallback title appears immediately, then a semantic short title may replace it without page/sidebar refresh.
- Rename before DeepSeek returns: the manual name stays unchanged.
- Temporarily fail DeepInfra: user chat still answers normally and the fallback remains.
- Redis title-stream failure: normal text `event.job.queued` has exactly its normal publication/retry path.
- Queue metrics include `stream:conversations:title`; no request payload/prompt is visible in logs/metrics/stream tasks.
