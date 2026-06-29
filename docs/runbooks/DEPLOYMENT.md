# Deployment Runbook

This is the active runbook for production-shaped deployment.

## Runtime Map

| Runtime | Public | Responsibility |
| --- | --- | --- |
| `cmd/api` | Yes | VK Callback API, Mini App BFF, protected admin/operator routes, health |
| `cmd/worker` | No | AI provider calls, polling, artifacts, moderation, delivery, billing capture/release |
| `cmd/worker` with `WORKER_MODE=maintenance` | No | retention cleanup, provider payload redaction, analytics aggregates |
| `cmd/provider-webhook` | Yes, exact route only | YooKassa webhook inbox and provider-verified processing |
| `web/miniapp/dist` | Yes | Static VK Mini App frontend |
| Postgres | No | durable source of truth |
| Redis | No | queues, rate limits, transient state |
| S3/MinIO | No public bucket | generated artifacts and safe media variants |

## Branch And Image Flow

Production flow:

```text
main push/merge
  -> Docker Images workflow builds sha-<commit> images in GHCR
  -> Deploy Production workflow connects to VPS
  -> deploy-prod.sh pulls immutable images
  -> smoke-prod.sh verifies public/private routes
  -> rollback-prod.* may restore previous stateless image tag if smoke fails
```

Do not build images on the VPS unless explicitly debugging a fallback path.

## Required Production Secrets

Production secrets live in GitHub Repository Secrets and the VPS `.env`, never in
git:

- `PROD_ENV_FILE`
- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_PATH`
- `GHCR_USERNAME`
- `GHCR_TOKEN`
- optional Telegram notification secrets

Before production deploy, compare DEV/PROD variable names:

```bash
bash scripts/deploy/check-env-parity.sh --dev .env.dev --prod .env.prod
```

The script prints variable names only, never values.

## VPS Deploy Command

Use only when manually operating the VPS:

```bash
cd /opt/vk-ai-aggregator
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env --with-cloudflare
```

Expected behavior:

- verifies Docker and env;
- logs in to GHCR if credentials are present;
- starts local data services only when `DATA_SERVICES_MODE=local`;
- waits for Postgres/Redis/MinIO health before migrations;
- runs migrations before runtime services;
- starts `api`, `worker`, `maintenance-worker`, `provider-webhook`,
  `miniapp`, `reverse-proxy` and optionally `cloudflared`;
- runs health checks and prints a deploy summary.

## Public Routes

Expected production routing:

| Public route | Internal target |
| --- | --- |
| `https://vk.neiirohub.ru/webhooks/vk` | `cmd/api:8080` |
| `https://vk.neiirohub.ru/health` | `cmd/api:8080` |
| `https://neiirohub.ru/billing/webhooks/yookassa` | `cmd/provider-webhook:8082` |
| `https://app.neiirohub.ru` | Mini App static frontend |
| `https://app.neiirohub.ru/miniapp/*` | `cmd/api:8080` |

Must stay closed publicly:

- `/admin/*`
- `/metrics`
- `/debug/*`
- broad `/billing/*` except exact YooKassa webhook
- internal readiness endpoints unless intentionally exposed for smoke

## Smoke

Run after every deploy:

```bash
bash scripts/deploy/smoke-prod.sh --env-file .env
```

PowerShell equivalent:

```powershell
.\scripts\deploy\smoke-prod.ps1 -EnvFile .env
```

Smoke must verify:

- API health;
- worker health;
- provider-webhook health;
- Mini App availability;
- VK webhook route;
- YooKassa webhook route;
- `/admin/*`, `/metrics`, `/debug/*` are not public.

## Data Services Modes

Supported modes:

| Mode | Meaning |
| --- | --- |
| `local` | Docker Postgres/Redis/MinIO run with the app stack |
| `external` | externally managed by us, deploy script does not start containers |
| `managed` | provider-managed service, deploy script only validates connectivity |

Production can start as `local`, but serious traffic should move Postgres,
Redis and S3-compatible storage out of the app VPS.
