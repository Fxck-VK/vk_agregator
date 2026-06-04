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
# 1. Start the Go API in another terminal (mock mode):
. .\.env.ps1
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

### Open inside VK via VK Tunnel

```bash
# API + worker running (mock); Vite dev server up via `npm run dev`.
# Export VK_APP_ID (the project uses .env.ps1 on Windows), then:
npm run tunnel
```

`npm run tunnel` runs `@vkontakte/vk-tunnel` against the local dev server
(`localhost:5173`). It needs a **one-time VK OAuth**: open the printed
`oauth.vk.ru` link, authorize, press ENTER. The tunnel then prints an HTTPS URL.

Paste that URL into **dev.vk.com → your app → Settings → Mini Apps** as the app
(iframe) URL, then open the app from VK. The OAuth step is interactive and
cannot be scripted.

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
| `VK_APP_ID` | VK Application ID (used by `npm run tunnel`) |
| `MINIAPP_LAUNCH_PARAMS_MAX_AGE` | Max age of launch params (default `1h`) |

## Authentication flow

1. VK opens the mini app and appends launch params to the URL query string:
   `?vk_user_id=...&vk_app_id=...&vk_ts=...&sign=...`
2. The SPA captures `window.location.search.slice(1)` on init and stores it.
3. Every BFF request includes the raw string in the `X-Launch-Params` header.
4. The BFF verifies the HMAC-SHA256 signature with `VK_APP_SECRET` before
   processing any request. Invalid/expired params → `401`.

## Screens

| Screen | Route trigger |
|---|---|
| **Задачи** (job list) | Default tab |
| **Новая задача** | "Создать" button |
| **Детали задачи** | Tap on a job row; auto-refreshes until terminal |
| **Баланс** | Second tab |
