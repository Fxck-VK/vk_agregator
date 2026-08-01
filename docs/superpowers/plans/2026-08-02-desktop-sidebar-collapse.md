# Desktop Sidebar Collapse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow a desktop user to collapse and restore the fixed workspace sidebar while preserving the preference locally.

**Architecture:** Add a small client `WorkspaceFrame` that owns a boolean preference and renders the existing `AppShell` and `Sidebar`. `AppShell` changes only layout geometry; `Sidebar` changes only its control, panel accessibility, and wide-screen CSS. Mobile drawer state remains independent.

**Tech Stack:** Next.js App Router, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- The desktop breakpoint is exactly `48rem`; narrow layouts keep the existing drawer behavior.
- Persist only `neirohub.desktop-sidebar-collapsed` in `localStorage`; do not call an API or backend.
- All user-facing labels come from `web/platform/src/i18n/ru.ts`.
- Preserve the existing component-folder convention and leave the workspace scroll region unchanged.
- A hidden desktop panel must be `aria-hidden` and `inert`, while its restore button stays reachable.

---

### Task 1: Persisted desktop sidebar layout

**Files:**
- Create: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx`
- Create: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`
- Modify: `web/platform/src/app/app/layout.tsx`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.tsx`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- `WorkspaceFrame` consumes `account`, `conversations`, and `children` slots from the server layout.
- `WorkspaceFrame` produces `isDesktopSidebarCollapsed` and `onDesktopSidebarToggle` props for `AppShell` and `Sidebar`.
- `AppShell` consumes `isDesktopSidebarCollapsed?: boolean` and exposes it as `data-desktop-sidebar-collapsed` on the shell and sidebar region.
- `Sidebar` consumes optional `isDesktopCollapsed?: boolean` and `onDesktopToggle?: () => void` props; omitted props preserve standalone test usage.

- [ ] **Step 1: Write failing behavior tests**

Add assertions that a wide `Sidebar` renders a button named `ru.navigation.collapseSidebarLabel`, calls `onDesktopToggle`, exposes `aria-expanded="false"` and makes `sidebar-panel` inert when `isDesktopCollapsed` is true. Add a `WorkspaceFrame` test that clicks the control, checks `localStorage.getItem("neirohub.desktop-sidebar-collapsed") === "true"`, unmounts, then renders again and observes the restored collapsed state.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm.cmd --prefix web/platform test -- --run src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/AppShell/AppShell.test.tsx`

Expected: FAIL because the desktop control, propagated state, and persisted preference do not exist yet.

- [ ] **Step 3: Implement the smallest state and layout changes**

Create `WorkspaceFrame` as a client component. Start with `false`, read a stored literal `"true"` after mount, and write only from an explicit toggle so initial hydration cannot overwrite a saved value. Render `AppShell` with the same `sidebar` and `children` slots. Add Russian collapse/expand labels. Add optional state props to `AppShell` and `Sidebar`; use data attributes for CSS instead of global selectors. Add an inline SVG sidebar icon inside the desktop control. Use a wide-screen CSS rule to move the panel left and shrink the `AppShell` sidebar width/margin, and a narrow-screen rule to keep the existing hamburger drawer untouched.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm.cmd --prefix web/platform test -- --run src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/AppShell/AppShell.test.tsx`

Expected: PASS with no test failures.

- [ ] **Step 5: Run type and style checks**

Run: `npm.cmd --prefix web/platform run typecheck` and `npm.cmd --prefix web/platform run lint`

Expected: both commands exit with code 0.

- [ ] **Step 6: Commit the task**

```bash
git add web/platform/src/app/app/layout.tsx web/platform/src/components/layout/WorkspaceFrame web/platform/src/components/layout/AppShell web/platform/src/components/layout/Sidebar web/platform/src/i18n/ru.ts
git commit -m "feat: add desktop sidebar collapse"
```
