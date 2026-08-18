# Chat Scroll Waiting Indicator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the circular scroll-down arrow with animated dots while NeiroHub is producing a reply, then restore the normal scroll-down control when waiting ends.

**Architecture:** `ConversationHistory` already owns the authoritative active-response state. It passes a boolean through `ConversationComposer` to `ChatScrollToBottom`; the scroll component keeps all existing follow/scroll behavior and changes only the circular control's presentation while waiting.

**Tech Stack:** TypeScript, React 19, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Keep the circular control's size and position stable.
- Use `AssistantTypingIndicator` for the existing accessible three-dot animation.
- The waiting control is a status, not a clickable scroll button.
- Preserve all current scroll-follow and reduced-motion behavior.

---

### Task 1: Waiting presentation for the scroll control

**Files:**
- Modify: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.tsx`
- Modify: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.module.css`
- Modify: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

**Interfaces:**
- Consumes: `pendingTurnIsActive: boolean` and `activeRefreshID: number | null` from `ConversationHistoryReady`.
- Produces: `isAwaitingResponse?: boolean` on `ConversationComposer` and `ChatScrollToBottom`.

- [ ] **Step 1: Write failing component tests**

Add assertions that `isAwaitingResponse` renders `role="status"` with `ru.conversations.composerAwaitingResponse`, removes the scroll button, and restores the button after waiting ends while the region remains away from the bottom.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`

Expected: FAIL because `isAwaitingResponse` is not accepted and no circular status exists.

- [ ] **Step 3: Implement the minimal state propagation and presentation**

Pass `isAwaitingResponse={pendingTurnIsActive || (pendingTurn === null && activeRefreshID !== null)}` from the history to the composer and scroll control. Render the shared `AssistantTypingIndicator` inside the same circular shell while waiting; otherwise preserve the existing arrow button.

- [ ] **Step 4: Verify GREEN and the integration behavior**

Run the focused component tests, then `ConversationHistory.test.tsx`. Confirm that the status appears immediately after submission and the scroll button returns after the assistant response resolves.

- [ ] **Step 5: Run frontend verification**

Run: `npm run typecheck`, `npm run lint`, and `npm test` from `web/platform`.

Expected: all commands exit with code 0 and no warnings.
