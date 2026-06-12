# Admin / Operator UI Temporary Work Plan

This is a temporary execution prompt file. Delete this file in the final cleanup
stage after the work is completed and the final result is summarized in
`TASKS.md`.

## Common Rules For Every Stage

Mode: IMPLEMENT.

Before starting:
- Read `AGENTS.md`.
- Read `.agents/state.json`.
- If touching a directory with local `AGENTS.md`, read it.
- If changing startup/env/deploy behavior, check `RUNBOOK.md`.
- If changing architecture boundaries, check `docs/ARCHITECTURE.md`.

Security:
- Do not log or commit secrets, tokens, auth headers, prompts, full VK launch
  params, raw PII, raw provider payloads, raw payment payloads, private artifact
  URLs, raw URLs, raw stack traces or full idempotency keys.
- Admin UI must not call DB, Redis, S3/MinIO, VK, YooKassa, AI providers,
  ffprobe or ffmpeg directly.
- Admin UI may call only protected backend admin/operator endpoints.
- Billing actions must go through service/ledger paths. Never mutate balance
  directly.
- Provider/media/payment/referral actions must be idempotent and audit-logged.
- Do not expose raw VK user ids, internal UUIDs, private storage keys, raw
  payment/provider payloads or prompts in public UI by default.
- `/metrics`, Grafana, Prometheus, Loki, Tempo, Alertmanager, OTel and
  exporters must remain private.

Product architecture invariants:
- VK Bot and Mini App remain job-creation surfaces only.
- Provider calls stay in workers/provider adapters.
- Media processing stays worker-owned.
- Inbound events, job creation, provider submit/poll, delivery and payment
  flows stay idempotent.
- Frontend never becomes source of truth for auth, balance, billing, moderation,
  job state or delivery state.

Quality:
- Make small scoped diffs.
- Do not touch unrelated changes.
- Keep the UI utilitarian and operational, not marketing-like.
- Prefer read-only screens before mutation actions.
- Add mutation buttons only after backend contract, audit, idempotency and safe
  DTOs are confirmed.
- Every stage should report changed files, checks run/skipped,
  security/architecture impact and `git status --short --branch`.

Commit/push:
- One logical commit per completed stage.
- Commit only after relevant checks are green or skipped checks are explicitly
  explained.
- Push only when the stage prompt explicitly allows push.
- If checks fail, do not push; report exact failures and next fix step.

## Key Pitfalls To Avoid

- Building a UI before backend contract is known.
- Adding frontend-only operator actions that bypass service invariants.
- Exposing raw provider/YooKassa payloads because they are convenient for debug.
- Showing private artifact URLs, storage keys or presigned URLs.
- Treating frontend payment redirect, frontend job status or localStorage as
  trusted truth.
- Allowing admin token to leak into console logs, telemetry, tests, screenshots
  or persistent localStorage.
- Adding broad `CORS: *` or making admin UI public without fail-closed auth.
- Creating high-cardinality metrics labels from user id, job id, artifact id,
  payment id, referral code, raw error, raw URL or idempotency key.
- Adding destructive actions before audit log and idempotency are in place.
- Adding manual balance edits. Use ledger-backed operations only.
- Making provider disable/degrade actions able to disable all providers without
  fallback/degradation state.
- Making requeue/retry buttons that can duplicate paid provider work.
- Showing PII-heavy user search/results by default.
- Storing long-lived admin session secrets in localStorage by default.
- Putting generated plans, handoff docs or archive logs back into the repo after
  completion.

## Stage Admin 0 - Backend Contract Audit

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: map current backend admin/operator capabilities before building UI.

What to do:
- Find current admin/operator/billing endpoints.
- Verify auth model, middleware, route groups and safe DTOs.
- Map readable entities:
  jobs, users, billing ledger/balances, payment intents/events/refunds,
  referrals, providers/circuit breakers, artifacts, alerts/observability status.
- Map existing operator actions:
  payment sync/cancel/refund, referral suspicious/freeze, DLQ/retry/requeue,
  provider/model disable/degrade if present.
- Identify missing backend endpoints and unsafe endpoints.
- Do not build UI yet.

Checks:
- `go test ./internal/adapter/inbound/admin ./internal/adapter/inbound/billing ./internal/platform/config`
- `go vet ./internal/adapter/inbound/admin ./internal/adapter/inbound/billing ./internal/platform/config`
- `git status --short --branch`

Commit:
- No commit unless code/docs changed.

## Stage Admin 1 - UI Placement And Skeleton

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add a minimal operator console shell without unsafe actions.

