# Optimistic Chat Bootstrap And Ratings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Open new conversations before the network responds and persist assistant-message ratings without delaying their controls.

**Architecture:** A validated session-scoped bootstrap intent transfers ownership from the originating composer to the temporary chat route while preserving both idempotency keys. Ratings use one nullable message column, one account-scoped PUT endpoint, and a serialized optimistic client queue.

**Tech Stack:** Go, PostgreSQL, TypeScript, React, Next.js App Router, Zod, Vitest, Testing Library.

## Global Constraints

- The first prompt must never be placed in a URL.
- Retry must reuse both original idempotency UUIDs.
- Only assistant messages in active account-owned Web conversations may be rated.
- Rating writes must be applied in click order.
- Follow RED → GREEN → REFACTOR for each task.

---

### Task 1: Durable client bootstrap intent

**Files:**
- Create: `web/platform/src/features/conversations/pending-conversation-bootstrap.ts`
- Create: `web/platform/src/features/conversations/pending-conversation-bootstrap.test.ts`
- Modify: `web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.tsx`

**Interfaces:**
- Produces `PendingConversationBootstrapIntent`, `savePendingConversationBootstrap`, `readPendingConversationBootstrap`, `updatePendingConversationBootstrap`, `clearPendingConversationBootstrap`, and `clearPendingConversationBootstraps`.

- [ ] Write tests for normalization, UUID validation, optional canonical conversation ID, exact-key cleanup, and logout-wide cleanup.
- [ ] Run `npm test -- pending-conversation-bootstrap.test.ts` and verify RED because the module does not exist.
- [ ] Implement JSON storage validation with Zod and guarded `sessionStorage` access.
- [ ] Run the focused test and verify GREEN.

### Task 2: Immediate temporary chat route

**Files:**
- Create: `web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx`
- Create: `web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx`
- Modify: `web/platform/src/app/app/chat/[conversationId]/page.tsx`
- Modify: `web/platform/src/app/app/chat/[conversationId]/page.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

**Interfaces:**
- `WorkspacePrompt` writes one bootstrap intent and calls `router.push('/app/chat/{clientUUID}?pending=1')` synchronously.
- `PendingConversationBootstrap` consumes that intent, performs the existing POST pair, and calls `router.replace('/app/chat/{serverUUID}?refresh=1')`.

- [ ] Add a deferred-request test asserting immediate route navigation, cleared input, optimistic sidebar row, user bubble, and dots before the first response.
- [ ] Run focused prompt/page/bootstrap tests and verify RED under the current response-first flow.
- [ ] Move network ownership into the pending-route component and render the existing chat visual language with in-place failure/retry.
- [ ] Add retry tests proving both `X-Idempotency-Key` values remain unchanged.
- [ ] Run the focused tests and verify GREEN.

### Task 3: Rating domain and storage

**Files:**
- Create: `migrations/000051_conversation_message_rating.up.sql`
- Create: `migrations/000051_conversation_message_rating.down.sql`
- Modify: `internal/domain/conversation.go`
- Modify: `internal/domain/repositories.go`
- Modify: `internal/adapter/storage/memory/conversation.go`
- Modify: `internal/adapter/storage/memory/conversation_test.go`
- Modify: `internal/adapter/storage/postgres/conversation.go`
- Modify: `internal/adapter/storage/postgres/conversation_management_test.go`
- Modify: `internal/adapter/storage/postgres/conversation_security_test.go`

**Interfaces:**
- Produces `ConversationMessageRating` and `SetMessageRatingForAccount(ctx, accountID, conversationID, messageID, source, rating)`.

- [ ] Write domain and repository tests for like, dislike, clear, wrong account, wrong conversation, and user-message rejection.
- [ ] Run focused Go tests and verify RED because the field and method do not exist.
- [ ] Add the nullable constrained column and implement memory/PostgreSQL updates using message primary-key lookup plus conversation ownership predicates.
- [ ] Run focused tests and verify GREEN.

### Task 4: Rating Web API

**Files:**
- Create: `internal/adapter/inbound/websession/conversation_message_rating_test.go`
- Modify: `internal/adapter/inbound/websession/handler.go`
- Modify: `internal/adapter/inbound/websession/message_history_test.go`

**Interfaces:**
- Produces `PUT /web/v1/conversations/{conversationID}/messages/{messageID}/rating` and adds nullable `rating` to safe history messages.

- [ ] Write handler tests for unauthorized, invalid IDs/body, account isolation, assistant-only enforcement, set/change/clear, and returned history.
- [ ] Run focused Web-session tests and verify RED because the route is absent.
- [ ] Register the unsafe-principal route, decode the strict rating body, call the repository, and return the canonical rating with `Cache-Control: no-store`.
- [ ] Run focused tests and verify GREEN.

### Task 5: Optimistic rating controls

**Files:**
- Create: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/lib/web-api/contracts.ts`

**Interfaces:**
- Assistant actions consume `conversationId`, `messageId`, and `initialRating`; they issue ordered PUT requests while rendering the latest choice immediately.

- [ ] Write client tests for persisted initial state, immediate toggle, ordered rapid changes, canonical success, and latest-write rollback.
- [ ] Run focused tests and verify RED because ratings are currently component-local.
- [ ] Extend the Zod history contract and pass stable identifiers from history to the action component.
- [ ] Implement the serialized mutation queue and rollback to the last confirmed value.
- [ ] Run focused tests and verify GREEN.

### Task 6: Full verification

**Files:**
- Verify all files changed in Tasks 1–5.

- [ ] Run `go test ./...` and require zero failures.
- [ ] Run frontend `npm test`, `npm run typecheck`, and `npm run lint` and require exit code 0.
- [ ] Run frontend `npm run build` and require exit code 0.
- [ ] Run `git diff --check` and inspect `git status --short` for scope.
