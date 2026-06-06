# PR-18 Durable Shared Chat Core - Copy/Paste Prompts

Use these blocks one by one. Do not start a later PR until the previous PR is
merged or explicitly accepted.

---

## PR-18.1 - Durable conversation identity foundation

```text
MODE: IMPLEMENT

Task:
PR-18.1: extend durable conversation identity so VK bot and Mini App chat can
share the same backend conversation core without mixing contexts. Do not switch
Mini App runtime behavior yet.

Target branch: fastlife_dev

Language:
Answer in Russian. File names, functions, env, commands and commit message in
English.

Context:
VK text bot already has durable context through `internal/service/dialogcontext`
and Postgres conversations. Mini App chat currently has `conversation_id`, but
recent turns are also kept in process-local
`internal/adapter/inbound/miniapp/conversation.go`. The goal is to prepare a
safe durable identity model before switching Mini App.

Allowed scope:
- migrations/**
- internal/domain/conversation.go
- internal/domain/repositories.go
- internal/adapter/storage/postgres/conversation.go
- internal/adapter/storage/memory/conversation.go
- internal/service/dialogcontext/** tests only if needed for compile
- PROGRESS.md
- TASKS.md / DECISIONS.md / docs/CHAT_CONTEXT_REFACTOR_LOG.md if needed

Do not change:
- VK bot behavior
- Mini App BFF behavior
- worker provider calls
- billing logic
- frontend

Safety:
- No provider calls outside worker.
- No billing mutation changes.
- Do not make `conversation_id` trusted identity by itself.
- Existing VK bot conversations must continue to resolve by `user_id + vk_peer_id`.
- Mini App future threads must be scoped by backend user ownership.
- No prompts/PII/launch params in logs.

STEP 0 - reconnaissance without code:
1. Read current conversation migration/domain/repositories.
2. Confirm current unique key is `user_id + vk_peer_id`.
3. Report the exact schema change plan before editing.

Implementation:
1. Add conversation source identity:
   - `source`: `vk_bot` or `miniapp`.
   - `external_thread_id`: opaque string for Mini App threads.
2. Backfill existing rows as `source='vk_bot'`.
3. Preserve VK bot lookup by `user_id + vk_peer_id`.
4. Add repository methods for explicit references, for example:
   - `GetActiveByReference(ctx, ConversationRef)`
   - `ListByUserSource(ctx, userID, source, limit, offset)`
   - `GetByIDForUser(ctx, userID, conversationID)`
5. Add/adjust memory and Postgres tests.
6. Add indexes:
   - active VK bot conversation by `user_id, vk_peer_id`
   - active Mini App thread by `user_id, source, external_thread_id`
   - list by `user_id, source, updated_at DESC`
7. Update docs/log.

Checks:
- gofmt -w <changed Go files>
- go test ./internal/adapter/storage/postgres ./internal/adapter/storage/memory ./internal/service/dialogcontext
- go test ./...
- go build ./...

If green:
git add <only PR-18.1 files explicitly>
git commit -m "chat: add durable conversation identity"
git push origin fastlife_dev
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Existing VK bot context lookup remains backward compatible.
- [ ] Durable model can represent Mini App threads without `vk_peer_id` collisions.
- [ ] No runtime behavior switch yet.
- [ ] Tests/build green.

Final response format:
1. PR-18.1 status: completed / blocked
2. STEP 0 summary
3. Schema/domain changes
4. Backward compatibility
5. Checks
6. Commit/push + SHA
7. Security notes
```

---

## PR-18.2 - Worker dialogcontext explicit references and shared chat contract

