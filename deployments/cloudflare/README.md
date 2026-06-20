# Cloudflare Deploy Routes

This directory contains Cloudflare deployment notes only. Do not commit real
Cloudflare dashboard tunnel tokens, local credentials JSON files, API tokens or
`cloudflared service install ...` commands with embedded tokens.

## Production Route Map

The production path uses a dashboard-managed Cloudflare Tunnel connector on the
VPS. The connector token is stored only in the real server `.env` as
`CLOUDFLARED_TUNNEL_TOKEN` and is consumed by `docker-compose.prod.yml` through
the container environment variable `TUNNEL_TOKEN`.

`cloudflared` runs with host networking in production compose. This keeps
dashboard-managed routes compatible with the same origin used by a host-level
connector: `http://127.0.0.1:8088`. The reverse proxy remains bound to VPS
loopback only.

| Public hostname/path | Cloudflare Tunnel service | Reverse proxy target |
|---|---|---|
| `vk.neiirohub.ru` | `http://127.0.0.1:8088` | `cmd/api` for `/webhooks/vk` and `/health` |
| `app.neiirohub.ru` | `http://127.0.0.1:8088` | Mini App static for `/`, `cmd/api` for `/miniapp/*` |
| `neiirohub.ru` | `http://127.0.0.1:8088` | `cmd/provider-webhook` only for `/billing/webhooks/yookassa` |

The canonical YooKassa webhook URL is:

```text
https://neiirohub.ru/billing/webhooks/yookassa
```

Do not route broad `/billing/*`, `/admin/*`, `/metrics`, `/debug/*` or private
health/readiness endpoints publicly.

## Development Tunnel Route Map

For an isolated VK dev community, use a separate dashboard-managed tunnel and
route the dev hostnames to the same loopback reverse-proxy origin:

| Public hostname/path | Cloudflare Tunnel service | Reverse proxy target |
|---|---|---|
| `dev-vk.neiirohub.ru` | `http://127.0.0.1:8088` | `cmd/api` for `/webhooks/vk` and `/health` |
| `dev-app.neiirohub.ru` | `http://127.0.0.1:8088` | Mini App static for `/`, `cmd/api` for `/miniapp/*` |
| `dev.neiirohub.ru` | `http://127.0.0.1:8088` | `cmd/provider-webhook` only for `/billing/webhooks/yookassa` |

The dev VK callback URL is:

```text
https://dev-vk.neiirohub.ru/webhooks/vk
```

The dev YooKassa webhook URL is:

```text
https://dev.neiirohub.ru/billing/webhooks/yookassa
```

### DEV Dashboard Setup

Create or use a separate dashboard-managed tunnel, for example
`neiirohub-vk-dev`. Do not reuse the production tunnel token locally.

Add these published application routes:

| Hostname | Path | Service |
|---|---|---|
| `dev-vk.neiirohub.ru` | `*` | `http://127.0.0.1:8088` |
| `dev-app.neiirohub.ru` | `*` | `http://127.0.0.1:8088` |
| `dev.neiirohub.ru` | `*` | `http://127.0.0.1:8088` |

Use these values in the DEV VK community Callback API settings:

```text
URL: https://dev-vk.neiirohub.ru/webhooks/vk
Secret key: VK_SECRET from the local DEV .env
Confirmation string: VK_CONFIRMATION_TOKEN from the local DEV .env
Events: message_new and any DEV-only event types under test
```

Use these values in the YooKassa test shop webhook settings:

```text
URL: https://dev.neiirohub.ru/billing/webhooks/yookassa
Events: payment.succeeded, payment.canceled, refund.succeeded
```

Local DEV lifecycle:

```powershell
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
.\scripts\dev\smoke-dev.ps1
.\scripts\dev\stop-dev-stack.ps1
```

The standard DEV startup rebuilds app images from the current working tree.
This keeps DEV as a production-shaped copy with different env/tunnel/community
settings, not as a separate stale `:dev` image. `-SkipBuild` requires
`DEV_ALLOW_REMOTE_IMAGES=true` and should only be used for explicit image-tag
smoke testing.

Run only one connector for a given tunnel token on a development machine, and
do not point dev hostnames at production data stores.

The full DEV contour contract is documented in
`docs/DEV_CONTOUR.md`. DEV hostnames, VK community settings, YooKassa webhook
settings and env secrets must stay isolated from production.

## VPS Env

Required values live in the real server `.env`:

```text
PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://neiirohub.ru/billing/webhooks/yookassa
CLOUDFLARED_TUNNEL_TOKEN=<dashboard-managed tunnel token>
CLOUDFLARED_METRICS_PORT=2000
```

The deploy preflight validates these values only when `--with-cloudflare` /
`-WithCloudflare` is used.

## Data Service Modes

Cloudflare routing is the same in both production data modes: every public
hostname still points to the VPS reverse proxy at `http://127.0.0.1:8088`.
Only the private server env and compose selection change.

Single VPS mode:

```text
DATA_SERVICES_MODE=local
POSTGRES_MODE=local
REDIS_MODE=local
S3_MODE=local
```

Deploy starts local Postgres, Redis and MinIO from `docker-compose.data.yml`.

External data services mode:

```text
DATA_SERVICES_MODE=managed
POSTGRES_MODE=managed
REDIS_MODE=managed
S3_MODE=managed
```

Deploy skips local data containers and uses the private `DATABASE_URL`,
`REDIS_ADDR` and `S3_*` endpoints from the server `.env`. Do not expose
Postgres, Redis or S3/MinIO through Cloudflare public hostname routes.

## Deploy

```bash
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env --with-cloudflare
```

```powershell
.\scripts\deploy\deploy-prod.ps1 -Branch main -EnvFile .env -WithCloudflare
```

When Cloudflare is enabled, deploy starts `cloudflared` and then runs the safe
public smoke checks against `PUBLIC_*` URLs. Use `--skip-public-smoke` /
`-SkipPublicSmoke` only for DNS propagation incidents or controlled recovery.
