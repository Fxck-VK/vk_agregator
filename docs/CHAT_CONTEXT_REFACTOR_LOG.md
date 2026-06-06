# Chat Context Refactor Log

Purpose: track the planned fix that makes VK text bot and VK Mini App chat use
one durable backend chat context core without making either surface call the
other surface.

## 2026-06-06 - Initial finding

Current state:

- VK text bot context is durable and worker-owned:
  - `internal/service/dialogcontext/service.go`
  - `internal/worker/worker.go`
  - `internal/adapter/storage/postgres/conversation.go`
  - `migrations/000006_conversation_context.up.sql`
- The worker calls `dialogcontext.Prepare` before provider submit and
  `dialogcontext.Complete` after text output, so provider calls stay in the
  worker flow.
- VK bot conversations are keyed by `user_id + vk_peer_id`.
- Mini App chat sends `conversation_id` to `POST /miniapp/chat/messages`, but
  the BFF currently keeps recent turns in a process-local
  `internal/adapter/inbound/miniapp/conversation.go` store.
- Mini App process-local context is not durable and can be lost on API restart
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