```text
MODE: IMPLEMENT

Task:
PR-18.2: teach worker/dialogcontext to use explicit durable conversation
references from job params, with VK bot fallback preserved. Introduce a shared
chat job contract/facade if it reduces duplication, but do not switch Mini App
yet.

Target branch: fastlife_dev

Language:
Answer in Russian. File names, functions, env, commands and commit message in
English.

Goal:
The worker should be able to build text context for either:
- VK bot: legacy `user_id + vk_peer_id`.
- Mini App: explicit `source=miniapp` + `external_thread_id`.

Allowed scope:
- internal/service/dialogcontext/**
- internal/service/chatservice/** if a small facade is introduced
- internal/worker/**
- internal/domain/**
- internal/adapter/storage/memory/**
- internal/adapter/storage/postgres/** only if repository contract needs tests
- PROGRESS.md
- TASKS.md / DECISIONS.md / docs/CHAT_CONTEXT_REFACTOR_LOG.md if needed

Do not change:
- Mini App BFF behavior
- VK bot UI/menu behavior
- provider adapters
- billingservice behavior
- frontend

Safety:
- Chat service/facade may create text jobs only through `joborchestrator`.
- It must not call providers.
- Worker remains the only provider caller.
- Prompt rendering remains bounded: summary + recent messages + current prompt.
- Public model alias remains `ChatGPT`; raw DeepInfra model details stay hidden.

STEP 0 - reconnaissance without code:
1. Read `dialogcontext.Prepare`, `dialogcontext.Complete`, `worker.buildRequest`
   and text job params.
2. Report where explicit conversation reference will be read/written.

Implementation:
1. Add a stable job param shape for chat context, for example:
   - `conversation_source`
   - `conversation_id` or `external_thread_id`
   - public `model_name` remains `ChatGPT`.
2. Update `dialogcontext.Prepare` to prefer explicit conversation ref when
   present.
3. Preserve fallback to `user_id + vk_peer_id` for existing VK bot jobs.
4. Update `dialogcontext.Complete` to save assistant messages to the same
   explicit conversation.
5. Add tests:
   - VK bot fallback still works.
   - Mini App explicit thread A/B do not mix.
   - Same VK user can have VK bot and Mini App conversations without mixing.
   - Invalid/empty explicit refs degrade safely.
6. If introducing `internal/service/chatservice`, keep it small:
   - normalize chat job params
   - call `joborchestrator.CreateJob`
   - no provider/billing direct ownership

Checks:
- gofmt -w <changed Go files>
- go test ./internal/service/dialogcontext ./internal/worker
- go test ./...
- go build ./...

If green:
git add <only PR-18.2 files explicitly>
git commit -m "chat: support explicit conversation references"
git push origin fastlife_dev
git rev-parse --short HEAD

Acceptance criteria:
- [ ] VK bot context unchanged.
- [ ] Worker supports explicit Mini App conversation refs.
- [ ] No provider call outside worker.
- [ ] Thread isolation tests exist.
- [ ] Tests/build green.

Final response format:
1. PR-18.2 status: completed / blocked
2. STEP 0 summary
3. What changed in worker/dialogcontext
4. Compatibility and isolation
5. Checks
6. Commit/push + SHA
7. Security notes
```

---

## PR-18.3 - Mini App durable chat core switch

```text
MODE: IMPLEMENT

Task:
PR-18.3: switch Mini App chat from process-local context to the shared durable
chat core. Remove Mini App prompt-prefix memory from BFF. Keep endpoint
contracts compatible.

Target branch: fastlife_dev

Language:
Answer in Russian. File names, functions, env, commands and commit message in
English.

Goal:
Mini App `/miniapp/chat/messages` should create text jobs with explicit
durable conversation refs and let the worker/dialogcontext assemble context.

Allowed scope:
- internal/adapter/inbound/miniapp/**
- internal/app/miniapp/**
- internal/service/chatservice/** if introduced earlier
- internal/service/dialogcontext/** tests only if needed
- PROGRESS.md
- TASKS.md / DECISIONS.md / docs/CHAT_CONTEXT_REFACTOR_LOG.md if needed

Do not change:
- frontend behavior unless absolutely required for existing DTO compatibility
- provider adapters
- billingservice
- worker provider execution path

Safety:
- Mini App still verifies VK launch params on every chat request.
- `conversation_id` is an opaque per-user thread id, not trusted identity.
- No process-local prompt history.
- No prompt body / generated answer / launch params / PII in logs.
- Public model name remains `ChatGPT`.
- Billing and provider remain job/worker-owned.

STEP 0 - reconnaissance without code:
1. Read Mini App chat handler and `conversation.go`.
2. Confirm all usages of process-local `conversationStore`.
3. Report removal/switch plan before editing.

Implementation:
1. Stop prefixing prompts in BFF with process-local context.
2. Create text jobs with explicit durable chat params:
   - `conversation_source=miniapp`
   - `conversation_id`/`external_thread_id` from normalized request
   - public model alias `ChatGPT`
3. Remove or retire `internal/adapter/inbound/miniapp/conversation.go` if no
   longer needed.
4. Keep `conversation_id=""` backward compatible as `default`.
5. Tests:
   - create chat message sets durable Mini App conversation ref.
   - unsupported/invalid conversation id returns safe 400.
   - no process-local context prompt prefix remains.
   - two Mini App threads do not share context via BFF state.
6. Update docs/log.

Checks:
- gofmt -w <changed Go files>
- go test ./internal/adapter/inbound/miniapp ./internal/service/dialogcontext ./internal/worker
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build

If green:
git add <only PR-18.3 files explicitly>
git commit -m "miniapp: use durable chat context"
git push origin fastlife_dev
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Mini App chat context survives API restart.
- [ ] Mini App threads are isolated by backend user + thread id.
- [ ] VK bot context unchanged.
- [ ] No provider calls from Mini App BFF.
- [ ] Tests/build green.

Final response format:
1. PR-18.3 status: completed / blocked
2. STEP 0 summary
3. Mini App context switch
4. Compatibility and isolation
5. Checks
6. Commit/push + SHA
7. Security notes
```

