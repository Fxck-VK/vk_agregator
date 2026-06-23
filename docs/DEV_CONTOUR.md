# DEV Contour

This document defines the isolated development contour for local/manual VK and
Mini App testing. It is intentionally production-shaped, but it must never use
production secrets, production VK community settings, production YooKassa
webhooks or production data stores.

## Purpose

The DEV contour exists so local development can exercise the same public
surface shape as production:

- VK Callback API
- VK Mini App frontend
- Mini App BFF
- YooKassa webhook entrypoint
- reverse proxy routing

The implementation must keep the same architecture boundaries as production:
VK Bot and Mini App create backend requests/jobs only; providers are called
only by workers; payments are completed only through provider-verified
webhook/reconciliation and billing ledger.

## Public Hosts

| Surface | DEV URL | Production URL |
|---|---|---|
| VK Callback API | `https://dev-vk.neiirohub.ru/webhooks/vk` | `https://vk.neiirohub.ru/webhooks/vk` |
| Mini App | `https://dev-app.neiirohub.ru` | `https://app.neiirohub.ru` |
| Mini App BFF | `https://dev-app.neiirohub.ru/miniapp/*` | `https://app.neiirohub.ru/miniapp/*` |
| YooKassa webhook | `https://dev.neiirohub.ru/billing/webhooks/yookassa` | `https://neiirohub.ru/billing/webhooks/yookassa` |

All DEV hostnames should point through a separate dashboard-managed Cloudflare
Tunnel to the same local reverse-proxy origin:

```text
http://127.0.0.1:8088
```

The reverse proxy is responsible for routing by hostname/path to the local
services:

| DEV hostname/path | Target service |
|---|---|
| `dev-vk.neiirohub.ru/webhooks/vk` | `cmd/api` |
| `dev-vk.neiirohub.ru/health` | `cmd/api` |
| `dev-app.neiirohub.ru/` | Mini App frontend |
| `dev-app.neiirohub.ru/miniapp/*` | `cmd/api` |
| `dev.neiirohub.ru/billing/webhooks/yookassa` | `cmd/provider-webhook` |

The local reverse proxy listens on `127.0.0.1:8088`. Cloudflare must not route
directly to `cmd/api`, `cmd/provider-webhook` or the Mini App dev server; all
three DEV hostnames go to the same reverse proxy origin and are separated by
`Host` header plus path.

After the local services and DEV Cloudflare tunnel are running, verify routing
from the workstation:

```powershell
.\scripts\dev\check-dev-reverse-proxy.ps1
.\scripts\dev\smoke-dev.ps1
```

The script sends local requests to `http://127.0.0.1:8088` with DEV `Host`
headers. It expects the API/Mini App/webhook paths to be routed and sensitive
paths such as `/admin/*` and `/metrics` to stay blocked.

`smoke-dev.ps1` verifies the public DEV Cloudflare routes over HTTPS. It checks
the DEV VK health/callback route, Mini App frontend and BFF, YooKassa webhook
entrypoint, and confirms that `/admin/*` and `/metrics` are not publicly open.
It refuses non-HTTPS URLs and production hostnames.

## Mini App Model Catalog Migration

Mini App model selection uses one public BFF endpoint:

```text
GET /miniapp/model-catalog
```

The old catalog endpoints were removed and must keep returning `404`:

```text
GET /miniapp/image-models
GET /miniapp/video-routes
```

The Mini App frontend is already migrated to `/miniapp/model-catalog`. Keep
Cloudflare routes, VK Mini App settings, smoke scripts and external monitors off
the removed endpoints. Direct unauthenticated probes to `/miniapp/model-catalog`
may return `401`; that still means the route exists behind Mini App auth.

## Start Command

Start the full local DEV contour with one command:

```powershell
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
```

Normal DEV startup builds runtime images from the current working tree. This
keeps DEV and production architecture identical while changing only env,
Cloudflare tunnel and external test community/provider settings. Do not use
prebuilt `:dev` images for regular local development.

What it does:

- checks that `dev.env` exists and is a DEV environment, not production;
- refuses `-SkipBuild` unless `DEV_ALLOW_REMOTE_IMAGES=true` is deliberately
  set for image-tag smoke testing;
