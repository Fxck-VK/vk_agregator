# VK AI Aggregator — Mini App Frontend

React SPA (VKUI + VK Bridge) that provides a mobile-first interface to the
VK AI Aggregator BFF endpoints (`/miniapp/*`).

## Stack

| Package | Role |
|---|---|
| `@vkontakte/vkui` | VK design-system components |
| `@vkontakte/vk-bridge` | VK platform integration (init, close, etc.) |
| `react` + `react-dom` 18 | UI framework |
| `vite` | Dev server + production build |
| `typescript` | Type safety |

## Prerequisites

- Node.js ≥ 18
- The Go API server running on `http://localhost:8080` (or set `VITE_API_URL`)

## Install

```bash
cd web/miniapp
npm install
```

## Dev mode (local, without real VK)

```powershell
# 1. From the repository root, start the Go API in another terminal.
#    Local .env / _env files are loaded automatically when present.
go run ./cmd/api

# 2. Start the frontend dev server:
cd web\miniapp
npm run dev
# → http://localhost:5173
```

The Vite dev server proxies `/miniapp/*` requests to `http://localhost:8080`.

For authentication in dev mode the BFF accepts `X-Launch-Params: vk_user_id=<id>`
when `VK_APP_SECRET` is not set. The app reads launch params from the URL query
string, so test with:

```
http://localhost:5173/?vk_user_id=777
```

### Open inside VK via HTTPS tunnel (`localhost.run` / `*.lhr.life`)

VK WebView needs a public HTTPS URL. For local dev use **`localhost.run`**
(`https://<random>.lhr.life`) — it works inside VK without the ngrok warning
page that blocks the iframe.

**One command (Windows, recommended):**

```powershell
# From repo root. Starts Docker deps, API + worker + Vite and prints the public URL.
.\scripts\dev\start-miniapp.ps1 -NoWait
.\scripts\dev\status-miniapp.ps1
.\scripts\dev\stop-miniapp.ps1
```

Backward-compatible wrapper: `.\start-miniapp-ngrok.ps1 -NoWait` / `-StopOnly`.

Tunnel is **`localhost.run` via SSH** (`https://*.lhr.life`), not ngrok.
Requirements: `go`, `npm`, `ssh` (OpenSSH client), `.env` with `DEEPINFRA_API_KEY`
and database/redis settings. Runtime logs: `.runtime/vk-miniapp/`.

Paste the printed `https://....lhr.life` URL into **dev.vk.com → your app →
Версия для vk.com → "URL для разработки"**, save, then open the app from VK.

**Manual stack (if you prefer separate terminals):**

```powershell
# 1. Infrastructure + API + worker (see repo RUNBOOK.md)
docker compose up -d
go run ./cmd/migrate up
go run ./cmd/api
go run ./cmd/worker

# 2. Vite
cd web\miniapp
npm run dev

# 3. Tunnel (prints https://....lhr.life)
ssh -o StrictHostKeyChecking=no -R 80:127.0.0.1:5173 nokey@localhost.run
```

**Verify in VK DevTools → Network:** you should see `/src/main.tsx` and
`/miniapp/balance`. If you see `error.js` / `allerrors.js` instead, VK loaded
an ngrok warning page — switch to `*.lhr.life`.

**Do not use free ngrok for VK Mini App dev** — the interstitial cannot be
passed in VK WebView.

**Stable alternative:** Cloudflare named tunnel `app.neiirohub.ru → localhost:5173`
via `scripts/dev/setup-miniapp-cloudflare-route.ps1` (see `RUNBOOK.md`).

### Using vk-bridge-mock (optional)

To simulate VK Bridge methods without opening the app inside VK:

```bash
npm install --save-dev @vkontakte/vk-bridge-mock
```

Then in `src/main.tsx` add before the `bridge.send('VKWebAppInit')` call:

```typescript
import { initializeMockBridge } from '@vkontakte/vk-bridge-mock';
if (import.meta.env.DEV) initializeMockBridge();
```

## Production build

```bash
cd web/miniapp
npm run build
# Output: web/miniapp/dist/
```

Serve the `dist/` folder from any static host or CDN registered in the VK Mini
App admin panel as the "Application URL".

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `''` (same origin) | BFF base URL for production deploys |

## Required backend env vars

| Variable | Description |
|---|---|
| `VK_APP_SECRET` | VK App protected key for sign verification. Set = real check; empty = dev mode (no check); empty in production = fail-closed startup |
| `VK_APP_ID` | VK Application ID (informational for the BFF) |
| `MINIAPP_LAUNCH_PARAMS_MAX_AGE` | Max age of launch params (default `1h`) |

## Authentication flow

1. VK opens the mini app and appends launch params to the URL query string:
   `?vk_user_id=...&vk_app_id=...&vk_ts=...&sign=...`
2. The SPA captures `window.location.search.slice(1)` on init and stores it.
3. Every BFF request includes the raw string in the `X-Launch-Params` header.
4. The BFF verifies the HMAC-SHA256 signature with `VK_APP_SECRET` before
   processing any request. Invalid/expired params → `401`.

## Screens

| Screen | Purpose |
|---|---|
| **Чат** | Durable Mini App conversations backed by `/miniapp/conversations*` and normal Job creation |
| **Создать** | Photo/video entry surface that creates backend Jobs and never calls providers directly |
| **Настройки / Профиль** | Balance, theme, payment products, payment intent creation and safe payment history from `/miniapp/payments` |
