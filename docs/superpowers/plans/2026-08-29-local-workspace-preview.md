# Local Workspace Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the complete authenticated `/app` workspace locally without the backend stack when an explicit development-only flag is enabled.

**Architecture:** A server-only preview module owns the private flag and fixed non-sensitive layout/catalog data. The session loader uses it before API requests, while the platform BFF serves only the preview image-model catalogue; all other environments and routes retain the existing API-backed flow.

**Tech Stack:** Next.js 16, TypeScript, Vitest, ignored Next.js development environment file

## Global Constraints

- Production, DEV deployment, Docker, CI/CD, authentication, cookies, and API routes remain unchanged.
- Preview activation requires `NODE_ENV=development` and `NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1`.
- Preview values contain no real account identifiers, email addresses, credentials, tokens, or conversations.
- Backend-dependent mutations remain unavailable.
- The existing unavailable state remains unchanged when preview mode is disabled.

---

### Task 1: Add and activate the local workspace preview

**Files:**
- Modify: `web/platform/src/features/session/session-data.ts`
- Test: `web/platform/src/features/session/session-data.test.ts`
- Create: `web/platform/src/features/session/local-workspace-preview.ts`
- Modify: `web/platform/src/app/web/v1/[...path]/route.ts`
- Test: `web/platform/src/app/web/v1/[...path]/route.test.ts`
- Create locally, ignored by Git: `web/platform/.env.development.local`

**Interfaces:**
- Consumes: `NODE_ENV` and the server-only `NEIROHUB_LOCAL_WORKSPACE_PREVIEW` environment variable.
- Produces: the existing `WorkspaceSession` authenticated variant and a read-only `GET /web/v1/image-models` response for layout-only local development.

- [ ] **Step 1: Write failing preview tests**

Add one test that sets both required development values and expects an authenticated preview session without any call to `webServerFetch`. Add another test that sets the flag in production, returns an API `401`, and expects the normal unauthenticated result with an API call.

- [ ] **Step 2: Run the focused test and observe RED**

Run:

```powershell
npm exec -- vitest run src/features/session/session-data.test.ts
```

Expected: the development preview test fails because `loadWorkspaceSession()` still calls the mocked API.

- [ ] **Step 3: Add the minimal server-only preview session**

In `session-data.ts`, define a fixed authenticated `WorkspaceSession` with placeholder UUIDs, `preview@neirohub.local`, balance `1000`, and a small list of placeholder conversations. Return it only when:

```ts
process.env.NODE_ENV === "development" &&
process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW === "1"
```

- [ ] **Step 4: Add the failing catalogue route tests**

Test that `GET /web/v1/image-models` returns four preview models without consulting the internal API only when both preview conditions are true. Test that production still calls the existing internal-origin and proxy functions.

- [ ] **Step 5: Add the minimal read-only preview catalogue**

Move the shared preview flag and fixed data into `local-workspace-preview.ts`. Intercept only an exact development-preview `GET /web/v1/image-models` in the route handler and return the static catalogue with `Cache-Control: no-store`; pass every other request through unchanged.

- [ ] **Step 6: Run the focused tests and observe GREEN**

Run both focused Vitest files and expect all session and route tests to pass.

- [ ] **Step 7: Enable and verify the local preview**

Create the ignored file `web/platform/.env.development.local` containing:

```text
NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1
```

Restart `npm run dev`, reload `http://localhost:7158/app`, and verify that the workspace content, sidebar, preview account, balance, conversations, model selector, shortcuts, and cards are visible while the unavailable fallback is absent.

- [ ] **Step 8: Run regression checks and commit**

Run the full test suite, lint, typecheck, and packaging checks. Stage only the tracked preview/session/route files and commit the verified implementation; keep the local env file untracked and ignored.