What to do:
- Decide UI location, preferably `web/admin` if no existing admin app exists.
- Add Vite/React/TypeScript skeleton.
- Keep `web/admin` self-contained with its own `package-lock.json` and required
  Vite build dependencies, including `esbuild`.
- Add layout:
  sidebar, top status bar, content area, route-level error boundary.
- Add placeholder screens:
  Overview, Jobs, Users, Payments, Providers, Media Safety, Referrals, Alerts,
  Audit Log, Config Health.
- Add auth token entry that avoids localStorage by default.
- Do not add live mutation actions.

Checks:
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add operator console skeleton`
- Push only if explicitly allowed.

## Stage Admin 2 - Safe API Client And Error Boundary

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: make all admin UI backend communication safe and typed.

What to do:
- Add typed API client for admin/operator endpoints.
- Add timeout/abort handling.
- Timeout/abort helpers must work in browser and Node test environments; avoid
  browser-only globals when `globalThis` is enough.
- Normalize errors into safe UI errors.
- Ensure token/auth headers never appear in logs, telemetry, UI, test snapshots
  or thrown display text.
- Add idempotency-key helper for future actions, but do not add action buttons
  yet.
- Add tests proving raw payloads, private URLs, prompts and tokens are not
  rendered.

Checks:
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add safe operator api client`
- Push only if explicitly allowed.

## Stage Admin 3 - Read-Only Overview

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: ship the first useful read-only operational dashboard.

What to do:
- Implement Overview screen.
- Show health summaries for API, VK Bot, Mini App, workers,
  provider webhook/payment processing, queue backlog, active alerts,
  provider health, media safety and payment reconciliation.
- If endpoints are missing, add safe read-only backend endpoints.
- DTOs must be bounded and secret-free.
- No mutation actions.

Checks:
- Backend tests/vet for touched packages.
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add read-only overview dashboard`
- Push only if explicitly allowed.

## Stage Admin 4 - Jobs, Workers And Queues

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add read-only visibility into jobs and worker state.

What to do:
- Add Jobs screen with safe filters:
  status, kind, bounded error class, created range, safe public/correlation id.
- Add job detail page/drawer:
  status transitions, reservation/capture state, delivery state, artifact safe
  ids, bounded error class, retry counters.
- Add worker/queue section:
  backlog, oldest age, DLQ count, retry count, degradation state.
- Carry Stage Admin 3 finding forward: Overview currently marks queue/DLQ,
  provider circuit health, alert status and media policy health as `not_wired`
  unless they can be derived from bounded job/payment snapshots. Add dedicated
  read-only backend aggregation before rendering those areas as healthy.
- Do not add requeue/retry buttons yet.

Checks:
- Backend tests/vet if endpoints are added.
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add jobs operator view`
- Push only if explicitly allowed.

## Stage Admin 5 - Payments And Billing

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add safe read-only payment and ledger visibility.

What to do:
- Add Payments screen:
  payment intents, webhook inbox/provider events, reconciliation state,
  refunds, cancel/capture state.
- Add Billing view:
  ledger entries, reservations, balance snapshot.
- Mask idempotency keys and provider ids where needed.
- Never render raw YooKassa payloads, payment method details, auth data or
  provider-native errors.
- No refund/cancel/sync buttons yet.

