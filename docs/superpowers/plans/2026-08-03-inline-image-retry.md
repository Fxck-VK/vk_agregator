# Inline Image Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retry an expired image request in-place while preserving account ownership, idempotency, atomic billing and responsive UI behavior.

**Architecture:** Add a narrowly scoped Web retry route backed by a new orchestrator method. The method reads only the account-owned expired job, derives server-owned retry metadata, and reuses the transactional `CreateJob` path so new job, reservation and outbox queue event commit together. The files UI replaces only the affected job and polls only that job until terminal.

**Tech Stack:** Go, PostgreSQL/unit-of-work transactional outbox, Next.js/React, TypeScript, Vitest.

## Global Constraints

- The original expired job is never activated, changed or charged.
- A retry preserves trusted prompt, model, quality, input artifacts and stored price snapshot.
- Retry is idempotent across repeated clicks and tabs for one original job.
- No provider or raw internal error data appears in browser DTOs.
- UI remains in `/app/files`, starts immediately, and never reloads the entire file list for one retry.
- Poll only the returned retry job, stop at terminal status, and respect hidden-tab behavior.
- Keep account ownership and `ResultModeAccountHistory`; do not introduce VK fields.

---

### Task 1: Transactional retry orchestration and Web endpoint

**Files:**
- Modify: `internal/service/joborchestrator/orchestrator.go`
- Modify: `internal/service/joborchestrator/account_activation_test.go`
- Modify: `internal/adapter/inbound/websession/handler.go`
- Modify: `internal/adapter/inbound/websession/image_jobs_test.go`

**Interfaces:**
- Produces `RetryExpiredAccountImageJob(ctx context.Context, accountID, originalJobID uuid.UUID) (*domain.Job, error)` on the image job service.
- Produces `POST /web/v1/image-jobs/{jobID}/retry` returning `safeImageJobActivation`.

- [ ] **Step 1: Write failing orchestration tests**

Add tests showing that one retry of an account-owned expired confirmation creates a new queued account-history job with a new ID, one reservation and one queue outbox event; then call it again and assert the same retry job is returned without a second reservation. Add a separate insufficient-credit test that returns an `awaiting_payment` job without a queue event.

- [ ] **Step 2: Run the new orchestration tests to verify they fail**

Run: `go test ./internal/service/joborchestrator -run RetryExpired -count=1`

Expected: FAIL because the retry operation does not exist.

- [ ] **Step 3: Implement the minimal orchestrator method**

Validate the original job through account scope and require Web/image/account-history plus `expired` and `prepared_confirmation_expired`. Decode its stored pricing snapshot, derive a deterministic retry idempotency key from its UUID, and call the existing atomic `CreateJob` path with copied trusted data. If the existing deterministic retry is `awaiting_payment`, re-use activation so a later retry after top-up can queue it safely.

- [ ] **Step 4: Run the orchestration tests to verify they pass**

Run: `go test ./internal/service/joborchestrator -run RetryExpired -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing Web handler tests**

Add tests for the new POST route verifying account ID propagation, safe `200` queued DTO, safe `402` payment DTO, and no route response for an unavailable service.

- [ ] **Step 6: Run the new handler tests to verify they fail**

Run: `go test ./internal/adapter/inbound/websession -run ImageJobRetry -count=1`

Expected: FAIL because the route and service method are missing.

- [ ] **Step 7: Add route and response mapping**

Extend the existing image job service interface, register the protected POST route, map domain insufficient credits to `402`, safe owned absence to `404`, retryability conflicts to `409`, and all other internal errors to `503`.

- [ ] **Step 8: Run the handler tests to verify they pass**

Run: `go test ./internal/adapter/inbound/websession -run ImageJobRetry -count=1`

Expected: PASS.

### Task 2: Inline card retry and job-specific polling

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`
- Modify: `web/platform/src/features/files/FilesGrid/FilesGrid.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/files-data.ts`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes `POST /web/v1/image-jobs/{jobID}/retry` and `GET /web/v1/image-jobs/{jobID}`.
- `FileCard` accepts `isRetrying` and `onRetryJob` props.

- [ ] **Step 1: Write failing component tests**

Add tests that click `Повторить` without navigation, show an in-card busy spinner while the mutation is unresolved, replace the old card with the returned queued job, poll only that ID, and render the insufficient-balance description after a `402` response.

- [ ] **Step 2: Run the files workspace test to verify it fails**

Run: `npm.cmd test -- src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: FAIL because the inline retry control and request do not exist.

- [ ] **Step 3: Implement the client mutation and targeted state update**

Replace the expired-card link with a button. Add a client data helper that performs a CSRF mutation and validates the existing safe activation DTO. In `FilesWorkspace`, disable only the clicked card, replace that exact local job on success or payment-required response, and schedule a bounded existing-cadence status poll for the replacement job. Do not invalidate the first-page cache or re-fetch all files.

- [ ] **Step 4: Run the files workspace test to verify it passes**

Run: `npm.cmd test -- src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: PASS.

### Task 3: Verification and DEV rollout

**Files:**
- No product source files expected.

- [ ] **Step 1: Format and run focused suites**

Run: `gofmt -w internal/service/joborchestrator/orchestrator.go internal/service/joborchestrator/account_activation_test.go internal/adapter/inbound/websession/handler.go internal/adapter/inbound/websession/image_jobs_test.go`

Run: `go test ./internal/service/joborchestrator ./internal/adapter/inbound/websession`

Run: `npm.cmd test -- src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

- [ ] **Step 2: Run full checks**

Run: `go test ./...`

Run: `npm.cmd run typecheck`

Run: `npm.cmd run lint`

Run: `npm.cmd run build`

- [ ] **Step 3: Commit and deploy**

Commit all focused changes, push `HEAD` to `origin/dev-deploy`, wait for the image build, dispatch the existing `Deploy DEV` workflow from `dev-deploy`, and confirm its successful conclusion before giving the test URL.
