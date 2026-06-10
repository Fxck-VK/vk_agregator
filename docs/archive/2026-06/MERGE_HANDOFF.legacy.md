# Merge Handoff - fastlife_dev

Last updated: 2026-06-06

## Current branch state

- Active branch: `fastlife_dev`
- Current pushed head: `3b26d43` (`miniapp: disable create post flow`)
- Remote: `origin/fastlife_dev` was pushed at `3b26d43`
- Historical context: `fastlife_dev` was created from the integrated `main`
  after the Mini App and VK backend integration. Earlier branch
  `feature/vk-miniapp` must not be used for new work.
- Important local state: `start-miniapp-ngrok.ps1` has an uncommitted local
  experiment that switches the tunnel from ngrok to `localhost.run`. Treat it
  as a merge decision item, not as committed product state.

## What is already in fastlife_dev

- PR-6: Mini App sends optional `model_id` to `POST /miniapp/jobs`; backend
  validates supported models. Unsupported model errors are shown safely.
- PR-7: `POST /miniapp/estimate` exists. Backend remains the price source of
  truth, estimate does not create jobs, reserve credits or mutate billing.
- PR-8: Result/artifact UX added with safe text rendering and backend artifact
  routes.
- PR-9: Mini App reload recovery and local retention added. Local storage is
  limited to UI-safe metadata and preferences, not prompts, launch params,
  tokens, balances or artifact URLs.
- PR-11: VKUI research ADR accepted Outcome C: hybrid usage.
- PR-14: VKUI hybrid base primitives added. VKUI providers wrap the app; base
  controls use VKUI while signature UX remains custom.
- PR-15: Chat mode aligned with VK text bot behavior and DeepSeek provider
  flow. Public model label is user-facing, provider details stay hidden.
- PR-16.1: Three-tab shell added: Create, Chat, Settings. Chat is default.
- PR-16.2: Chat threads added on frontend with local metadata only. Backend
  process-local context remains a known limitation.
- PR-16.3 and PR-16.3.1: Create tab was revised into a choice screen. Current
  committed product state removes the Create Post path and keeps only Create
  Photo / Create Video.
- PR-16.4: Settings tab added with theme preference, backend balance display,
  payment-history placeholder, local-history privacy controls and summary
  generation history.
- Latest backend safety fix: Mini App launch-param auth allows small future
  timestamp skew and still rejects large future timestamps.

## Current known runtime breakage

- VK Mini App loading over the ngrok domain was unreliable. The observed ngrok
  endpoint returned an ngrok warning/interstitial (`ERR_NGROK_6024`) instead of
  the Vite app, which can make VK WebView load forever.
- Local Vite/API were healthy during the last smoke: Vite served HTML, API
  `/health` returned 200, and authenticated Mini App balance/estimate requests
  worked when valid `X-Launch-Params` were supplied.
- A `localhost.run` tunnel worked during troubleshooting, but it is only a dev
  workaround. Prefer the owned domain path in `docs/DOMAIN_DEPLOYMENT_PLAN.md`.
- If the Mini App is opened outside VK without valid launch params, auth-bound
  calls are expected to fail. Do not weaken VK launch-param verification to make
  plain-browser testing pass.
- Payment history and top-up are UI placeholders/dependencies. There is no
  committed backend payment-intent or ledger-history endpoint yet.
- Backend chat thread listing/history endpoint is missing. Frontend stores only
  thread metadata locally and degrades if backend process-local context is lost.

## Merge hotspots

- `web/miniapp/src/chat/ChatScreen.tsx`
- `web/miniapp/src/chat/MessageBubble.tsx`
- `web/miniapp/src/chat/Composer.tsx`
- `web/miniapp/src/workflow/WorkflowMode.tsx`
- `web/miniapp/src/settings/SettingsScreen.tsx`
- `web/miniapp/src/components/ResultCard.tsx`
- `web/miniapp/src/api/client.ts`
- `web/miniapp/src/ui/theme.css`
- `internal/adapter/inbound/miniapp/handler.go`
- `internal/adapter/inbound/miniapp/handler_test.go`
- `internal/adapter/inbound/miniapp/sign.go`
- `DECISIONS.md`, `PROGRESS.md`, `TASKS.md`, `AUDIT.md`
- `start-miniapp-ngrok.ps1`

## Architecture invariants for the merge

- Mini App never calls AI providers directly.
- VK inbound handlers never call AI providers directly.
- Provider calls stay under `internal/adapter/provider` and worker paths.
- Billing remains append-only ledger based; frontend never mutates balance.
- Client-side estimate display is not a source of truth.
- Auth through VK launch params must stay backend-verified.
- No raw launch params, tokens, prompts, PII, provider keys or private artifact
  URLs in logs or local storage.
- AI output must be rendered as React text or through a safe renderer, never
  raw `innerHTML` / `dangerouslySetInnerHTML`.

## Immediate recommendation

Before merging more branches, decide the fate of the local
`start-miniapp-ngrok.ps1` tunnel experiment:

1. Keep it as a dev-only `localhost.run` fallback and commit it in a focused
   operational commit.
2. Revert it and keep the old ngrok script.
3. Replace both with a production-domain runbook based on `neiirohub.ru`.

For product testing in VK, prefer moving to the owned domain instead of chasing
temporary tunnel URLs.
