# Mini App Redesign Context

Purpose: keep UI redesign work from accidentally breaking backend-owned logic,
auth, billing, durable chat context, or dev tunnel setup.

## Current runtime shape

- VK text bot and VK Mini App are separate app surfaces over the shared backend
  core.
- Mini App frontend lives in `web/miniapp`.
- Mini App BFF lives under `/miniapp/*` and is mounted by the Go API.
- Provider calls must happen only from the worker flow. The Mini App frontend,
  Vite dev server, BFF handlers, and app-surface modules must not call AI
  providers directly.
- Public model naming must stay product-safe: user-facing UI may use the
  NeuroHub brand, but raw DeepInfra/DeepSeek model ids must not be shown. When
  a model alias is needed, use `ChatGPT`.

## Redesign safe boundaries

Redesign may change:

- Layout, spacing, typography, color tokens, icons, tab surfaces and component
  composition in `web/miniapp/src/**`.
- Visual representation of chat bubbles, create tab, settings tab, history
  lists, balances and status timelines.
- Vite dev proxy rules that only route local development traffic to the local
  Go API.

Redesign must not change:

- BFF endpoint paths or request/response DTO semantics.
- VK launch-param auth behavior.
- Job status meanings or terminal-state handling.
- Billing, balance, price or estimate ownership.
- Artifact access rules.
- Worker/provider execution path.
- Durable chat conversation identity semantics.

## Required Mini App API contracts

The frontend calls relative paths so dev/prod can keep same-origin routing:

- `GET /miniapp/balance`
- `POST /miniapp/estimate`
- `POST /miniapp/jobs`
- `GET /miniapp/jobs`
- `GET /miniapp/jobs/{id}`
- `GET /miniapp/artifacts/{id}`
- `POST /miniapp/chat/messages`
- `GET /miniapp/chat/conversations`
- `GET /miniapp/chat/conversations/{id}/messages`

Every request from `web/miniapp/src/api/client.ts` must send:

- `X-Launch-Params` from the current VK launch URL, or dev launch params in
  local development.
- `X-Idempotency-Key` on job/chat creation calls.

Frontend must not send:

- price, cost, balance, provider name, raw model/provider internals;
- user identity as source of truth;
- moderation state or trusted job status;
- artifact URLs from user input or local storage.

## Chat context rules

- Mini App chat uses durable backend conversation history.
- Frontend sends only an opaque per-user `conversation_id` to
  `POST /miniapp/chat/messages`.
- Empty `conversation_id` maps to the backend-compatible `default` thread.
- Worker/dialogcontext owns prompt-context assembly and assistant-message
  persistence.
- BFF must not prefix prompts with local process memory.
- VK text bot and Mini App threads must not mix:
  - VK bot fallback uses legacy `user_id + vk_peer_id`.
  - Mini App uses `conversation_source=miniapp` plus `external_thread_id`.

## localStorage rules

Allowed localStorage keys are UI preferences only:

- `vk_miniapp_active_tab_v1`
- `vk_miniapp_active_thread_v1`
- `vk_miniapp_theme_v1`

Do not persist:

- prompt text;
- assistant answers;
- launch params or signatures;
- tokens/secrets;
- balance;
- artifact URLs;
- provider/model internals;
- job payload bodies.

## Rendering and security

- AI-generated text must render as React text, never `innerHTML` or
  `dangerouslySetInnerHTML`.
- Raw backend errors and stack traces must not be shown to users.
- Artifact previews must use backend-owned `/miniapp/artifacts/{id}` URLs
  derived from trusted job DTO artifact ids.
- Private artifact URLs must not be stored in localStorage.

## Polling and job state

- Backend job state is the source of truth.
- The UI may poll, but must clean up timers on unmount/terminal state.
- Switching tabs or redesigning tab containers must not lose an in-flight job or
  create duplicate pollers.
- Result UI should be shown only after backend terminal success.

## Billing and estimate

- Backend owns all balance, price and billing decisions.
- `POST /miniapp/estimate` is read-only: it must not create jobs, reserve
  credits, call providers or write ledger entries.
- Create/submit UI must use backend estimate and balance responses; it must not
  calculate trusted prices client-side.

## Dev tunnel notes

For local VK testing, the Vite dev server can be the public entrypoint:

- `/` serves the Mini App frontend.
- `/miniapp/*` proxies to the local Go API.
- `/webhooks/*` proxies to the local Go API so one temporary HTTPS URL can be
  used for both Mini App and VK bot callback during local smoke.

Cloudflare helper:

- `scripts/dev/setup-miniapp-cloudflare-route.ps1` adds
  `app.neiirohub.ru -> http://localhost:5173` to the local Cloudflare tunnel
  config after the tunnel credentials exist.

Default dev tunnel for VK Mini App:

- Use `localhost.run` (`https://<random>.lhr.life`) via
  `start-miniapp-ngrok.ps1` (repo root) or manual SSH:
  `ssh -R 80:127.0.0.1:5173 nokey@localhost.run`.
- Do **not** use free ngrok for VK iframe dev — its warning page (`error.js`)
  blocks WebView.
- Stable option: Cloudflare named route `app.neiirohub.ru -> localhost:5173`
  (`scripts/dev/setup-miniapp-cloudflare-route.ps1`).

## Pre-redesign checklist

Before committing a redesign:

1. Verify `npm --prefix web/miniapp run build`.
2. Smoke the Mini App open path with a valid VK launch URL.
3. Smoke chat send, reload, and backend history restore.
4. Confirm localStorage contains only UI preference keys.
5. Confirm no raw provider/model ids are visible to users.
6. Confirm `/miniapp/balance` and `/miniapp/estimate` still come from backend.
7. Confirm no prompt/launch params/secrets appear in logs.
