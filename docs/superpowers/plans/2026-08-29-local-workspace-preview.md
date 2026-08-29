# Local Workspace Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the complete authenticated `/app` workspace locally without the backend stack when an explicit development-only flag is enabled.

**Architecture:** The server-only session loader checks a private preview flag before making API requests. A fixed, non-sensitive `WorkspaceSession` supplies layout data only in development; all other environments retain the existing API-backed flow.

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
- Create locally, ignored by Git: `web/platform/.env.development.local`

**Interfaces:**
- Consumes: `NODE_ENV` and the server-only `NEIROHUB_LOCAL_WORKSPACE_PREVIEW` environment variable.
- Produces: the existing `WorkspaceSession` authenticated variant for layout-only local development.

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

- [ ] **Step 4: Run the focused test and observe GREEN**

Run the focused Vitest command again and expect all `session-data` tests to pass.

- [ ] **Step 5: Enable and verify the local preview**

Create the ignored file `web/platform/.env.development.local` containing:

```text
NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1
```

Restart `npm run dev`, reload `http://localhost:7158/app`, and verify that the workspace content, sidebar, preview account, balance, and conversations are visible while the unavailable fallback is absent.

- [ ] **Step 6: Run regression checks and commit**

Run the full test suite, lint, typecheck, and packaging checks. Stage only the two tracked session files and commit the verified implementation; keep the local env file untracked and ignored.
