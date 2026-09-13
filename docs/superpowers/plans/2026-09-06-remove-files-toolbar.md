# Remove Files Toolbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the search and status toolbar from the files workspace and always render every loaded image job.

**Architecture:** `FilesWorkspace` remains responsible for loading, retrying, and paginating jobs, but no longer owns local filter state. The standalone `FilesToolbar` component stays unchanged and unmounted.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Preserve tabs, pagination, preview loading, retry behavior, and empty/error states.
- Do not commit or publish the changes.

---

### Task 1: Lock the toolbar-free behavior with a component test

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

- [x] Replace obsolete search/status filtering tests with a test that renders ready and pending jobs.
- [x] Assert that both jobs remain visible.
- [x] Assert that the search field, status select, and loaded-scope notice are absent.
- [x] Run `npx vitest run src/features/files/FilesWorkspace/FilesWorkspace.test.tsx --maxWorkers=4` and confirm the new assertions fail against the current toolbar.

### Task 2: Remove the toolbar integration and local filter state

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.module.css`

- [x] Remove the `FilesToolbar` import, filter helper, and query/status state.
- [x] Render `jobs` directly in `FilesGrid` and remove the filtered empty state.
- [x] Remove the unused scope-notice styles.
- [x] Run the targeted component test and confirm it passes.

### Task 3: Verify the complete change

**Files:**
- Modify: `docs/superpowers/plans/2026-09-06-remove-files-toolbar.md`

- [x] Inspect the files page and confirm the grid begins directly below the tabs.
- [x] Run the full Vitest suite with at most four workers.
- [x] Run asset tests, typecheck, and lint.
- [x] Review the final diff without committing or publishing.