---

## PR-18.4 - Mini App conversation list/history endpoints and frontend

```text
MODE: IMPLEMENT

Task:
PR-18.4: add backend-owned Mini App chat thread list/history endpoints and make
the frontend read thread metadata/messages from backend instead of treating
localStorage as truth.

Target branch: fastlife_dev

Language:
Answer in Russian. File names, functions, env, commands and commit message in
English.

Goal:
Mini App chat history should be durable and reload-safe. Frontend may keep only
active thread id and short UI preferences locally.

Allowed scope:
- internal/adapter/inbound/miniapp/**
- internal/app/miniapp/**
- internal/domain/**
- internal/adapter/storage/postgres/**
- internal/adapter/storage/memory/**
- web/miniapp/src/chat/**
- web/miniapp/src/api/client.ts
- PROGRESS.md
- TASKS.md / DECISIONS.md / docs/CHAT_CONTEXT_REFACTOR_LOG.md if needed

Do not change:
- provider adapters
- billingservice behavior
- VK bot behavior
- unrelated frontend tabs

Endpoints:
- GET /miniapp/chat/conversations
- GET /miniapp/chat/conversations/{id}/messages

Safety:
- Both endpoints require VK launch-param auth.
- Return only conversations/messages owned by the verified backend user.
- Do not return provider internals or raw error details.
- Do not persist prompt/answer text in localStorage.
- Artifact URLs remain backend-owned routes only.

STEP 0 - reconnaissance without code:
1. Read Mini App frontend thread state and current backend conversation repo.
2. Report localStorage keys and what will remain client-side.

Implementation:
1. Add BFF DTOs for conversation list and message history.
2. Add repository methods if missing.
3. Add Mini App handler tests:
   - auth required.
   - owner-only access.
   - pagination/limit bounded.
   - invalid thread id safe 400/404.
4. Update frontend:
   - fetch thread list from backend.
   - fetch messages for active thread from backend.
   - keep only active thread id / UI preferences in localStorage.
   - graceful empty state after backend restart/data absence.
5. Safe render messages as React text; no innerHTML.
6. Update docs/log.

Checks:
- gofmt -w <changed Go files>
- go test ./internal/adapter/inbound/miniapp ./internal/adapter/storage/postgres ./internal/adapter/storage/memory
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build

If green:
git add <only PR-18.4 files explicitly>
git commit -m "miniapp: add durable chat history"
git push origin fastlife_dev
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Mini App thread list comes from backend.
- [ ] Mini App message history comes from backend.
- [ ] localStorage is not source of truth and stores no prompt/answer text.
- [ ] Owner checks and auth enforced.
- [ ] Tests/build green.

Final response format:
1. PR-18.4 status: completed / blocked
2. STEP 0 summary
3. Endpoints/frontend changes
4. localStorage/security notes
5. Checks
6. Commit/push + SHA
7. Notes
```

---

## PR-18.5 - Shared chat context cleanup and verification

```text
MODE: IMPLEMENT

Task:
PR-18.5: cleanup and verify the shared durable chat context rollout. Remove
obsolete docs/code references to process-local Mini App chat memory, run
regression checks, and update architecture/runbook.

Target branch: fastlife_dev

Language:
Answer in Russian. File names, functions, env, commands and commit message in
English.

Allowed scope:
- docs/**
- README.md
- RUNBOOK.md
- PROGRESS.md
- TASKS.md
- DECISIONS.md
- internal/adapter/inbound/miniapp/** only for dead-code cleanup
- web/miniapp/src/** only for dead-code cleanup

Safety:
- No behavior changes except removal of unreachable/dead code.
- No provider/billing/worker logic changes.
- No .env/secrets/logs.

Verification:
1. Confirm VK bot and Mini App both use durable conversation core.
2. Confirm no process-local Mini App context store remains.
3. Confirm provider calls still occur only from worker flow.
4. Confirm Mini App localStorage has no prompt/answer/artifact URL.
5. Confirm public model output remains `ChatGPT`.

Checks:
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build
- git grep -nE "^(<<<<<<<|=======|>>>>>>>)" || echo clean
- git diff --check

If green:
git add <only PR-18.5 files explicitly>
git commit -m "docs: verify shared chat context rollout"
git push origin fastlife_dev
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Docs reflect final shared chat context architecture.
- [ ] Obsolete process-local Mini App context references removed or marked old.
- [ ] Security invariants verified.
- [ ] Checks green.

Final response format:
1. PR-18.5 status: completed / blocked
2. What was verified
3. Cleanup/docs updated
4. Checks
5. Commit/push + SHA
6. Notes
```
