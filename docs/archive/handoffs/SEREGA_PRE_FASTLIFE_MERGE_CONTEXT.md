# Handoff: serega Context Before fastlife_dev Merge

Date: 2026-06-22

Purpose: give the next agent enough context about what was done on `serega`
before merging `origin/fastlife_dev`, and what was preserved during the merge.
This file contains no secrets, tokens, raw prompts, launch params, provider
payloads or PII.

Current state after merge:

- branch: `serega`
- merge commit: `b0bbb11 Merge remote-tracking branch 'origin/fastlife_dev' into serega`
- pushed to: `origin/serega`
- source merged in: `origin/fastlife_dev`

## Read First

Before doing more work, read:

- `AGENTS.md`
- `.agents/state.json`
- this file
- `docs/FASTLIFE_POST_MERGE_CONTEXT.md`
- `docs/LOAD_TESTING.md`, if touching load tests
- `docs/DATA_SERVICES_CONTRACT.md`, if touching deploy/data services
- `docs/DEV_CONTOUR.md`, if touching local DEV
- `RUNBOOK.md`, if touching deploy/env/runtime

Do not read logs/archive by default. Do not commit `.env`, `.runtime`, reports,
tokens, keys, passwords, launch params, raw provider payloads, raw prompts or PII.

## What serega Had Before The Merge

### 1. Load-Test Foundation

We added a safe load-test contour. It is explicitly not production traffic and
must use mock providers/payments by default.

Important files:

- `.env.loadtest.example`
- `docs/LOAD_TESTING.md`
- `tests/k6/basic-api.js`
- `tests/k6/vk-bot.js`
- `tests/k6/job-worker.js`
- `tests/k6/billing-payments.js`
- `scripts/loadtest/loadtest-report.ps1`
- `scripts/loadtest/postgres-diagnostics.ps1`
- `scripts/loadtest/postgres-diagnostics.sql`
- `scripts/loadtest/redis-diagnostics.ps1`

Safe load-test defaults:

```text
APP_ENV=loadtest
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
PAYMENT_PROVIDER=mock
VK_DELIVERY_MODE=mock
MODERATION_PROVIDER=keyword
ARTIFACT_SCANNER=none
```

Last local load-test result:

- basic API, VK bot and billing mock paths were fast and green;
- `job-worker` scenario had an error rate around `12.2%`;
- Redis queue depth grew to about `438`;
- CPU/RAM/Postgres/Redis were not saturated;
- first bottleneck candidate is job/worker path plus delivery backlog;
- the run is not a final capacity limit because the video job fixture/route needs cleanup first.

Next load-test step:

1. fix or replace the video mock route in `tests/k6/job-worker.js`;
2. reset loadtest Redis/DB before the next official run;
3. run text-only, image-only, video-only and mixed jobs separately;
4. repeat mixed workload with two worker instances;
5. compare queue depth, throughput and p95/p99.

Local report directories under `reports/loadtest/**` are ignored and should not
be committed.

### 2. Data Services Prepared For Local Or External Storage

We prepared the code/deploy model so Postgres, Redis and S3/MinIO can either
live on the same VPS or be moved to external/managed services later.

Important files:

- `docs/DATA_SERVICES_CONTRACT.md`
- `.env.prod.example`
- `.env.dev.example`
- `.env.loadtest.example`
- `docker-compose.prod.yml`
- `docker-compose.data.yml`
- `scripts/deploy/deploy-prod.sh`
- `scripts/deploy/deploy-prod.ps1`
- `scripts/deploy/check-prod-env.sh`
- `scripts/deploy/check-prod-env.ps1`
- `scripts/deploy/rollback-prod.sh`
- `scripts/deploy/rollback-prod.ps1`

Important env contract:

```text
DATA_SERVICES_MODE=local|external|managed
POSTGRES_MODE=local|external|managed
REDIS_MODE=local|external|managed
S3_MODE=local|external|managed
```

Meaning:

- `local`: deploy starts Docker Postgres/Redis/MinIO from `docker-compose.data.yml`;
- `external`: service is self-managed outside this compose project;
- `managed`: provider-managed service such as managed Postgres, managed Redis or S3-compatible object storage.

Do not hardcode local Docker service names where external/managed mode should
work. Keep `DATABASE_URL`, `REDIS_ADDR`, `S3_ENDPOINT`, `S3_REGION` and
`S3_ADDRESSING_STYLE` configurable.

### 3. Retention, Artifact Lifecycle And Analytics Foundations

We added the foundation for not storing hot user content forever and for keeping
analytics separate from raw operational tables.

Main ideas:

- financial ledger/payment history is not automatically deleted;
- job metadata can be retained longer than raw provider/log payloads;
- conversation raw messages/prompts must be retained for limited time;
- summaries can live longer than raw messages;
- artifacts have lifecycle rules by tier/type;
- analytics should be aggregated into daily aggregate tables instead of dashboards querying raw hot tables.

Important areas:

- `internal/service/maintenance/`
- retention-related config in `internal/platform/config/config.go`
- maintenance/runtime wiring in worker/deploy files
- docs updated around data services and retention policy

Important invariant:

```text
ledger/payments are audit data; do not auto-delete them.
```

### 4. Production Deploy And CI Guardrails

