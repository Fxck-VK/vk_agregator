# Chat Context Refactor Log

Purpose: track the planned fix that makes VK text bot and VK Mini App chat use
one durable backend chat context core without making either surface call the
other surface.

## 2026-06-06 - Initial finding

Initial state before PR-18:

- VK text bot context is durable and worker-owned:
  - `internal/service/dialogcontext/service.go`
  - `internal/worker/worker.go`
  - `internal/adapter/storage/postgres/conversation.go`
  - `migrations/000006_conversation_context.up.sql`
- The worker calls `dialogcontext.Prepare` before provider submit and
  `dialogcontext.Complete` after text output, so provider calls stay in the
  worker flow.
- VK bot conversations are keyed by `user_id + vk_peer_id`.
- Mini App chat sent `conversation_id` to `POST /miniapp/chat/messages`, but
  the BFF kept recent turns in a process-local
  `internal/adapter/inbound/miniapp/conversation.go` store.
- Mini App process-local context was not durable and could be lost on API restart
  or scale-out.
- If Mini App is forced through the existing `user_id + vk_peer_id` lookup,
  different Mini App threads for the same VK user can be mixed.

Target direction:

- Do not call VK bot from Mini App.
- Do not copy VK bot context code into Mini App.
- Extend the shared durable conversation model with explicit source/thread
  identity.
- Keep both surfaces thin:
  - VK bot surface owns VK callback/menu/dialog-mode details.
  - Mini App surface owns launch-param auth and BFF DTOs.
  - Shared chat/conversation core owns durable memory and text job setup.
  - Worker remains the only provider caller.

Core safety requirements:

- Provider calls stay out of `cmd/api`, `internal/app/*` and inbound handlers.
- Billing remains append-only and job-owned.
- Mini App `conversation_id` is opaque and ownership-scoped to the verified
  backend user.
- VK bot context, Mini App default thread and Mini App custom threads must not
  mix.
- No prompt bodies, generated answers, launch params, tokens, secrets, PII or
  private artifact URLs in logs.

Planned PR chain:

- PR-18.1: durable conversation identity schema/domain/repository foundation.
- PR-18.2: worker/dialogcontext explicit conversation reference support plus
  shared chat job contract.
- PR-18.3: Mini App chat switches from process-local context to durable shared
  chat core.
- PR-18.4: Mini App thread list/history endpoints and frontend integration.
- PR-18.5: cleanup, docs and regression/security verification.

## 2026-06-06 - PR-18.1 foundation implemented

Changes:

- Added migration `000008_conversation_sources`:
  - `conversations.source` with allowed values `vk_bot` / `miniapp`;
  - `conversations.external_thread_id`;
  - active VK bot unique index on `user_id + vk_peer_id` for
    `source='vk_bot'`;
  - active Mini App/source-thread unique index on
    `user_id + source + external_thread_id` for non-VK-bot sources;
  - list index on `user_id + source + updated_at DESC`.
- Extended `domain.Conversation` with `Source` and `ExternalThreadID`.
- Added `domain.ConversationRef` for future explicit source/thread lookup.
- Extended `domain.ConversationRepository` with:
  - `GetActiveByReference`;
  - `GetByIDForUser`;
  - `ListByUserSource`.
- Updated Postgres and memory repositories.
- Added memory repository tests proving VK bot lookup remains compatible and
  Mini App thread ids are isolated for the same backend user.

Behavior note:

- PR-18.1 does not switch Mini App runtime behavior yet.
- VK bot durable context remains backward compatible through
  `GetActiveByUserPeer`.
- Mini App process-local context still exists until PR-18.3.

## 2026-06-06 - PR-18.2 explicit conversation refs implemented

Changes:

- `dialogcontext.Prepare` now supports explicit text-job conversation params:
  - `conversation_source`;
  - `external_thread_id`;
  - durable `conversation_id` after the worker has created/loaded a backend
    conversation.
- VK bot jobs without explicit params still fall back to the legacy
  `user_id + vk_peer_id` conversation lookup.
- Mini App-style explicit refs use `source=miniapp` plus the opaque
  `external_thread_id`, scoped by backend `user_id`.
- `dialogcontext.Complete` can save assistant messages to the durable
  conversation created from an explicit ref.
- The worker preserves `conversation_source` and `external_thread_id` when it
  patches `job.Params` with the durable `conversation_id`.
- Empty or invalid explicit refs degrade to a plain prompt with no conversation
  context instead of falling back to a shared/legacy thread.

