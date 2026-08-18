# Guest Workspace Login Entry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let unauthenticated visitors view the existing `/app` landing while exposing explicit login buttons and withholding all private workspace data.

**Architecture:** Add a dedicated guest composition around the existing `AppShell`, `Sidebar` and `WorkspaceHeader`. The server layout selects this composition only for `unauthenticated`, while authenticated, refresh and unavailable flows stay unchanged. Guest prompt submission is an explicit login transition and cannot reach private mutations.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, CSS Modules, Vitest and Testing Library.

## Global Constraints

- Do not change DEV Basic Auth or backend APIs.
- Do not render conversations, account identity, files or balance before authentication.
- Preserve the existing `/app` visual design and authenticated behavior.
- Use `/login` as the only guest authentication destination.
- Follow TDD: observe every new test failing before implementation.

---

### Task 1: Shared login action and header trailing slot

**Files:**
- Create: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.tsx`
- Create: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.module.css`
- Create: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.test.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.test.tsx`

**Interfaces:**
- Produces: `WorkspaceLoginAction({ placement: "header" | "sidebar" })` linking to `/login`.
- Produces: `WorkspaceHeader({ balance, trailingAction })`, where `trailingAction` replaces the balance when provided.

- [ ] **Step 1: Write failing tests** proving both login-action variants link to `/login` and a header action removes the balance.
- [ ] **Step 2: Run focused tests** with `npm test -- WorkspaceLoginAction.test.tsx WorkspaceHeader.test.tsx`; expect missing component/prop failures.
- [ ] **Step 3: Implement the minimal component and header slot** using existing tokens and CSS modules.
- [ ] **Step 4: Re-run focused tests** and expect all selected tests to pass.

### Task 2: Guest shell composition

**Files:**
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`

**Interfaces:**
- Produces: `GuestWorkspaceFrame({ children })` with no private providers or private props.
- Consumes: `WorkspaceLoginAction` and the `WorkspaceHeader.trailingAction` slot from Task 1.

- [ ] **Step 1: Write a failing frame test** requiring two `Войти` links, no balance and no conversations/account data.
- [ ] **Step 2: Run the focused frame/sidebar tests** and observe the missing guest composition failure.
- [ ] **Step 3: Extract the shared responsive chrome state** and implement `GuestWorkspaceFrame` without `WorkspaceAccountProvider` or `SidebarConversations`.
- [ ] **Step 4: Re-run the focused tests** and expect them to pass without changing authenticated assertions.

### Task 3: Guest prompt and server layout selection

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.tsx`
- Modify: `web/platform/src/app/app/layout.tsx`
- Modify: `web/platform/src/app/app/layout.test.tsx`

**Interfaces:**
- Produces: `WorkspacePrompt({ access: "authenticated" | "guest", variant })` with guest submit routing to `/login`.
- Produces: `WorkspaceLanding({ access })` and `WorkspaceHome({ access, section })` with authenticated default.
- Consumes: `GuestWorkspaceFrame` from Task 2.

- [ ] **Step 1: Replace the redirect assertion with a failing guest-layout test** and add a failing guest-prompt no-mutation test.
- [ ] **Step 2: Run the two focused test files** and verify failures reflect current redirect/private-mutation behavior.
- [ ] **Step 3: Implement guest prompt routing and unauthenticated layout composition** while omitting private children.
- [ ] **Step 4: Re-run focused tests** and expect both authenticated and guest scenarios to pass.

### Task 4: Full frontend verification

**Files:**
- Verify only; no new source file is required.

**Interfaces:**
- Consumes all preceding tasks.
- Produces a verified local implementation; it does not publish or deploy.

- [ ] **Step 1: Run all frontend unit tests** with `npm test` and expect zero failures.
- [ ] **Step 2: Run TypeScript validation** with `npm run typecheck` and expect exit code 0.
- [ ] **Step 3: Run lint** with `npm run lint` and expect exit code 0.
- [ ] **Step 4: Run the production build** with `npm run build` and expect exit code 0.
- [ ] **Step 5: Review `git diff --check` and `git status --short`** to confirm only intended frontend/spec files changed and no secrets were introduced.