- loads `dev.env` into the current process for Docker Compose interpolation;
- starts local Docker Postgres, Redis and MinIO;
- runs migrations through the existing `migrate` service;
- starts `cmd/api`, `cmd/worker`, `cmd/provider-webhook`, Mini App and the
  reverse proxy;
- starts the local `cloudflared` connector when `-WithCloudflare` is passed;
- verifies local service health and reverse-proxy routing;
- verifies public DEV hosts unless `-SkipPublicSmoke` is passed.

Useful commands:

```powershell
.\scripts\dev\start-dev-stack.ps1 -StatusOnly
.\scripts\dev\start-dev-stack.ps1 -StopOnly
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare -SkipPublicSmoke
```

`-SkipBuild` is not a normal DEV command. It is blocked by default because it
can run an already-built GHCR image that differs from the local project. Use it
only for explicit image smoke testing with `DEV_ALLOW_REMOTE_IMAGES=true` and a
known `IMAGE_TAG`.

Dedicated wrappers:

```powershell
.\scripts\dev\status-dev-stack.ps1
.\scripts\dev\stop-dev-stack.ps1
.\scripts\dev\smoke-dev.ps1
```

`status-dev-stack.ps1` is read-only. It prints Docker Compose services, local
ports, local health endpoints, DEV reverse-proxy routes, VK callback
confirmation behavior, Mini App reachability and tunnel/public route status. It
does not print VK tokens, tunnel tokens or other secrets.

`stop-dev-stack.ps1` stops the local DEV stack through the existing start script
and preserves Docker volumes. It does not delete local Postgres/Redis/MinIO
data and does not touch production infrastructure.

`smoke-dev.ps1` is the public Cloudflare smoke check for DEV. Run it after the
DEV stack and DEV tunnel are up to verify:

```text
https://dev-vk.neiirohub.ru/health
https://dev-vk.neiirohub.ru/webhooks/vk
https://dev-app.neiirohub.ru
https://dev-app.neiirohub.ru/miniapp/balance
https://dev.neiirohub.ru/billing/webhooks/yookassa
```

The same smoke also verifies that public `/admin/*` and `/metrics` routes are
blocked.

The script preserves Docker volumes on stop. It does not touch production
Cloudflare, production VK settings, production YooKassa settings or production
data stores.

## Isolation Rules

- Do not run the production Cloudflare tunnel token on a development machine.
- Do not point DEV hostnames to the production VPS.
- Do not edit production VK Callback API settings for local tests.
- Do not reuse the production VK community token in local DEV.
- Do not reuse production YooKassa webhook settings for local DEV.
- Do not connect local DEV services to production Postgres, Redis or S3/MinIO.
- Do not commit local `dev.env`, tunnel tokens, VK tokens, YooKassa keys or
  provider keys.

## Security Rules

The DEV scripts enforce these rules before they start or inspect the stack:

```text
DEV secrets must stay local and untracked.
PROD secrets must not be used by local DEV scripts.
Production Cloudflare tunnel must not be started locally.
DEV VK token and callback settings must belong to the DEV VK community.
DEV YooKassa can be enabled only with a test key.
AI providers can run against DEV/test provider credentials only.
```

Real AI providers are allowed only when explicitly enabled:

```text
DEV_ALLOW_REAL_AI_PROVIDERS=true
```

When this flag is not enabled, `PROVIDER`, `PROVIDER_CHAIN`, `IMAGE_PROVIDER`
and `VIDEO_PROVIDER` must stay `mock`, route provider flags must stay disabled,
and provider API keys must stay empty. The committed DEV template enables this
flag so manual VK testing can mirror production behavior with DEV-only provider
keys.

Real YooKassa is blocked by default. To run a deliberate YooKassa test-store
smoke, set:

```text
DEV_ALLOW_REAL_PAYMENTS=true
PAYMENT_PROVIDER=yookassa
```

In that mode `YOOKASSA_SECRET_KEY` must be a YooKassa test key. Production
YooKassa credentials are not allowed in the DEV contour.

## Required DEV Inputs

The local `dev.env` for this contour must come from a DEV-specific template or be
filled manually with DEV-only values:

```powershell
Copy-Item .env.dev.example dev.env
notepad dev.env
```

