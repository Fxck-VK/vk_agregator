# Local Development Port 7158 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `npm run dev` start the Next.js platform on port `7158`.

**Architecture:** Keep the port in the existing npm script as an explicit Next.js CLI argument. Protect the configuration with the existing packaging assertion script and verify the actual server binding.

**Tech Stack:** Next.js 16, npm, Node.js assertions

## Global Constraints

- Change only the local frontend development command.
- Do not change production, Docker, CI/CD, proxy, API, or deployment ports.
- Do not add dependencies or environment variables.

---

### Task 1: Fix and verify the local development port

**Files:**
- Modify: `web/platform/package.json`
- Test: `web/platform/scripts/assert-packaging.mjs`

**Interfaces:**
- Consumes: the existing `npm run dev` command.
- Produces: a Next.js development server listening on `http://localhost:7158`.

- [ ] **Step 1: Write the failing assertion**

Read `package.json` in `assert-packaging.mjs` and assert that `scripts.dev` equals `next dev --port 7158`.

- [ ] **Step 2: Verify the assertion fails**

Run `npm run test:packaging` from `web/platform` and expect a mismatch between `next dev` and `next dev --port 7158`.

- [ ] **Step 3: Implement the fixed port**

Set the package script to:

```json
"dev": "next dev --port 7158"
```

- [ ] **Step 4: Verify configuration and real startup**

Run `npm run test:packaging`, then run `npm run dev` and confirm the startup output advertises `http://localhost:7158`.

- [ ] **Step 5: Commit**

Stage only `web/platform/package.json` and `web/platform/scripts/assert-packaging.mjs`, then commit the verified change.
