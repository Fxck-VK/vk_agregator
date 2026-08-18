# Optimistic Workspace Logout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the visible logout wait with an immediate guest-locked workspace while securely completing and verifying server logout in the background.

**Architecture:** A client `WorkspaceLogoutBoundary` owns the logout state above the private workspace providers and exposes actions through context. It swaps the entire authenticated subtree for the existing guest composition synchronously, performs bounded transport retries, coordinates other tabs, and retains a retryable guest state on failure.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, CSS Modules, Vitest and Testing Library.

## Global Constraints

- Do not change the backend, cookies, DEV Basic Auth or session refresh behaviour.
- Never restore private workspace data automatically after a failed optimistic logout.
- Keep the network retry window below five seconds.
- Keep theme and sidebar preferences; clear only account-derived in-memory and pending-conversation data.
- Follow TDD and observe focused tests fail before production changes.

---

### Task 1: Bounded logout operation and private session-storage cleanup

**Files:**
- Create: `web/platform/src/features/session/WorkspaceLogout/workspace-logout-request.ts`
- Create: `web/platform/src/features/session/WorkspaceLogout/workspace-logout-request.test.ts`
- Modify: `web/platform/src/features/conversations/pending-conversation-prompt.ts`
- Modify: `web/platform/src/features/conversations/pending-conversation-prompt.test.ts`
- Modify: `web/platform/src/features/conversations/pending-conversation-title-sync.ts`
- Modify: `web/platform/src/features/conversations/pending-conversation-title-sync.test.ts`

**Interfaces:**
- Produces: `requestWorkspaceLogout(): Promise<void>`; resolves only for HTTP `204`, retries transport failures at most twice and rejects all terminal failures.
- Produces: `clearPendingConversationPrompts()` and `clearPendingConversationTitleSyncs()`; remove only keys owned by those modules.

- [ ] **Step 1:** Add failing tests for transport retry, timeout exhaustion, non-204 rejection and prefix-scoped session-storage cleanup.
- [ ] **Step 2:** Run `npm test -- workspace-logout-request.test.ts pending-conversation-prompt.test.ts pending-conversation-title-sync.test.ts` and verify the new APIs are missing.
- [ ] **Step 3:** Implement three 1.3-second attempts with 250/500 ms waits, pass an `AbortSignal` to `webBrowserMutation`, and add prefix-scoped cleanup functions.
- [ ] **Step 4:** Re-run the focused tests and expect them all to pass.

### Task 2: Persistent optimistic logout boundary

**Files:**
- Create: `web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.tsx`
- Create: `web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.module.css`
- Create: `web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Produces: `WorkspaceLogoutBoundary({ children, guest })`.
- Produces: `useWorkspaceLogout()` for the account action and `useOptionalWorkspaceLogout()` for guest login controls.
- Context actions: `logout(): void`, `requestLogin(): void`, `retry(): void`.

- [ ] **Step 1:** Add failing tests proving the guest subtree replaces private content before a deferred request settles, failed logout never restores private content, retry can recover, login intent waits for confirmation and BroadcastChannel messages synchronize tabs.
- [ ] **Step 2:** Run `npm test -- WorkspaceLogoutBoundary.test.tsx` and verify the component/API is absent.
- [ ] **Step 3:** Implement the state machine, 140 ms reduced-motion-aware veil, polite failure notice, deduplicated request lifecycle, route refresh and optional BroadcastChannel coordination.
- [ ] **Step 4:** Re-run the boundary tests and expect all scenarios to pass.

### Task 3: Account menu, login controls and workspace composition

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.tsx`
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`
- Modify: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.tsx`
- Modify: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.test.tsx`
- Modify: `web/platform/src/app/app/layout.tsx`
- Modify: `web/platform/src/app/app/layout.test.tsx`

**Interfaces:**
- Consumes: boundary context actions from Task 2.
- Produces: authenticated layout wrapped once by `WorkspaceLogoutBoundary`, with the existing `GuestWorkspaceFrame` and `WorkspaceHome access="guest"` as its optimistic guest subtree.

- [ ] **Step 1:** Replace the old AccountControl network assertions with failing context-driven logout assertions; add failing layout and login-action tests for the optimistic guest composition.
- [ ] **Step 2:** Run `npm test -- AccountControl.test.tsx WorkspaceLoginAction.test.tsx src/app/app/layout.test.tsx` and verify failures describe the old local request/redirect flow.
- [ ] **Step 3:** Move request ownership out of `AccountControl`, make optimistic guest login controls boundary-aware, and wrap the authenticated server layout with the boundary.
- [ ] **Step 4:** Re-run focused tests and expect authenticated and ordinary unauthenticated rendering to remain green.

### Task 4: Full frontend verification

**Files:**
- Verify only.

**Interfaces:**
- Consumes all preceding tasks and produces a locally verified change without publishing it.

- [ ] **Step 1:** Run `npm test` and expect zero failures.
- [ ] **Step 2:** Run `npm run typecheck` and expect exit code 0.
- [ ] **Step 3:** Run `npm run lint` and expect exit code 0.
- [ ] **Step 4:** Run `npm run build` and expect exit code 0.
- [ ] **Step 5:** Run `git diff --check` and inspect `git status --short` for unintended or secret-bearing files.

