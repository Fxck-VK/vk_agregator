# Local File Cards Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render stable, representative cards on `/app/files` during frontend-only development.

**Architecture:** Extend the existing server-only preview data module with contract-valid image jobs, results, and same-origin artifact mappings. Let the existing BFF route serve JSON and local PNG bytes only for matching GET requests when the guarded preview mode is active; all other requests keep using the backend proxy.

**Tech Stack:** Next.js 16 route handlers, TypeScript, Zod-backed API contracts, Vitest

## Global Constraints

- Preview requires `NODE_ENV=development` and `NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1`.
- Production, preview-disabled traffic, and all mutations must retain the current proxy behavior.
- Use only non-sensitive fixed UUIDs and existing assets under `public/assets`.
- Do not alter the real file-card components to support preview data.
- Do not commit while the shared worktree contains unrelated user changes.

---

### Task 1: Serve local file-card fixtures

**Files:**
- Modify: `web/platform/src/app/web/v1/[...path]/route.test.ts`
- Modify: `web/platform/src/features/session/local-workspace-preview.ts`
- Modify: `web/platform/src/app/web/v1/[...path]/route.ts`

**Interfaces:**
- Consumes: `isLocalWorkspacePreviewEnabled(): boolean` and existing image-job API contracts.
- Produces: preview GET responses for the image-job list, succeeded-job results, and artifact images.

- [x] **Step 1: Write the failing route test**

Add a development-preview test that requests `/web/v1/image-jobs?limit=12`, verifies representative terminal states, follows each succeeded job to `/result`, and verifies its artifact endpoint returns non-empty `image/png` bytes. Assert that neither the internal origin nor the proxy is consulted.

- [x] **Step 2: Run the test to verify RED**

Run:

```powershell
npm exec vitest run "src/app/web/v1/[...path]/route.test.ts" -- --reporter=dot
```

Expected: the new preview-files assertion fails because `/web/v1/image-jobs` still returns the mocked proxy's `204` response.

- [x] **Step 3: Add contract-valid preview data**

Export fixed `ImageJobList`, `ImageJobResult` records, and artifact-to-public-path mappings from `local-workspace-preview.ts`. Include succeeded, expired, awaiting-payment, and failed-terminal jobs; only succeeded jobs receive results and artifacts.

- [x] **Step 4: Route matching GET requests to preview data**

In the BFF route, return JSON with `Cache-Control: no-store` for the preview job list and known results. Read mapped PNG files from `public/assets` and return their bytes with `Content-Type: image/png` and `Cache-Control: no-store`. Fall through to the existing proxy for unknown paths, all mutations, preview-disabled requests, and production.

- [x] **Step 5: Run focused tests to verify GREEN**

Run:

```powershell
npm exec vitest run "src/app/web/v1/[...path]/route.test.ts" src/features/files/FilesWorkspace/FilesWorkspace.test.tsx -- --reporter=dot
```

Expected: both test files pass with zero failures.

- [x] **Step 6: Verify the real page and regression suite**

Restart the development server so it reloads `.env.development.local`, then inspect `http://localhost:7158/app/files`. Run `npm test`, `npm run typecheck`, `npm run lint`, and `git diff --check` before reporting completion.