```text
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa

VK_ACCESS_TOKEN=<dev VK community token>
VK_SECRET=<dev VK Callback API secret>
VK_CONFIRMATION_TOKEN=<dev VK confirmation string>
VK_GROUP_ID=<dev VK group id>

CLOUDFLARED_TUNNEL_TOKEN=<dev dashboard-managed tunnel token>
DEEPINFRA_API_KEY=<dev/test DeepInfra key>
APIMART_API_KEY=<dev/test APIMart key>
APIMART_BASE_URL=https://api.apimart.ai/v1
POYO_API_KEY=<dev/test PoYo key>
RUNWAYML_API_SECRET=<dev/test Runway key>
PAYMENT_PROVIDER=mock
PROVIDER_CHAIN=deepinfra,apimart,poyo,runway,mock
VK_DELIVERY_MODE=mock
DEV_ALLOW_REAL_AI_PROVIDERS=true
DEV_ALLOW_REAL_PAYMENTS=false
VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=true
```

`PUBLIC_VK_BASE_URL` is also used by VK Bot payment buttons. The bot sends a
server-owned `/payments/vk/{intent_id}` link and the API redirects active
`vk_bot` payment intents to the provider confirmation page. The redirect route
is rate-limited (`PAYMENT_REDIRECT_RATE_LIMIT_RPS` /
`PAYMENT_REDIRECT_RATE_LIMIT_BURST`), and production fails closed when VK bot
top-up is enabled without a valid HTTPS `PUBLIC_VK_BASE_URL`. Runtime VK bot
output also fails closed to a safe unavailable-payment notice instead of
falling back to direct provider checkout links when this base URL is missing or
invalid.

Rate limiting is split by layer. Nginx/Cloudflare own production-shaped
source-IP limits before proxying. Redis-backed services own durable user/action
limits such as VK antispam. The Go in-process limiter is a bounded local
fallback: buckets expire after idle time, total bucket count is capped per
process, and spoofable proxy headers are ignored unless a caller adds explicit
trusted-proxy normalization.

Mini App reference uploads are bounded at both the API and proxy layer. The API
uses `MEDIA_MAX_IMAGE_UPLOAD_BYTES` and `MEDIA_MAX_CONCURRENT_UPLOADS` to cap
per-process upload memory; with the committed DEV defaults this is
approximately `20 MiB * 4` file-buffer memory per API process, plus a small
multipart envelope. The production nginx route for `POST /miniapp/artifacts`
uses a matching `21m` body limit and disables proxy request buffering for that
exact upload endpoint.

Real provider mode is part of the DEV contour for manual VK testing, but it
must use DEV/test credentials only. Payments stay mocked unless
`DEV_ALLOW_REAL_PAYMENTS=true` is deliberately set with YooKassa test-store
credentials.

`VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=true` is a development-only UI parity
switch. It lets the VK bot show the same video model route buttons as the
production-shaped menu. Provider route execution still requires the matching
route flags, provider config and worker path. This flag is ignored in
production.

## Cloudflare

Use a separate dashboard-managed tunnel for DEV, for example:

```text
neiirohub-vk-dev
```

Published application routes:

| Hostname | Service |
|---|---|
| `dev-vk.neiirohub.ru` | `http://127.0.0.1:8088` |
| `dev-app.neiirohub.ru` | `http://127.0.0.1:8088` |
| `dev.neiirohub.ru` | `http://127.0.0.1:8088` |

Do not add broad public routes for `/admin/*`, `/metrics`, `/debug/*` or broad
`/billing/*`. Only the exact YooKassa webhook path is public.

## Readiness Criteria

The DEV contour is ready when all checks pass:

```text
https://dev-vk.neiirohub.ru/health -> 200
https://dev-vk.neiirohub.ru/webhooks/vk -> controlled non-5xx for invalid GET
https://dev-app.neiirohub.ru -> Mini App frontend
https://dev-app.neiirohub.ru/miniapp/* -> API/BFF responses
https://dev.neiirohub.ru/billing/webhooks/yookassa -> controlled non-5xx for invalid body
.\scripts\dev\smoke-dev.ps1 -> public DEV smoke passes
```

Production must remain reachable and unchanged while DEV is started/stopped.
