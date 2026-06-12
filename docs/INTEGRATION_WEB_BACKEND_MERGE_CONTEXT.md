# Integration Web Backend Merge Context

This file is a sanitized handoff for the agent that will merge this branch into
the colleague integration branch/worktree. It contains no secrets and no raw
provider/payment/VK payloads.

## Branches

- Source branch prepared here: `fastlife_dev`
- Push target requested by owner: `origin/integration_web_backend`
- Existing related branch seen locally: `feature/integration-web-backend`

If the actual colleague branch is `feature/integration-web-backend`, merge the
same source changes there as well. Do not assume the underscore and hyphenated
branch names are interchangeable without checking remote branch policy.

## What This Branch Adds

Key commits in this feature chain:

- `6c5d89f1 frontend: add miniapp safety tests`
- `4ec00ae2 frontend: add miniapp smoke tests`
- `f72c8d42 media: add video pipeline metadata config`
- `d33b8bf6 worker: validate video media before delivery`
- `69fc2e40 media: add vk-ready video variants`
- `c581409b worker: deliver safe video variants`
- `0e655832 observability: add media pipeline monitoring`
- `d4fef6df chore: add final audit lint config`

Functional summary:

- Mini App quality gates were added: ESLint, Vitest tests, Playwright smoke
  tests, and safety coverage for telemetry/artifact/localStorage/error states.
- Video/media pipeline backend foundation was added:
  - additive media config;
  - additive artifact media metadata;
  - `ffprobe`-based video validation in worker-owned services;
  - `ffmpeg`-based VK-ready video variants;
  - artifact variants for original and VK-ready media;
  - safe delivery selection that prefers prepared VK-ready variants.
- Delivery and billing were tightened:
  - raw provider video output must not be delivered when media pipeline is on;
  - media probe/transcode failures fail closed;
  - billing capture happens only after safe delivery;
  - retries must not duplicate ledger entries or artifact variants.
- Media lifecycle and observability were added:
  - cleanup for old inactive failed/deleted media only;
  - private bounded-label Prometheus metrics;
  - Grafana panels and Prometheus alerts for media pipeline health;
  - RUNBOOK and `.env.example` updated for operational behavior.
- Final audit support was added:
  - `.golangci.yml` excludes vendored/frontend dependency directories;
  - `gosec` suppressions are narrow and only for operator-configured
    `ffprobe`/`ffmpeg` paths and temp-file reads created by the same flow.

## Must-Preserve Architecture Invariants

- VK Bot and VK Mini App must not call AI providers directly.
- VK Bot and VK Mini App must not call `ffprobe` or `ffmpeg` directly.
- Provider calls and media processing belong to worker/services/adapters.
- Provider adapters must not know about VK delivery or billing.
- VK delivery must go through `internal/adapter/delivery/vk`.
- Billing must stay ledger-based. Do not mutate balances directly.
- Capture billing only after successful safe delivery.
- External inbound events, job creation, delivery, payment processing and
  referral activation must remain idempotent.
- All user-visible outputs must remain artifact-backed and owner-checked.
- Metrics, dashboards, exporters and `/metrics` must stay private.
- Metrics labels must stay bounded. Never label by user id, VK id, job id,
  artifact id, prompt, raw URL, raw error, idempotency key or private storage key.
- Do not log or commit secrets, tokens, auth headers, prompts, full launch
  params, raw PII, raw provider payloads, raw payment payloads, private artifact
  URLs or raw storage/provider URLs.

## Referral Merge Invariants

If the colleague branch contains the shared referral work, keep these rules:

- One stable public referral code per internal user.
- VK Bot and VK Mini App must share the same referral identity.
- Apply registers the relation only; Apply must not grant rewards.
- Activate moves the relation toward activated/rewarded and posts rewards
  through the billing ledger.
- Repeat Activate must be idempotent and must not duplicate ledger entries.
- Self-referral remains forbidden.
- One invited user may be bound to only one referrer.
- Referral DTOs must not expose VK user ids, internal UUIDs or invited-user
  lists.

## Conflict Hotspots

Check these files/directories carefully during merge conflict resolution:

- `migrations/`
  - This branch adds `000016_video_media_metadata.*` and
    `000017_media_cleanup_indexes.*`.
  - Colleague branch may add referral migrations `000013` and `000014`.
  - If migration numbers collide, renumber additively and preserve both schemas.
  - Do not make destructive migrations for existing artifact/referral/payment
    data.
- `internal/domain/artifact.go`
- `internal/domain/provider.go`
- `internal/platform/config/config.go`
- `internal/platform/metrics/metrics.go`
- `internal/service/artifactservice/`
- `internal/service/maintenance/`
- `internal/service/mediaprobe/`
- `internal/service/mediatranscode/`
- `internal/worker/`
- `internal/adapter/storage/postgres/`
- `internal/adapter/storage/s3/`
- `internal/adapter/storage/memory/`
- `internal/adapter/delivery/vk/`
- `internal/adapter/inbound/miniapp/`
- `internal/adapter/inbound/vk/`
- `web/miniapp/package.json`
- `web/miniapp/package-lock.json`
- `web/miniapp/eslint.config.js`
- `web/miniapp/playwright.config.ts`
- `web/miniapp/vitest.config.ts`
- `web/miniapp/src/api/client.ts`
- `web/miniapp/src/chat/`
- `web/miniapp/src/settings/`
- `web/miniapp/src/workflow/`
- `observability/prometheus/rules/product-alerts.yml`
- `observability/grafana/dashboards/jobs-worker.json`
- `.env.example`
- `RUNBOOK.md`
- `docs/ARCHITECTURE.md`

## Required Merge Checks

Run the relevant full checks after resolving conflicts:

```text
gofmt -l .
go test ./...
go vet ./...
golangci-lint run ./...
gosec ./...
govulncheck ./...
gitleaks detect --redact
npm --prefix web/miniapp run build
npm --prefix web/miniapp run lint
npm --prefix web/miniapp run test
npm --prefix web/miniapp run e2e:smoke
promtool check rules observability/prometheus/rules/*.yml
promtool check config observability/prometheus/prometheus.yml
git diff --check
git status --short --branch
```

Run `docker compose config` if compose files change.

Do not push a merge result if relevant checks fail, unless the owner explicitly
accepts the failure list.

## Operational Notes

- Local dev must still work with the media pipeline disabled and without
  `ffprobe`/`ffmpeg`.
- Production-like media pipeline should fail closed when configured tools are
  missing or media is unsafe.
- Do not deliver raw provider video output when a VK-ready variant is required.
- Do not expose original/private storage keys through Mini App DTOs or logs.
- Do not treat payment redirect/confirmation URL as proof of money arrival.
- Payment credit grant remains provider-webhook/provider-verified and
  ledger-backed.
