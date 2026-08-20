# Collapsed Desktop Sidebar Implementation Plan

> **For Codex:** Execute this plan in the existing `remove-workspace-dividers` worktree with TDD. Do not change the mobile drawer behavior and do not push unless the user asks.

**Goal:** Replace the current desktop behavior that hides the sidebar with a persistent narrow icon rail matching the approved NeiroHub design.

**Architecture:** Keep `WorkspaceFrame` as the single owner of the persisted collapsed preference. Keep one `Sidebar` DOM tree for both widths and switch its presentation through the existing `data-desktop-collapsed` state. Add explicit data hooks to conversation and account controls so the rail can hide text, show stable icons, preserve accessible names, and render tooltips without duplicating navigation logic.

**Tech Stack:** Next.js, React, TypeScript, CSS Modules, Vitest, Testing Library.

---

### Task 1: Lock the rail behavior with component tests

**Files:**
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.desktop-collapse.test.ts`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.test.tsx`

1. Replace the old assertion that a collapsed desktop panel is inert with assertions that its navigation, conversations, and account remain accessible.
2. Assert that the collapsed brand control expands the sidebar and exposes the approved accessible label.
3. Assert that navigation and conversation controls expose tooltip text through stable data attributes.
4. Assert that the rail uses a square active treatment and the desktop workspace reserves the full rail width.
5. Run the targeted tests and confirm they fail for the current slide-away implementation.

### Task 2: Keep the desktop panel active and render the rail structure

**Files:**
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`

1. Keep the panel active on wide viewports even while collapsed; preserve existing inert and focus-trap behavior for the closed mobile drawer.
2. Move the desktop toggle into the brand row so the logo becomes the expand control in collapsed mode.
3. Add tooltip labels to brand and navigation controls while preserving visible labels in expanded mode.
4. Change the shell from a hidden trigger strip to a fixed rail width and align the workspace margin to it.
5. Style the collapsed rail with square active states, compact spacing, hover/focus tooltips, and no text overflow.

### Task 3: Adapt chats and account controls without duplicating components

**Files:**
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.module.css`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.tsx`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`

1. Add semantic data hooks for the chat list, chat rows, chat titles, and account trigger.
2. Add one reusable chat glyph inside each conversation row; show it only in collapsed desktop mode.
3. Hide chat titles and action ellipses in the rail while retaining links, focus behavior, and accessible names.
4. Reduce the account control to its avatar and show the profile tooltip on hover/focus.
5. Ensure active chats use the same square highlight as active navigation items.

### Task 4: Verify regressions and accessibility

**Files:**
- Verify only.

1. Run the targeted sidebar, frame, conversation-row, and account tests.
2. Run the platform typecheck and lint commands.
3. Run the relevant CSS contract tests.
4. Confirm the existing mobile drawer tests still pass unchanged.
5. Inspect `git diff` and ensure unrelated local files are not modified or staged.
