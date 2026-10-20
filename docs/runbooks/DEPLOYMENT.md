# Deployment Runbook

This is the active runbook for production-shaped deployment.

## Runtime Map

| Runtime | Public | Responsibility |
| --- | --- | --- |
| `cmd/api` | Yes | VK Callback API, Mini App BFF, protected admin/operator routes, health |
| `cmd/worker` | No | AI provider calls, polling, artifacts, moderation, delivery, billing capture/release |
| `cmd/worker` with `WORKER_MODE=relay` | No | leased transactional outbox relay only; no generation, provider-poll, or delivery engines |
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
  -> Docker Images workflow builds immutable sha-<full-main-commit> images in GHCR
  -> operator manually dispatches Deploy Production from main and confirms the relay-only rollout gate
  -> Deploy Production workflow connects to VPS
  -> deploy-prod.sh pulls immutable images
  -> smoke-prod.sh verifies public/private routes
  -> rollback-prod.* may restore previous stateless image tag if smoke fails
```

Do not build images on the VPS unless explicitly debugging a fallback path.
Manual production dispatches in GitHub Actions must be started from the `main`
branch only. `Deploy Production` deploys the checked-out `main` commit by its
immutable `sha-<full commit>` GHCR tag and fails before SSH/env upload if the
matching images have not already been built by `Docker Images`. The VPS checks
out that exact commit in detached mode, and both runtime and backup images are
pinned to the same immutable tag for the rollout.

## Required Production Secrets

Production secrets live in GitHub Repository Secrets and the VPS `.env`, never in
git:

- `ENV_COMMON`
- `ENV_PROVIDERS_COMMON`
- `ENV_SECRETS_PROD`
- `ENV_PAYMENTS_PROD`
- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_SSH_KNOWN_HOSTS`
- `DEPLOY_PATH`
- `GHCR_USERNAME`
- `GHCR_TOKEN`
- optional Telegram notification secrets

Optional production repository variable:

- `PROD_VK_MENU_TOP_UP_ENABLED` — when set to `true` or `false`, replaces
  `VK_MENU_TOP_UP_ENABLED` after split env assembly. With `true`, the VK Bot
  creates quick top-ups only when a server receipt contact exists; otherwise
  its top-up action opens the HTTPS `YOOKASSA_RETURN_URL_MINIAPP`, where the
  payer enters the receipt contact. Use `false` only to hide VK Bot top-up
  entirely.

`ENV_PROVIDERS_COMMON` owns provider routing and model feature flags. The
production preflight requires Hailuo to stay disabled and requires Kling O3
Standard, Runway Gen-4 Turbo, Seedance 2.0 Fast and Runway Gen-4.5 plus their
PoYo/Runway provider readiness.

`DEPLOY_SSH_KNOWN_HOSTS` must contain the production VPS SSH host key line(s)
verified out of band, in OpenSSH `known_hosts` format. The deploy workflow
does not trust live `ssh-keyscan`; if the secret is missing, invalid, does not
contain `DEPLOY_HOST`, or the server presents a different host key, deploy fails
before uploading `.env` or running remote commands.

Before production deploy, compare DEV/PROD variable names:

```bash
bash scripts/deploy/check-env-parity.sh --dev .env.dev --prod .env.prod
```

The script prints variable names only, never values.
Production deploy does not read DEV env secrets. Run DEV/PROD parity as a
separate operator check when changing env structure.

## VPS Deploy Command

Use only when manually operating the VPS:

```bash
cd /opt/vk-ai-aggregator
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env --with-cloudflare --relay-only-workers-upgraded
```

The `--relay-only-workers-upgraded` acknowledgement is a hard compatibility
gate: before it is supplied, upgrade or stop every separately managed
`WORKER_MODE=relay` process. A new API may emit an outbox event that an older
relay cannot classify. The command starts and waits for the new compose jobs
worker before the API, but it cannot discover relay-only processes operated
outside that compose project. Do not use the acknowledgement to permit a
rolling overlap with an old relay.

Expected behavior:

- verifies Docker and env;
- logs in to GHCR with bounded retry if credentials are present;
- pulls compose images with a per-attempt timeout and bounded retry, so a
  stalled registry transfer fails predictably and can enter rollback;
- starts local data services only when `DATA_SERVICES_MODE=local`;
- waits for Postgres/Redis/MinIO health before migrations;
- runs migrations before runtime services;
- starts and waits for the new `worker`, then starts `api`,
  `maintenance-worker`, `provider-webhook`, `miniapp`, `reverse-proxy` and
  optionally `cloudflared`;
- runs health checks and prints a deploy summary.

The active immutable `IMAGE_TAG` in the VPS `.env` is the source of truth when
capturing the previous release. This keeps a retry recoverable when a failed
rollout updated the repository checkout before runtime rollback completed.

## Epifera cutover preparation

Only DEV is currently provisioned. The new Cloudflare zone and certificate
are active. DEV aliases `dev-web`, `dev-vk`, `dev-app` and `dev` on
`epifera.com` use the existing DEV tunnel at `http://127.0.0.1:8088`.
The `vk`, `app` and apex routes for future PROD are prepared, but their tunnel
has no active connector. The imported apex Beget A record still exists;
the future PROD apex CNAME was not created automatically.

The disabled Cloudflare rule `Temporary Epifera website to protected DEV`
redirects GET/HEAD website paths (`/`, `/ru`, `/en` and their descendants)
from apex/www to `dev-web.epifera.com`, preserving path and query with a
temporary 302. Enable it only after the protected new DEV host passes smoke.
It deliberately excludes API/OAuth callbacks and payment webhook POSTs.

Cutover order:

1. Update the default-branch DEV workflow to resolve origins from the source
   Nginx config, retaining compatibility with pre-cutover DEV revisions.
2. Confirm `notify.epifera.com` with the mail provider and configure the exact
   supplied DNS records. Do not assume that the old sender's key or verification
   automatically grants sending rights on the new domain.
3. Update enabled OAuth callbacks/allowed origins, VK launch settings and
   contour-specific YooKassa webhook/return settings. Google remains disabled.
4. Release immutable DEV images, run the full post-deploy smoke and check
   browser login, referral URL generation, uploads and callbacks on the new hosts.
5. Enable the temporary apex redirect and account for pending payments/jobs
   and expiring signed URLs before retiring the old routes and DNS records.

Image-only rollback does not restore the previous hostname contract. If the
cutover fails, keep the old Cloudflare routes and deploy a forward revert of
the domain cutover commit so that the previous Nginx rules, origin checks and
sender configuration are restored together. The unchanged split secrets still
support that source revision. Do not roll database schemas back for this change.

Host-only browser cookies require signing in again on the new host. Preserve
the existing database, account IDs, balances, artifacts and referral codes.
Future PROD also needs a provisioned server and the browser platform/reverse
proxy wiring before the temporary redirect can be removed.

## Public Routes

Expected production routing:

| Public route | Internal target |
| --- | --- |
| `https://vk.epifera.com/webhooks/vk` | `cmd/api:8080` |
| `https://vk.epifera.com/health` | `cmd/api:8080` |
| `https://epifera.com/billing/webhooks/yookassa` | `cmd/provider-webhook:8082` |
| `https://app.epifera.com` | Mini App static frontend |
| `https://app.epifera.com/miniapp/*` | `cmd/api:8080` |

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