Tests added:

- VK bot fallback remains covered by existing dialog context tests.
- Mini App thread A/B contexts do not mix.
- The same backend user can have VK bot and Mini App conversations without
  cross-surface context leakage.
- Invalid explicit refs pass through safely.
- Worker param patching preserves the explicit conversation ref fields.

Behavior note:

- PR-18.2 does not switch Mini App BFF behavior yet. That remains PR-18.3.

## 2026-06-06 - PR-18.3 Mini App durable context switch implemented

Changes:

- Removed the Mini App process-local prompt-prefix memory store from
  `internal/adapter/inbound/miniapp`.
- `POST /miniapp/chat/messages` now creates text jobs with the current user
  prompt only. It does not prepend prior turns in the BFF.
- Mini App chat jobs carry explicit durable context refs:
  - `conversation_source=miniapp`;
  - `external_thread_id=<normalized conversation_id>`.
- The user-facing `conversation_id` stays an opaque per-user thread id.
  Empty `conversation_id` still maps to `default`.
- The durable backend `conversation_id` is still worker-owned and is patched
  into `job.Params` after `dialogcontext.Prepare` creates/loads the
  conversation.
- Completed assistant messages are saved by worker/dialogcontext, not by
  Mini App BFF.

Tests added/updated:

- Chat job creation sets Mini App durable conversation refs and keeps public
  model alias `ChatGPT`.
- Empty Mini App `conversation_id` maps to `default`.
- Invalid Mini App `conversation_id` returns a safe 400.
- Mini App BFF no longer prefixes prompts with local history and keeps thread
  refs isolated by `external_thread_id`.

Behavior note:

- Mini App chat context now survives API restart after jobs are processed by
  the worker, because conversation history is stored through the durable
  conversation repository.
- Conversation list/history endpoints are still PR-18.4 follow-up work.

## 2026-06-06 - PR-18.4 durable Mini App history endpoints implemented

Changes:

- Added authenticated Mini App BFF read endpoints:
  - `GET /miniapp/chat/conversations`;
  - `GET /miniapp/chat/conversations/{id}/messages`.
- Wired `ConversationRepository` into the Mini App app surface through shared
  API core.
- Conversation list uses backend `user_id + source=miniapp` ownership scope.
- Message history lookup uses verified backend user plus opaque
  `external_thread_id`.
- Endpoint DTOs expose only product-level data:
  - Mini App thread id;
  - title / timestamps / last message preview;
  - message role `user` or `bot`;
  - message text.
- Provider names, model ids, billing internals and storage/artifact internals
  are not exposed by these history endpoints.
- Frontend now fetches thread list and active thread messages from backend.
- `localStorage` now keeps only `vk_miniapp_active_thread_v1`; old local
  thread/message keys are removed and prompt/answer text is not persisted.

Tests added:

- Auth is required for conversation list.
- Conversation list is owner-scoped and bounded by the existing Mini App
  pagination limit.
- Message history is owner-only.
- Invalid thread ids return safe 400.

Behavior note:

- Deleting/clearing local UI state does not delete backend conversation
  history. Backend delete/archive remains a separate future product decision.

## 2026-06-07 - PR-18.5 cleanup and verification completed

Verified rollout:

- VK bot and Mini App chat both use the durable shared conversation core:
  `conversations`, `conversation_messages`, `conversation_summaries`,
  `internal/service/dialogcontext` and worker-owned prompt rendering.
- Mini App BFF no longer has `internal/adapter/inbound/miniapp/conversation.go`
  or any process-local prompt/answer memory path.
- Mini App chat creation sends the current prompt plus
  `conversation_source=miniapp` and opaque `external_thread_id`; the worker
  creates/loads the durable backend conversation and saves assistant turns.
- Mini App conversation list/history endpoints are authenticated and scoped to
  the verified backend user.
- Provider calls remain worker-owned and out of `cmd/api`, VK inbound and Mini
  App BFF handlers.
- Mini App `localStorage` is limited to active thread/tab/theme UI state and
  legacy cache cleanup. Prompt bodies, generated answers, job ids, artifact
  ids/URLs, launch params, tokens, balance and provider details are not
  persisted there.
- Public Mini App chat model output remains `ChatGPT`.

Docs cleanup:

- ADR/process notes now mark old Mini App process-local chat memory as
  historical/superseded.
- `README.md`, `RUNBOOK.md` and `docs/ARCHITECTURE.md` describe the final shared
  durable chat context architecture and Mini App list/history smoke checks.
