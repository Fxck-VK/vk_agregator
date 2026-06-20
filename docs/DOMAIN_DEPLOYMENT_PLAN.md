# Domain Deployment Plan - neiirohub.ru

This is a planning note for replacing temporary ngrok/localhost.run tunnels with
the owned domain.

## Domain data

- Domain: `neiirohub.ru`
- Cloudflare nameservers:
  - `poppy.ns.cloudflare.com`
  - `zod.ns.cloudflare.com`

## Recommended shape

Use one public HTTPS origin for both the Mini App frontend and the Mini App/VK
BFF routes:

- Mini App URL: `https://neiirohub.ru/`
- Mini App BFF routes: `https://neiirohub.ru/miniapp/*`
- VK callback route: `https://neiirohub.ru/webhooks/vk`
- YooKassa webhook route: `https://neiirohub.ru/billing/webhooks/yookassa`
- Health route: `https://neiirohub.ru/health`

Same-origin frontend/API avoids fragile tunnel domains and reduces VK WebView
CORS/auth problems.

## DNS

In Cloudflare:

- `A neiirohub.ru -> <VPS IPv4>`
- optional: `AAAA neiirohub.ru -> <VPS IPv6>`
- optional: `CNAME www -> neiirohub.ru`

Use Cloudflare proxy if the server TLS/proxy configuration is ready. For early
debugging, DNS-only can make origin issues easier to see.

## Server layout

One simple deployment option:

- Build Mini App:
  - `cd web/miniapp`
  - `npm ci`
  - `npm run build`
- Serve static files from:
  - `web/miniapp/dist`
- Run Go API:
  - `go run ./cmd/api` or a service binary
- Run worker:
  - `go run ./cmd/worker` or a service binary
- Run payment provider webhook runtime:
  - `go run ./cmd/provider-webhook` or a service binary
- Use Caddy or nginx:
  - `/` serves Mini App static files.
  - `/miniapp/*`, `/webhooks/*`, `/health`, `/metrics` proxy to Go API.
  - `/billing/webhooks/yookassa` proxies to `cmd/provider-webhook`
    (`PAYMENT_WEBHOOK_ADDR`, default `:8082`).

## Data service modes

The domain/reverse-proxy map does not depend on where data lives. Production
supports two data placement modes.

Single VPS mode:

- `docker-compose.prod.yml` runs stateless app services.
- `docker-compose.data.yml` runs Postgres, Redis and MinIO on the same VPS.
- `DATABASE_URL=...@postgres:5432/...`, `REDIS_ADDR=redis:6379`,
  `S3_ENDPOINT=minio:9000`.

External data services mode:

- `docker-compose.prod.yml` runs only app/runtime services.
- Postgres, Redis and S3-compatible storage are external or managed services.
- `DATABASE_URL`, `REDIS_ADDR` and `S3_*` point to private external endpoints.
- Public Cloudflare routes still point only to the reverse proxy; data services
  must not be exposed as public hostnames.

## Example Caddy shape

Do not paste secrets into this file. Replace paths and ports per deployment.

```caddyfile
neiirohub.ru {
  encode zstd gzip

  root * /srv/neiirohub/web/miniapp/dist
  try_files {path} /index.html
  file_server

  handle_path /miniapp/* {
    reverse_proxy 127.0.0.1:8080
  }

  handle_path /webhooks/* {
    reverse_proxy 127.0.0.1:8080
  }

  handle /billing/webhooks/yookassa {
    reverse_proxy 127.0.0.1:8082
  }

  handle /health {
    reverse_proxy 127.0.0.1:8080
  }
}
```

Validate exact route handling before production. If `handle_path` strips a
prefix the API expects, use `handle` instead.

## Required runtime configuration

Values live in the server environment or secret manager, not in git:

- Database DSN
- VK Mini App secret / service token values
- VK bot group token / confirmation settings
- Provider keys such as DeepInfra
- YooKassa shop id/secret and payment webhook runtime settings
- Public app URL set to `https://neiirohub.ru`
- CORS/allowed origin set narrowly to `https://neiirohub.ru`

Do not commit `.env`, `.env.ps1` or copied production config.

## VK settings

In VK app/community settings:

- Mini App address: `https://neiirohub.ru/`
- Callback API endpoint: `https://neiirohub.ru/webhooks/vk`
- Ensure the backend has the same VK app/group secrets that VK uses to sign
  launch params and callbacks.

## Smoke checklist

After deployment:

1. Open `https://neiirohub.ru/` in a normal browser and verify assets load.
2. Open the Mini App from VK and verify it passes launch params.
3. Verify `GET /miniapp/balance` through the Mini App succeeds.
4. Verify `POST /miniapp/estimate` succeeds.
5. Verify chat request creates a backend job and polls to terminal state.
6. Verify Create Photo/Video flows still use backend estimate and job polling.
7. Verify VK callback confirmation / message handling still works.
8. Verify YooKassa dashboard webhooks reach
   `https://neiirohub.ru/billing/webhooks/yookassa` and are processed by
   `cmd/provider-webhook`, not `cmd/api`.

## Known follow-ups

- Add a production service unit or container deployment manifest.
- Add explicit production service wiring for `cmd/provider-webhook` and its
  health/metrics checks.