Checks:
- `go test ./internal/adapter/inbound/billing ./internal/service/paymentservice ./internal/service/billingservice`
- `go vet ./internal/adapter/inbound/billing ./internal/service/paymentservice ./internal/service/billingservice`
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add payments billing console`
- Push only if explicitly allowed.

## Stage Admin 6 - Providers And Media Safety

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add provider/media safety control room.

What to do:
- Add Providers screen:
  provider/model_class health, circuit state, rate-limit state, fallback health,
  provider waste and delivery/capture gap.
- Add Media Safety screen:
  upload rejects, queue pressure, probe/transcode policy, fast path vs fallback,
  invalid provider output, cleanup stats.
- Add config health read-only display for non-secret flags only.
- Use curated provider/model classes, not raw unbounded labels.
- No provider disable/degrade buttons yet.

Checks:
- Backend tests/vet if endpoints are added.
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `promtool check rules observability/prometheus/rules/*.yml` if alerts change.
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add provider media safety console`
- Push only if explicitly allowed.

## Stage Admin 7 - Users, Referrals And Audit Log

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add safe user/referral/operator audit views.

What to do:
- Add Users screen:
  safe user summary, jobs/payments/referrals summary, no raw PII by default.
- Add Referrals screen:
  code stats, suspicious activity, status distribution, no invited-user PII
  lists.
- Add Audit Log screen:
  timestamp, actor safe id, action, target type, target safe id, result,
  request/correlation id.
- If audit model is missing, add additive backend model/endpoint for operator
  actions.
- Keep this stage read-only unless audit infrastructure itself is being added.

Checks:
- Backend tests/vet if endpoints/migrations are added.
- Migration validation if migrations are added.
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add users referrals audit views`
- Push only if explicitly allowed.

## Stage Admin 8 - Safe Operator Actions

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: add limited mutation actions only after backend safety is proven.

What to do:
- Add only actions backed by safe service-layer endpoints:
  payment sync, payment cancel if allowed, full refund MVP if allowed,
  DLQ/job retry if idempotent, provider/model degrade/disable if safe fallback
  exists, referral freeze if backend mutation is implemented safely.
- Each action needs:
  confirmation modal, required reason, idempotency key, audit entry, safe
  success/error message, permission check, loading/disabled state.
- Hide or disable actions whose backend contract is missing or unsafe.
- Never allow manual balance edits.
- Never allow retry that can duplicate paid provider work.

Checks:
- Backend tests for every touched action.
- Backend vet for touched packages.
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add safe operator actions`
- Push only if explicitly allowed.

## Stage Admin 9 - E2E Smoke And Hardening

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: prove admin UI does not leak sensitive data and handles failures safely.

What to do:
- Add Playwright smoke tests with mocked backend:
  token entry, overview load, jobs detail, payments detail, provider/media view,
  action confirmation flow.
- Capture DOM/console/network test output and assert secrets/raw payloads/private
  URLs/prompts are absent.
- Verify token is not persisted in localStorage unless a deliberate documented
  option exists.
- Verify unsafe HTML is not rendered.
- Update RUNBOOK only if admin startup behavior changed.

Checks:
- `npm --prefix web/admin run e2e:smoke`
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run build`
- `go test ./...` if backend changed.
- `go vet ./...` if backend changed.
- `git diff --check`
- `git status --short --branch`

Commit:
- `admin: add operator console smoke tests`
- Push only if explicitly allowed.

## Stage Admin 10 - Final Audit And Push

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: verify the admin/operator console as a production-safe internal tool.

What to do:
- Audit:
  Admin UI does not call DB/VK/YooKassa/providers/S3/Redis directly.
  All data comes through protected backend endpoints.
  Detail views use display-safe ids by default; if protected lookup ids remain
  in frontend state, confirm they are not rendered/logged and decide whether a
  dedicated public job ref/index is needed before broad operator rollout.
  Auth fails closed.
  No secrets/raw payloads/prompts/private URLs/raw PII in UI/logs/tests.
  Billing actions use ledger/service paths.
  Mutations are idempotent and audit-logged.
  High-risk actions require confirmation and reason.
  Admin UI and observability surfaces are not public.
  Docs/RUNBOOK/env match actual startup.
- Check diff for unrelated changes.

Final checks:
- `gofmt -l .`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run ./...` if available.
- `gosec ./...` if available.
- `govulncheck ./...` if available.
- `gitleaks detect --redact` if available.
- `npm --prefix web/admin run lint`
- `npm --prefix web/admin run typecheck`
- `npm --prefix web/admin run test`
- `npm --prefix web/admin run build`
- `npm --prefix web/admin run e2e:smoke`
- `git diff --check`
- `git status --short --branch`

Commit/push:
- If final cleanup is needed, commit `admin: finalize operator console`.
- Push only if relevant checks are green or skipped checks are explained.
- End with a concise summary for the colleague's agent.

## Stage Admin 11 - Delete This Temporary Plan

Apply common rules: read `Common Rules For Every Stage` in
`.agents/admin_operator_ui_tmp_plan.md` before starting this stage and follow
them.

Goal: remove this temporary execution prompt file after the admin/operator UI
work is completed.

What to do:
- Move only the final completed summary and remaining follow-ups into `TASKS.md`.
- Delete `.agents/admin_operator_ui_tmp_plan.md`.
- Ensure `.agents/state.json` does not reference this temp file.
- Ensure no generated prompt plans, handoff docs or archive files remain in the
  active repo surface.

Checks:
- `Get-Content .agents/state.json | ConvertFrom-Json`
- `rg -n "admin_operator_ui_tmp_plan|Admin / Operator UI Temporary Work Plan" .`
- `git diff --check`
- `git status --short --branch`

Commit:
- `docs: remove admin ui temporary plan`
- Push only if explicitly allowed.
