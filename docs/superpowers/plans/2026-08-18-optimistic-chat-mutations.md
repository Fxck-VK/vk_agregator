# Optimistic Chat Mutations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add immediate, reversible UI state for conversation rename, deletion, and existing-chat message sending.

**Architecture:** Conversation metadata mutations project changes through the existing shared conversation-list context and reconcile with the server. Message mutation ownership moves to `ConversationHistory`, which already owns pending-turn rendering and polling, while `ConversationComposer` becomes a synchronous input component.

**Tech Stack:** TypeScript, React, Next.js App Router, Vitest, Testing Library, CSS modules.

## Global Constraints

- Do not change backend routes or contracts.
- Reuse the same message idempotency key on retry.
- Do not refresh the whole route for successful sidebar mutations.
- Keep recoverable errors next to the affected chat/message.
- Follow RED → GREEN → REFACTOR for every behavior.

---

### Task 1: Optimistic conversation rename

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.tsx`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `useOptionalWorkspaceConversationList()` with `updateConversationTitle(id, title)` and `replaceConversation(item)`.
- Produces: immediate shared title projection, canonical reconciliation, rollback, and retry label.

- [ ] **Step 1: Write the failing deferred-request tests**

Add tests that hold `PATCH` unresolved and assert the new title is already visible; reject the request and assert the old title returns while the editor keeps the proposed value and displays `Повторить`.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- ConversationRow.test.tsx`

Expected: optimistic-title and rollback assertions fail because the current component waits for the response.

- [ ] **Step 3: Implement the minimum optimistic projection**

Use the shared conversation-list context:

```ts
const previousConversation = conversation;
conversationList?.updateConversationTitle(conversation.id, proposedTitle);
try {
  const canonical = await renameConversation();
  conversationList?.replaceConversation(canonical);
} catch (error) {
  conversationList?.replaceConversation(previousConversation);
  throw error;
}
```

Preserve the current stale-request guard and show the retry copy after failure.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm test -- ConversationRow.test.tsx`

Expected: all rename tests pass with no warnings.

### Task 2: Optimistic conversation deletion

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.tsx`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.test.tsx`

**Interfaces:**
- Consumes: existing `onArchived(conversationId)` callback after canonical success.
- Produces: local hidden-row projection that remains mounted until the request settles.

- [ ] **Step 1: Write deferred delete tests**

Hold `DELETE` unresolved and assert the row is absent immediately. Reject it and assert the row and archive error return in the original list.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- ConversationRow.test.tsx`

Expected: the row remains visible until HTTP 204.

- [ ] **Step 3: Implement the minimum reversible hide**

Introduce `isOptimisticallyArchived`. Set it before the request and return `null` while true. On success call `onArchived`; on failure reset the flag and preserve the archive panel/error.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm test -- ConversationRow.test.tsx`

Expected: optimistic deletion and existing archive tests pass.

### Task 3: Optimistic message lifecycle

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- `ConversationComposer` produces `onSubmit(prompt: string): void` and clears its draft synchronously.
- `ConversationHistory` produces `submitPendingTurn(prompt, existingIntent?)`, owns HTTP mutation, pending status, polling, failure, and retry.

- [ ] **Step 1: Write composer RED tests**

Assert that Enter clears the field and calls `onSubmit` before any promise is required.

- [ ] **Step 2: Write history RED tests**

With a deferred mutation, assert the user bubble and typing dots are rendered immediately. Reject it and assert `Не отправлено` and `Повторить`; retry and compare the first and second `X-Idempotency-Key` headers.

- [ ] **Step 3: Run focused tests and verify RED**

Run: `npm test -- ConversationComposer.test.tsx ConversationHistory.test.tsx`

Expected: immediate pending and failure/retry assertions fail under the current response-first flow.

- [ ] **Step 4: Move mutation ownership and implement statuses**

Extend the pending turn:

```ts
type PendingTurn = {
  id: string;
  baselineSeq: number;
  prompt: string;
  idempotencyKey: string;
  status: "sending" | "accepted" | "failed";
};
```

On submit set `sending`; on valid response set `accepted` and start the existing poll; on failure set `failed`; on retry call the same mutation with `pendingTurn.idempotencyKey`.

- [ ] **Step 5: Render in-place failure controls**

Keep the failed user bubble, hide the typing indicator, and render the localized status plus retry button below it. Restore dots while retrying.

- [ ] **Step 6: Run focused tests and verify GREEN**

Run: `npm test -- ConversationComposer.test.tsx ConversationHistory.test.tsx`

Expected: all focused tests pass with no React act warnings.

### Task 4: Regression verification

**Files:**
- Verify all changed source and test files.

**Interfaces:**
- Produces: evidence that the three optimistic flows do not regress workspace behavior.

- [ ] **Step 1: Run the complete frontend suite**

Run: `npm test`

Expected: zero failed tests.

- [ ] **Step 2: Run static verification**

Run: `npm run typecheck`

Run: `npm run lint`

Expected: both commands exit 0.

- [ ] **Step 3: Run production build**

Run: `npm run build`

Expected: production build exits 0.

- [ ] **Step 4: Inspect the final diff**

Run: `git diff --check`

Run: `git status --short`

Expected: no whitespace errors and only the intended frontend/docs changes.
