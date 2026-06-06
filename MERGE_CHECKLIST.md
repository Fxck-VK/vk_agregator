# Merge Checklist

Use this before merging another branch into `fastlife_dev` or merging
`fastlife_dev` forward.

## 1. Preflight

- Confirm branch:
  - `git branch --show-current`
  - expected: `fastlife_dev` unless the human explicitly requested another
    target.
- Refresh refs:
  - `git fetch origin`
- Inspect local changes:
  - `git status --short`
- Expected current dirty item as of this handoff:
  - `M start-miniapp-ngrok.ps1`
- Do not stage or commit `.env`, `.env.ps1`, real credentials, run logs,
  private media, tunnel output or generated secrets.

## 2. Read first

Read in this order:

1. `AGENTS.md`
2. `MERGE_HANDOFF.md`
3. `DECISIONS.md` recent ADRs
4. `PROGRESS.md` recent entries
5. `TASKS.md` open backend dependencies
6. `docs/DOMAIN_DEPLOYMENT_PLAN.md`

Do not bulk-read the whole repository unless the merge conflict requires it.

## 3. Merge strategy

- Merge only the branch explicitly requested by the human.
- Keep `feature/vk-miniapp` untouched unless the human explicitly names it.
- Prefer preserving the committed product state from `fastlife_dev`:
  - Chat tab remains a chat-like AI surface.
  - Create tab currently exposes photo/video generation only.
  - Create Post is disabled/removed for now.
  - Settings owns theme, balance display, local privacy controls and summary
    generation history.
- If conflicts touch backend billing/auth/provider boundaries, stop and inspect
  the relevant `docs/AGENTS_FULL.md` sections before resolving.

## 4. Conflict decision notes

- `start-miniapp-ngrok.ps1`:
  - Current local diff changes dev tunnel behavior to `localhost.run`.
  - It is not part of pushed head `3b26d43`.
  - Decide explicitly: keep, revert or replace with domain deployment.
- `web/miniapp/package-lock.json`:
  - Keep it when dependencies change. It locks exact npm dependency versions
    and should normally be committed together with `package.json`.
- Chat model/provider labels:
  - Public UI may say `ChatGPT` or `NeiroHub` depending on current UX decision,
    but must not leak raw provider/model names unless intentionally documented.
- Payment/top-up:
  - Frontend must not mutate balance locally. Missing backend payment endpoints
    are dependencies, not frontend shortcuts.
- Launch params:
  - Do not remove backend verification. Plain-browser testing must use dev
    launch params or a proper VK context.

## 5. Checks after conflict resolution

Frontend:

- `cd web/miniapp`
- `npm run build`

Backend:

- `go test ./...`
- `go build ./...`

Focused checks if Mini App auth files changed:

- `go test ./internal/adapter/inbound/miniapp`

Optional local smoke:

- Start API, worker and Vite with real local env.
- Open the Mini App with valid VK launch params.
- Verify:
  - `GET /miniapp/balance` returns backend balance.
  - `POST /miniapp/estimate` returns backend estimate.
  - Chat sends through backend job path.
  - Create Photo/Video creates jobs and polls to a terminal state.

## 6. Stop conditions

Stop and ask the human before continuing if a conflict resolution would require:

- Disabling auth, launch-param verification, moderation, billing or idempotency.
- Direct provider calls from Mini App or VK inbound handlers.
- Frontend-side balance mutation or trusted billing state.
- Logging secrets, launch params, prompts, PII or private artifact URLs.
- Committing `.env`, `.env.ps1` or real credentials.
- Broad production `CORS: *`.
- Ignoring failing checks.
- Deleting data or running destructive migrations.

## 7. Final merge report

Report:

- Branch merged and target branch.
- Files with non-trivial conflict decisions.
- Checks run and result.
- Known remaining breakage or dependencies.
- Whether anything was intentionally left uncommitted.

## 8. 2026-06-06 merge execution notes

- Target reset from `origin/feature/integration-web-backend` at `44df8d4`.
- Source `fastlife_dev` was first cleaned with commit
  `f5f4873 chore: merge docs + dev script`.
- Backup branch created: `backup/pre-merge-integration-web-backend`.
- Merge command: `git merge --no-ff fastlife_dev`.
- Content conflicts:
  - `TASKS.md` - combined both sides' follow-ups.
  - `internal/worker/worker_test.go` - unified test harnesses to keep both
    text-context and provider-timeout/reservation-release tests.
- Auto-merged files manually reviewed:
  - `cmd/api/main.go`
  - `internal/worker/worker.go`
  - `internal/worker/generation.go`
  - `internal/service/billingservice/service.go`
  - Mini App BFF files
  - docs/progress files
- Checks completed before merge commit:
  - conflict marker scan - clean
  - `gofmt -l .` - exit 0
  - `go build ./...` - exit 0
  - `go test ./...` - exit 0
  - `npm --prefix web/miniapp run build` - exit 0
