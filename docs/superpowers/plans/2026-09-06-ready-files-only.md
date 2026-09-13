# Ready Files Only Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make «Мои файлы» render only successfully generated files.

**Architecture:** `FilesWorkspace` derives a type-narrowed `succeeded` list from every loaded or cached page and passes only that list to `FilesGrid`. Existing result-preview and pagination flows remain unchanged, while obsolete retry-card UI tests are removed from this workspace contract.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

## Global Constraints

- Only jobs with status `succeeded` may appear as file cards.
- Preserve successful-file order, preview loading, tabs, pagination, and empty/error states.
- Do not commit or publish the changes.

---

### Task 1: Specify the ready-only workspace contract

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

- [x] Change the mixed-results test to require the pending job to be absent.
- [x] Add a test that expects the library empty state when every loaded job is unfinished.
- [x] Run the two tests and confirm they fail because unfinished cards are currently rendered.

### Task 2: Filter the files collection

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

- [x] Derive a type-narrowed list containing only `status === "succeeded"` jobs.
- [x] Base grid and empty-state rendering on that ready list.
- [x] Remove workspace tests whose only contract was interacting with unfinished retry cards.
- [x] Keep and adapt the cached-page test to use successful jobs.
- [x] Run the FilesWorkspace tests and confirm they pass.

### Task 3: Verify the user-visible result

**Files:**
- Modify: `docs/superpowers/plans/2026-09-06-ready-files-only.md`

- [x] Inspect the local files page and confirm only completed media cards remain.
- [x] Run the full Vitest suite with at most four workers.
- [x] Run asset tests, typecheck, and lint.
- [x] Review the final diff without committing or publishing.
