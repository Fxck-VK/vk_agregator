# Fastlife Video Router Merge Guide

Audience: colleague agent merging Fastlife video-provider-router work into `integration_web_backend`.

## Source Branches

- Source branch with this guide: `origin/feauter/integration_web_backend`
- Original working branch: `origin/fastlife_dev`
- Base integration branch to merge into: `origin/integration_web_backend`
- Fastlife router foundation commit: `b27a3a4b Add video provider router foundation`

Do not use VPS during merge. Do not run live paid provider calls during merge verification.

## What This Branch Contains

- Server-side video route catalog and resolver.
- New provider adapters: APIMart, PoYo, official Runway.
- Route-specific billing estimates and immutable route snapshots on jobs.
- Mini App / VK product modes using route aliases, not provider model IDs.
- Admin observability for route enabled/disabled state.
- Prometheus metrics and alert rules for route submits, costs, latency, provider failures, media failures and billing release/capture.
- Dry-run/mocked smoke runner: `scripts/smoke/video-routes.ps1`.

## Merge Procedure

```powershell
git fetch origin
git switch integration_web_backend
git pull --ff-only origin integration_web_backend
git switch -c merge/video-router-into-integration
git merge --no-ff origin/feauter/integration_web_backend
```

If `integration_web_backend` moved after this guide was written, rebase/merge only after reading the new diff. Do not resolve conflicts by deleting router safety checks.

## Conflict Hotspots

Expect conflicts around:

- `.env.example`, `.env.staging.example`, `.env.prod.example`
- `cmd/api/main.go`
- `cmd/worker/main.go`
- `internal/platform/config/config.go`
- `internal/adapter/inbound/miniapp/*`
- `internal/adapter/inbound/vk/*`
- `internal/app/miniapp/module.go`
- `internal/app/vkbot/module.go`
- `internal/service/joborchestrator/orchestrator.go`
- `internal/worker/*`
- `web/miniapp/src/*`
- `docs/ARCHITECTURE.md`

When resolving, keep integration branch production/deploy fixes, but preserve the router invariants below.

## Invariants To Preserve

- `cmd/api`, Mini App BFF and VK handlers must not call video providers directly.
- Provider calls belong to `cmd/worker` through `internal/adapter/provider`.
- Frontend may send only public route aliases, never provider model IDs or prices.
- Backend must resolve route alias server-side before billing reserve.
- Reservation amount must be immutable for a job attempt.
- Technical provider/media failure must release reservation and must not capture.
- Capture only after output is downloaded to our storage and delivery/storage success path is safe.
- Paid fallback remains disabled unless explicitly proven safe later.
- Route feature flags stay disabled by default.
- No raw prompt, provider payload, API key, auth header, launch params, user PII or private provider URL in logs/admin/metrics.

## Provider Env Names

Keep these names:

```env
APIMART_API_KEY=
APIMART_BASE_URL=https://api.apimart.ai/v1
APIMART_PROVIDER_ENABLED=false

POYO_API_KEY=
POYO_BASE_URL=https://api.poyo.ai
POYO_PROVIDER_ENABLED=false

RUNWAYML_API_SECRET=
RUNWAYML_BASE_URL=https://api.dev.runwayml.com/v1
RUNWAY_PROVIDER_ENABLED=false

FEATURE_VIDEO_ROUTER_ENABLED=false
FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=false
FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=false
FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=false
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=false
FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=false
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=false
FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=false
```

Do not put real secrets into tracked files.

## Required Checks After Conflict Resolution

```powershell
gofmt -w <changed-go-files>
go test ./internal/domain ./internal/platform/config ./internal/service/joborchestrator ./internal/worker ./internal/adapter/provider/apimart ./internal/adapter/provider/poyo ./internal/adapter/provider/runway
go test ./...
go vet ./...
npm --prefix web/miniapp run build
npm --prefix web/admin run build
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke/video-routes.ps1 -Mode DryRun
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke/video-routes.ps1 -Mode Mock
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1
git diff --check
```

`validate-infra.ps1` may skip promtool when both local `promtool` and Docker daemon are unavailable. If production observability is being changed, run promtool in CI or on a machine with Docker/Prometheus tooling.

## Live Smoke Gate

Do not enable routes for users after merge. Live smoke requires separate human approval because it can spend provider balance.

Before any live provider smoke, confirm:

- APIMart, PoYo and Runway keys are present only in env/secret storage.
- `POYO_BASE_URL=https://api.poyo.ai` unless PoYo dashboard gives a different official base URL.
- There is explicit approval for paid provider calls.
- Test assets contain no PII.
- Route flags are enabled only for operator-controlled smoke, not general users.

Minimum live smoke per route:

- 5 successful T2V/I2V jobs.
- 2 invalid input jobs.
- 2 timeout/cancel paths if supported.
- 1 insufficient balance/auth failure in sandbox/stub.
- Output downloaded into our storage.
- Media probe/transcode path checked if enabled.
- Billing release on technical failure.
- Billing capture only after delivery/storage success.

Record actual cost, p50/p95 latency, technical failure rate, output URL TTL behavior and support/payment issues before route enablement.
