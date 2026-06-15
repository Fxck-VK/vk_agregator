# Cloudflare Deploy Routes

This directory contains Cloudflare deployment notes only. Do not commit real
Cloudflare dashboard tunnel tokens, local credentials JSON files, API tokens or
`cloudflared service install ...` commands with embedded tokens.

## Production Route Map

The production path uses a dashboard-managed Cloudflare Tunnel connector on the
VPS. The connector token is stored only in the real server `.env` as
`CLOUDFLARED_TUNNEL_TOKEN` and is consumed by `docker-compose.prod.yml` through
the container environment variable `TUNNEL_TOKEN`.

| Public hostname/path | Cloudflare Tunnel service | Reverse proxy target |
|---|---|---|
| `vk.neiirohub.ru` | `http://reverse-proxy:80` in Docker, or `http://127.0.0.1:8088` on host | `cmd/api` for `/webhooks/vk` and `/health` |
| `app.neiirohub.ru` | `http://reverse-proxy:80` in Docker, or `http://127.0.0.1:8088` on host | Mini App static for `/`, `cmd/api` for `/miniapp/*` |
| `neiirohub.ru` | `http://reverse-proxy:80` in Docker, or `http://127.0.0.1:8088` on host | `cmd/provider-webhook` only for `/billing/webhooks/yookassa` |

The canonical YooKassa webhook URL is:

```text
https://neiirohub.ru/billing/webhooks/yookassa
```

Do not route broad `/billing/*`, `/admin/*`, `/metrics`, `/debug/*` or private
health/readiness endpoints publicly.

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
