# Instant Sidebar Conversation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new server-created conversation to the workspace sidebar immediately, without a reload.

**Architecture:** A client-only, account-scoped provider owns the visible conversation list for one mounted workspace. The server remains the source of initial and refreshed data; `WorkspacePrompt` merely upserts the already validated create response into that shared state.

**Tech Stack:** Next.js App Router, React Context, TypeScript, Zod-derived DTO types, Vitest, Testing Library.

## Global Constraints

- Frontend only: do not change API endpoints, backend, database, session format, or deployment configuration.
- Never invent a browser-only conversation id; only upsert a validated `ConversationItem` returned by `POST /web/v1/conversations`.
- Provider state is scoped by `AccountProfile.account_id` and resets on an account change.
- Preserve existing request idempotency, prompt retry behavior, pending prompt storage, navigation, archive/rename controls, and mobile sidebar semantics.
- No global events, localStorage/sessionStorage for list data, or background conversation-list polling.

---

### Task 1: Account-scoped visible conversation list

**Files:**
- Create: `web/platform/src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.tsx`
- Create: `web/platform/src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.test.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`
- Modify: `web/platform/src/app/app/layout.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.test.tsx`

**Interfaces:**
- Consumes: `ConversationItem[]` and `accountId: string` from the existing session contract.
- Produces: `WorkspaceConversationListProvider`, `useWorkspaceConversationList()`, and `upsertConversation(conversation: ConversationItem): void`.

- [x] **Step 1: Write failing provider and sidebar tests**

Create provider tests that render a probe consumer and assert: the supplied list renders initially; `upsertConversation` adds a new returned DTO at index zero; a second upsert with the same id replaces it without duplication; a rerender with a new `accountId` starts from that account's initial list. Add a `SidebarConversations` test rendered under the provider that proves a just-upserted conversation is linked at `/app/chat/:id`.

- [x] **Step 2: Run the focused tests to verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.test.tsx src/features/conversations/SidebarConversations/SidebarConversations.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx
```

Expected: FAIL because the provider and hook do not exist and `WorkspaceFrame` still receives a rendered conversation node.

- [x] **Step 3: Implement the minimal provider and wire the workspace**

Implement a client `WorkspaceConversationListProvider` with `initialConversations`, `accountId`, and an upsert callback that returns `[conversation, ...previousWithoutSameId]`. Sync a changed `initialConversations` prop to the server list. In `WorkspaceFrame`, accept `accountId` and `ConversationItem[]`, key the provider by account id, and render `SidebarConversations` under it. In the server layout, pass `session.profile.account_id` and `session.conversations`. Keep `SidebarConversations` compatible with an optional direct prop when no provider is present.

- [x] **Step 4: Run the focused tests to verify GREEN**

Run the Step 2 command.

Expected: all focused tests pass; no duplicate row and no cross-account state remains.

- [x] **Step 5: Commit Task 1**

Run `git diff --check`, stage only Task 1 source and test files, and commit:

```powershell
git commit -m "feat: add workspace conversation list state"
```

### Task 2: Upsert the first-message conversation immediately

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

**Interfaces:**
- Consumes: `useWorkspaceConversationList()` and its `upsertConversation(conversation: ConversationItem): void` from Task 1.
- Produces: immediate sidebar state insertion after a valid create response.

- [x] **Step 1: Write the failing integration test**

Render `WorkspacePrompt` and `SidebarConversations` under `WorkspaceConversationListProvider` with an empty initial list. Mock a valid create response followed by a pending message response. Submit a prompt and assert the conversation link appears in the sidebar after the create response, before the message promise settles and before `router.push` is called. Add a malformed-create-response assertion that no item is inserted.

- [x] **Step 2: Run the focused test to verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx
```

Expected: FAIL because `WorkspacePrompt` does not upsert the validated created conversation.

- [x] **Step 3: Implement the minimum first-message update**

Read the valid DTO as today, assign `intent.conversationId`, then call `upsertConversation(conversation)` before starting the first message mutation. Do not call it for malformed/non-success creation responses. Leave existing pending prompt storage and `router.push(`/app/chat/${conversationID}?refresh=1`)` unchanged.

- [x] **Step 4: Run the focused test to verify GREEN**

Run the Step 2 command.

Expected: the sidebar link appears before message completion, retry and error behavior remains unchanged, and no route is pushed for invalid creation data.

- [x] **Step 5: Run full frontend verification and commit**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run
npm.cmd --prefix web/platform run typecheck
npm.cmd --prefix web/platform run lint
npm.cmd --prefix web/platform run build
npm.cmd --prefix web/platform run test:packaging
```

Run `git diff --check`, stage only Task 2 source/test files and these two documents, then commit:

```powershell
git commit -m "feat: show new conversations immediately"
```

## Delivery notes

- The implementation was completed in three focused commits plus a repair commit after independent review found an ESLint violation and an asynchronous server-refresh race.
- The final regression coverage proves that a valid returned conversation appears in the sidebar before the first-message request resolves, and remains there if the authoritative server list refreshes while creation is pending.
- Isolated workspace render tests now explicitly provide the workspace conversation-list context; production rendering is still supplied by `WorkspaceFrame`.
