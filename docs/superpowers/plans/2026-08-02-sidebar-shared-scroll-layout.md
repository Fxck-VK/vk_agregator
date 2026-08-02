# Sidebar Shared-Scroll Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the sidebar brand and account fixed while navigation and recent chats use one shared scroll area, without the duplicate create-chat button.

**Architecture:** `Sidebar` owns the structural three-zone flex layout and the sole scrollbar. `SidebarConversations` owns only the recent-chat list and uses the stable `sidebar-new-chat` link for archive-focus fallback when needed. No server contract changes are made.

**Tech Stack:** Next.js App Router, React, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Modify frontend files only; do not alter API calls, server code, or conversation payloads.
- Keep the existing mobile drawer close-on-navigation behaviour.
- Preserve visible rename/archive actions and keyboard focus after archiving a chat.
- Use existing design tokens and the existing custom scrollbar style.

---

### Task 1: Restructure the Sidebar and remove duplicate creation

**Files:**
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.scrollbar.test.ts`

**Interfaces:**
- Consumes: the existing `Sidebar` `account` and `conversations` slots; `SidebarConversations` receives its current `ConversationItem[]`.
- Produces: `styles.scrollArea` as the sole sidebar scroll owner and the stable DOM id `sidebar-new-chat` as the archive-focus fallback.

- [ ] **Step 1: Write failing component and stylesheet assertions**

Add assertions that `SidebarConversations` has no button named `ru.conversations.createLabel`, that the "New chat" navigation link is present with id `sidebar-new-chat`, that the archive fallback focuses it when no conversation successor exists, and that CSS exposes `scrollArea` with `overflow-y: auto` while `conversationsSlot` does not.

- [ ] **Step 2: Run focused tests to verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/conversations/SidebarConversations/SidebarConversations.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/Sidebar/Sidebar.scrollbar.test.ts
```

Expected: failure because the create button is still rendered and only `conversationsSlot` owns vertical scrolling.

- [ ] **Step 3: Implement the minimum structural change**

In `Sidebar.tsx`, place `nav` and `conversationsSlot` inside `<div className={styles.scrollArea}>`; assign `id="sidebar-new-chat"` to the existing `/app/chats` link. In the CSS, make `.panel` clip overflow, make `.scrollArea` flex and vertically scrollable, move scrollbar rules to it, and remove flex/overflow from `.conversationsSlot`. Keep brand and account as direct panel children.

In `SidebarConversations.tsx`, remove the `NewConversationButton` import and render. Replace its removed create-button focus fallback with a focus lookup for `#sidebar-new-chat` after the relevant last chat is archived.

- [ ] **Step 4: Run focused tests to verify GREEN**

Run the Step 2 command.

Expected: all focused tests pass, including mobile drawer behavior and focus fallback.

- [ ] **Step 5: Run complete frontend verification and commit**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run
npm.cmd --prefix web/platform run typecheck
npm.cmd --prefix web/platform run lint
```

Then stage only the six scoped source/test files and these two documents, and commit with:

```powershell
git commit -m "feat: unify sidebar navigation scrolling"
```
