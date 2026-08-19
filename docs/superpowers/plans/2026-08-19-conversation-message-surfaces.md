# Conversation Message Surfaces Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render compact user bubbles and unframed assistant replies in the shared conversation history.

**Architecture:** Keep the existing semantic ordered list and action components. Change only role-label rendering and CSS Module surface rules so optimistic, pending, and persisted messages share the same presentation automatically.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve all conversation requests, polling, retries, ratings, and idempotency.
- Preserve action buttons and typing indicators.
- Preserve card styling for empty/error states.
- Do not add dependencies.

---

### Task 1: Message surfaces

**Files:**
- Create: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.styles.test.ts`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.module.css`

**Interfaces:**
- Consumes: existing conversation message roles and `ConversationMessageActions`.
- Produces: unchanged `ConversationHistory` API with a new visual contract.

- [ ] **Step 1: Add failing component and stylesheet tests**

Verify that assistant items omit the role label, user items are fit-content and right-aligned, and assistant items have no card surface.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `npm test -- src/features/conversations/ConversationHistory/ConversationHistory.styles.test.ts src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Expected: FAIL against the current full-width user and card-style assistant messages.

- [ ] **Step 3: Implement minimal rendering and CSS changes**

Render the role label only for user messages. Split shared card rules so empty states stay cards, user messages become fit-content bubbles, and assistant messages are transparent and borderless.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the focused command from Step 2 and expect PASS.

- [ ] **Step 5: Run full verification**

Run `npm test`, `npm run lint`, `npm run typecheck`, and `npm run build` from `web/platform`; expect all checks to pass.
