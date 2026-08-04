# New Chat Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the `/app/chats` placeholder with the functional normal-chat entry screen approved by the user.

**Architecture:** Add a dedicated `chats` presentation branch to `WorkspaceHome` and a visual `newChat` variant to the existing `WorkspacePrompt`. Reuse all existing API, optimistic sidebar, pending prompt, conversation history, typing indicator, scrolling, and smart-title infrastructure.

**Tech Stack:** Next.js App Router, React, TypeScript, CSS Modules, Vitest, Testing Library.

## Constraints

- Frontend only; do not change backend contracts, billing, authentication, routing intelligence, or model selection.
- Do not duplicate the create-conversation request sequence.
- Preserve idempotency keys and recoverable retry behaviour.
- Keep the workspace header and balance layout-owned and static.
- Preserve mobile navigation and sidebar behaviour.

### Task 1: Lock the desired screen in tests

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

- [ ] Add a test that `section="chats"` renders the approved greeting and shared prompt, and excludes the old section placeholder plus quick actions.
- [ ] Add a test that the new-chat prompt variant uses “Задайте вопрос NeiroHub”.
- [ ] Run the two focused test files and confirm the new assertions fail for the intended missing UI.

### Task 2: Implement the dedicated new-chat presentation

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.module.css`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.module.css`

- [ ] Add the `chats` branch with centered greeting and `WorkspacePrompt variant="newChat"`.
- [ ] Add presentation-only prompt configuration without changing its request state machine.
- [ ] Use the existing `ChatTextInput` fixed-size and keyboard behaviour.
- [ ] Add responsive spacing for desktop and mobile.
- [ ] Run the focused tests and confirm they pass.

### Task 3: Regression verification and delivery

- [ ] Run all frontend unit tests.
- [ ] Run TypeScript checking and ESLint.
- [ ] Run a production Next.js build and packaging tests.
- [ ] Run `git diff --check` and inspect the final diff.
- [ ] Commit the tested frontend and documentation changes.
- [ ] Push the commit to `origin/dev-deploy`, run the DEV deployment workflow, and verify the DEV endpoint.

