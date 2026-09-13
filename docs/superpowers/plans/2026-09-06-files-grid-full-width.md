# Full-Width Files Grid Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/app/files` use the complete inner width of its existing workspace shell without changing side gutters.

**Architecture:** Apply a files-page-only CSS custom-property override through `WorkspacePageFrame.className`. Retain the shared frame and the existing responsive `auto-fill` card grid.

**Tech Stack:** React, CSS Modules, Vitest style contracts

## Global Constraints

- Keep `--workspace-page-shell-width` and `--workspace-page-inline-gutter` unchanged.
- Do not change card styling or other workspace pages.
- Preserve the current responsive one/two/three-column grid behavior.
- Do not commit while the worktree contains unrelated user changes.

---

### Task 1: Expand only the files content column

**Files:**
- Modify: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.module.css`

**Interfaces:**
- Consumes: `WorkspacePageFrame.className` and `--workspace-content-frame-width`.
- Produces: a files-only `.pageFrame` override with `--workspace-content-frame-width: 100%`.

- [x] **Step 1: Add the failing style contract**

Assert that `FilesWorkspace` passes `styles.pageFrame`, that `.pageFrame` sets only the content-width variable to `100%`, and that the shared frame still owns the side gutter.

- [x] **Step 2: Verify RED**

Run `npm exec vitest run src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts -- --reporter=dot`. Expect failure because `styles.pageFrame` does not exist.

- [x] **Step 3: Add the local override**

Pass `className={styles.pageFrame}` to `WorkspacePageFrame` and add:

```css
.pageFrame {
  --workspace-content-frame-width: 100%;
}
```

- [x] **Step 4: Verify GREEN and the browser**

Run the focused test, then inspect `/app/files` at desktop width and confirm the cards occupy the complete inner shell while the outer gutters stay fixed.

- [x] **Step 5: Run regression checks**

Run `npm test`, `npm run typecheck`, `npm run lint`, and `git diff --check`.