Before the merge, `serega` already had production deployment automation:

- Docker images built by GitHub Actions into GHCR;
- production deploy pulls immutable images;
- VPS build is fallback only;
- post-deploy smoke is mandatory;
- rollback can switch stateless containers back to previous image tag;
- schema rollback is never automatic;
- deployment secrets live only in GitHub Secrets and VPS `.env`.

Important files:

- `.github/workflows/docker-images.yml`
- `.github/workflows/deploy-prod.yml`
- `scripts/deploy/deploy-prod.*`
- `scripts/deploy/rollback-prod.*`
- `scripts/deploy/smoke-prod.*`
- `scripts/ci/validate-infra.ps1`
- `.gitleaks.toml`

During the `fastlife_dev` merge we preserved these guardrails and made two
small compatibility fixes:

- `.env.loadtest.example` is now allowed as a tracked env template;
- `validate-infra.ps1` now expects `maintenance-worker` in production runtime checks.

### 5. DEV Contour

We had a production-shaped local DEV contour:

```text
dev-vk.neiirohub.ru  -> local reverse proxy :8088
dev-app.neiirohub.ru -> local reverse proxy :8088
dev.neiirohub.ru     -> local reverse proxy :8088
```

The local reverse proxy then routes to API, Mini App frontend and provider
webhook services. DEV must use a separate VK community and separate Cloudflare
tunnel. Production VK callback and production tunnel must not be touched for
local tests.

Important files:

- `.env.dev.example`
- `docs/DEV_CONTOUR.md`
- `scripts/dev/start-dev-stack.ps1`
- `scripts/dev/status-dev-stack.ps1`
- `scripts/dev/stop-dev-stack.ps1`
- `scripts/dev/smoke-dev.ps1`

After merge, `.env.dev.example` must keep both:

```text
DATA_SERVICES_MODE=local
POSTGRES_MODE=local
REDIS_MODE=local
S3_MODE=local
S3_REGION=us-east-1
S3_ADDRESSING_STYLE=path
```

and the fastlife/product-provider related settings.

## What fastlife_dev Brought In

Read `docs/FASTLIFE_POST_MERGE_CONTEXT.md` for the detailed source context.
High-level merge-sensitive points:

- Product Catalog is now the shared source of truth for Mini App and VK Bot;
- do not reintroduce `/miniapp/image-models` or `/miniapp/video-routes`;
- use `/miniapp/model-catalog`;
- UI/API payloads must expose aliases/options, not provider internals;
- provider ids/model codes/resolved snapshots stay backend/worker-side;
- APIMart, PoYo and Runway adapters were expanded;
- route-specific billing estimates and immutable route snapshots were added;
- Mini App and VK product modes use route aliases.

## Merge Resolution Notes

During merge `origin/fastlife_dev` into `serega`, conflicts were resolved with
this policy:

- keep fastlife product catalog / route resolver / provider adapter logic;
- keep `serega` deploy, loadtest, data-services and retention guardrails;
- preserve both sets of reusable known-error entries in `.agents/logs/errors.jsonl`;
- do not expose provider internals to public Mini App/VK responses;
- do not weaken billing, idempotency, artifact ownership or worker boundaries.

Files that were conflict hot zones:

- `.agents/logs/errors.jsonl`
- `.env.dev.example`
- `.gitignore`
- `docs/DEV_CONTOUR.md`
- `internal/platform/config/config.go`
- `internal/adapter/inbound/vk/*`
- `internal/app/vkbot/*`
- `internal/service/commandrouter/*`
- `scripts/dev/*`
- `scripts/ci/validate-infra.ps1`

## Checks Run After Merge

These passed after resolving conflicts:

```text
go test ./internal/platform/config ./internal/service/commandrouter ./internal/adapter/inbound/vk ./internal/app/vkbot ./internal/service/productcatalog ./internal/service/modelcatalog ./internal/service/videorouter ./internal/worker ./internal/adapter/provider/apimart ./internal/adapter/provider/poyo ./internal/adapter/provider/runway
go test ./...
go vet ./...
npm --prefix web/miniapp run build
npm --prefix web/admin run build
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1
git diff --check
gitleaks detect --redact --no-banner --no-color --config .gitleaks.toml
```

## Follow-Up For The Next Agent

Recommended next work:

1. Run CI on GitHub for `serega`.
2. If preparing main release, open PR `serega -> main` and wait for required approval.
3. Before more load testing, fix the `job-worker` video mock route/fixture.
4. Keep real provider load separate from k6 load tests. Real FAL/Runway/APIMart/PoYo/DeepInfra checks must be low-volume live smoke only.
5. If touching Mini App model selection, preserve Product Catalog as the public source and keep provider details hidden.
6. If touching VK Bot model menus, use catalog/aliases and keep real provider calls in worker.

## Security Reminders

- No direct provider calls from VK Bot, Mini App BFF or `cmd/api`.
- Billing remains append-only ledger based.
- Capture only after successful storage/delivery where applicable.
- Technical failures release reservations.
- YooKassa top-up credits only after provider-verified webhook/reconciliation.
- Artifact access must stay owner-checked.
- `/admin`, `/metrics`, `/debug` must not be publicly exposed.
- No secrets or raw provider/payment/user payloads in git, logs or docs.
