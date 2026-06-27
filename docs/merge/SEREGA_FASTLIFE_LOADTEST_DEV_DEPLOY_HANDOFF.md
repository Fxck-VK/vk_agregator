# Handoff: serega + fastlife_dev + dev-deploy context

Date: 2026-06-23

Purpose: give the next agent enough context about the latest integration work
between `serega`, `fastlife_dev` and `dev-deploy`. This file contains no
secrets, tokens, launch params, provider payloads, prompt bodies or PII.

## Current Branch State

At the time this handoff was written:

- `serega`
- `origin/serega`
- `dev-deploy`
- `origin/dev-deploy`

all point to:

```text
d48c9f5 merge: bring fastlife_dev mock video route into serega
```

`origin/fastlife_dev` latest merged commit:

```text
4027f9c loadtest: add mock-safe video route
```

## Read First

Read these files before continuing:

```text
AGENTS.md
.agents/state.json
docs/LOAD_TESTING.md
docs/SECURITY_SCALE_HARDENING_HANDOFF.md
docs/DEV_CONTOUR.md
RUNBOOK.md
```

If touching video route/provider work, also read:

```text
docs/merge/FASTLIFE_VIDEO_ROUTER_MERGE_GUIDE.md
internal/service/videorouter/catalog.go
internal/service/productcatalog/builder.go
tests/k6/job-worker.js
```

## What Was Done

### 1. Merged fastlife_dev into serega

`origin/fastlife_dev` was merged into `serega`.

Important incoming commit:

```text
4027f9c loadtest: add mock-safe video route
```

Main purpose:

- add safe synthetic video route for load testing;
- allow k6 mixed text/image/video load without real paid provider calls;
- keep production/staging/dev route disabled by default.

### 2. Mock-safe video load-test route

New route alias:

```text
video_mock_text_to_video
```

Feature flag:

```text
FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=true
```

Important safety rules:

- allowed only with `APP_ENV=loadtest`;
- requires mock providers;
- real provider routes remain separate;
- production/staging/dev env templates keep the flag `false`.

Load-test env uses:

```text
K6_JOB_VIDEO_ROUTE_ALIAS=video_mock_text_to_video
```

Rollback:

```text
FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=false
K6_JOB_VIDEO_ROUTE_ALIAS=
```

or replace `K6_JOB_VIDEO_ROUTE_ALIAS` with an explicitly approved real route for
a paid/live smoke test.

### 3. Loadtest diagnostics improvements

The latest `serega` also includes our load-test diagnostics changes:

- Postgres diagnostics now include connection counts, watchlist tables and
  retention/analytics EXPLAIN samples.
- `pg_stat_statements` is enabled for local Docker Postgres when possible.
- Redis diagnostics now report stream length, pending, lag, backlog, DLQ and
  key-pattern TTL/memory metadata.
- Load-test report now distinguishes stream length from real backlog and writes
  scaling decisions into the report JSON/Markdown.
- Added migration:

```text
migrations/000025_postgres_diagnostics_indexes.up.sql
migrations/000025_postgres_diagnostics_indexes.down.sql
```

This migration adds cleanup-oriented indexes for artifacts and artifact
variants, and attempts to enable `pg_stat_statements` where permissions allow.

### 4. Mock payment webhook replay support

Payment mock webhook processing was hardened for loadtest-like multi-process
flows:

- mock `payment.succeeded` can be verified from event + intent when the mock
  provider instance does not have in-memory state;
- mock `payment.canceled`, `payment.expired`, `payment.failed` map to terminal
  intent statuses safely;
- tests were added in `internal/service/paymentservice/service_test.go`.

This is for mock/loadtest behavior. Real YooKassa still must be provider-
verified via `GetPayment`.

### 5. dev-deploy was updated to the same head

`dev-deploy` was fast-forwarded to the same commit as `serega`:

```text
d48c9f5 merge: bring fastlife_dev mock video route into serega
```

That means DEV deploy flow and current integrated project state are aligned.

## Important Changed Files

Recent integration touched these areas:

```text
.env.dev.example
.env.example
.env.loadtest.example
.env.prod.example
.env.staging.example
.github/workflows/deploy-dev.yml
.github/workflows/docker-images.yml
RUNBOOK.md
cmd/api/main.go
cmd/api/main_test.go
docs/LOAD_TESTING.md
docker-compose.data.yml
internal/adapter/inbound/admin/operator_provider_media.go
internal/adapter/inbound/miniapp/handler.go
internal/domain/video_route.go
internal/platform/config/config.go
internal/platform/config/config_test.go
internal/service/paymentservice/service_test.go
internal/service/paymentservice/webhook_processor.go
internal/service/productcatalog/builder.go
internal/service/productcatalog/builder_test.go
internal/service/productcatalog/catalog.go
internal/service/videorouter/catalog.go
internal/service/videorouter/catalog_test.go
migrations/000025_postgres_diagnostics_indexes.*
scripts/deploy/deploy-dev.sh
scripts/deploy/smoke-dev.sh
scripts/loadtest/loadtest-report.ps1
scripts/loadtest/postgres-diagnostics.sql
scripts/loadtest/redis-diagnostics.ps1
tests/k6/job-worker.js
```

## Checks Already Run

During the latest merge/push work, these checks were run locally:

```text
go test ./cmd/api ./internal/platform/config ./internal/service/productcatalog ./internal/service/videorouter ./internal/service/paymentservice
k6 inspect tests/k6/job-worker.js
git diff --check
bash -n scripts/deploy/deploy-dev.sh scripts/deploy/smoke-dev.sh
docker compose -f docker-compose.data.yml config
```

Note: `docker compose config` was run with explicit `COMPOSE_PROJECT_NAME`
because the local workspace path contains Cyrillic characters and Docker Compose
cannot infer a valid project name from it.

## Invariants To Preserve

- VK Bot, Mini App and `cmd/api` must not call AI providers directly.
- Provider calls belong to worker/provider adapters.
- Video route aliases must not expose provider model IDs or pricing to users.
- Load tests must not call real paid providers unless explicitly approved.
- `video_mock_text_to_video` must remain loadtest-only.
- Billing remains ledger-based and idempotent.
- YooKassa top-up credits must come only from provider-verified webhook or
  reconciliation.
- Do not log or commit secrets, tokens, launch params, prompt bodies, raw PII,
  provider payloads or private artifact URLs.

## Known Local Note

There may be a local untracked file:

```text
docs/merge/SEREGA_PRE_FASTLIFE_MERGE_CONTEXT.md
```

It was not included in the last pushes. Treat it as local context unless the
owner explicitly asks to add it.

## Recommended Next Steps

1. If continuing load testing, use `.env.loadtest` and mock providers only.
2. Re-run mixed job workload with `K6_JOB_VIDEO_ROUTE_ALIAS=video_mock_text_to_video`.
3. Produce a full load-test report with Postgres and Redis diagnostics.
4. Decide whether worker count, queue split or indexes need changes based on
   measured bottlenecks, not assumptions.
5. If preparing a main PR, verify GitHub checks and branch protection status.

