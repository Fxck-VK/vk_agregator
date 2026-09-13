# Workspace Right Gap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every authenticated application page the same fixed desktop gap between its workspace panel and the browser's right edge.

**Architecture:** Store the approved gap in one global CSS custom property and consume it only through the shared `AppShell`. Preserve the existing edge-to-edge mobile breakpoint.

**Tech Stack:** CSS custom properties, CSS Modules, Vitest

## Global Constraints

- Desktop gap remains exactly `0.125rem`.
- Every route rendered by `WorkspaceFrame` receives the same geometry.
- At widths below `48rem`, the workspace remains edge-to-edge.
- Do not change page-level content widths or padding.

---

### Task 1: Lock the shared workspace edge gap

**Files:**
- Modify: `web/platform/src/app/globals.css`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.floating-gap.test.ts`

**Interfaces:**
- Produces: global CSS token `--app-workspace-edge-gap`.
- Consumes: `AppShell` uses the token through `--app-shell-edge-gap` for the workspace's logical end margin.

- [ ] **Step 1: Write the failing contract test**

Read both stylesheets and assert that `globals.css` defines `--app-workspace-edge-gap: 0.125rem`, `AppShell` maps `--app-shell-edge-gap` to it, and `.workspace` uses the mapped token for `margin-inline-end`.

- [ ] **Step 2: Run the focused test and verify RED**

Run `npm exec vitest run src/components/layout/AppShell/AppShell.floating-gap.test.ts`. Expected: failure because the global token does not exist.

- [ ] **Step 3: Implement the shared token**

Add `--app-workspace-edge-gap: 0.125rem` to the global geometry token block and replace the local literal in `AppShell.module.css` with `var(--app-workspace-edge-gap)`.

- [ ] **Step 4: Run focused and AppShell tests**

Run `npm exec vitest run src/components/layout/AppShell`. Expected: all AppShell test files pass.

- [ ] **Step 5: Verify the project**

Run `npm run typecheck`, ESLint for the changed test, `git diff --check`, and inspect `/app` at desktop and mobile widths.
