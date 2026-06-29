# Fastlife Post-Merge Context

This file summarizes the work done on `fastlife_dev` after the shared merge
point with the integration/backend work. It is intentionally secret-free.

Context starts after:

- `3c33f0c9 Merge PR #9 from serega`

Current relevant commits on `fastlife_dev`:

- `9770c3b2 dev: add local dev contour startup`
- `78a9bf57 feat: stabilize video provider routing`
- `c9f0d448 feat: add routed media models`
- `91afdfff fix: tighten vk image model flow`
- `bc76a68d fix: normalize image provider catalog and errors`
- `3f5c8555 fix: recover async image provider tasks`
- `4059bacc feat: unify product catalog routing`

## Non-Negotiable Rules

- No real secrets, tokens, auth headers, full launch params, raw prompts, raw
  provider payloads, private artifact URLs or private provider URLs in commits,
  logs, docs, public DTOs or admin UI.
- Mini App, VK Bot and API never call providers directly. Provider calls belong
  to workers through provider adapters.
- Frontend and VK payloads may send only public aliases/options:
  `model_id`, `image_quality`, `video_route_alias`, `duration_sec`.
- Backend is the source of truth for model visibility, pricing, provider route,
  validation, reservation, capture and release.
- Unknown/disabled aliases fail closed server-side even if a button is hidden in
  UI.
- Billing remains ledger/reservation based: reserve before provider submit,
  capture only after delivery/storage success, release on technical failure.

## Local DEV Contour

The local DEV contour was added so live smoke tests can run without touching the
test VPS.

Important files:

- `scripts/dev/start-dev-stack.ps1`
- `scripts/dev/stop-dev-stack.ps1`
- `scripts/dev/smoke-dev.ps1`
- `docs/DEV_CONTOUR.md`
- `.env.dev.example`

Notes:

- Real `dev.env` / `.env` values are local-only and must not be committed.
- Cloudflare tunnel token is expected from local env, not from repo.
- Public DEV routes are meant for local smoke through Cloudflare, for example
  DEV VK callback and DEV Mini App frontend/BFF routes.
- Old Mini App model endpoints were removed:
  - `GET /miniapp/image-models`
  - `GET /miniapp/video-routes`
- Current public Mini App catalog endpoint:
  - `GET /miniapp/model-catalog`

## Provider And Media Routing

Video routes were moved to backend-owned product routes and provider adapters.

Public video route aliases:

- `video_hailuo_2_3_fast`
- `video_hailuo_2_3_standard`
- `video_kling_o3_standard`
- `video_seedance_2_0_fast`
- `video_runway_gen4_turbo`
- `video_runway_gen4_5`

Provider mapping is server-side only:

- APIMart: Hailuo 2.3 Fast / Standard
- PoYo: Kling O3 Standard, Seedance 2.0 Fast, Runway Gen-4.5
- Official Runway: Gen-4 Turbo

Image models integrated into the server-owned catalog:

- PoYo Nano Banana 2
- APIMart Nano Banana Pro
- APIMart GPT Image 2
- DeepInfra ByteDance Seedream 4.5
- DeepInfra Stability AI SDXL Turbo

DeepInfra image models are a documented legacy exception: they use
provider-level readiness (`DEEPINFRA_API_KEY` + `DEEPINFRA_BASE_URL`) and do not
yet have per-model feature flags. Tests cover fail-closed behavior without
DeepInfra readiness.

Important files:

- `internal/service/modelcatalog/catalog.go`
- `internal/service/videorouter/catalog.go`
- `internal/service/productcatalog/`
- `internal/adapter/provider/apimart/`
- `internal/adapter/provider/poyo/`
- `internal/adapter/provider/runway/`
- `internal/worker/generation.go`
- `internal/worker/poll.go`
- `internal/worker/worker.go`

## Product Catalog

`internal/service/productcatalog` is now the shared public catalog layer for Mini
App and VK Bot.

Key API:

- `productcatalog.FromConfig(cfg)`
- `productcatalog.RuntimeCatalog`
- `productcatalog.VideoRouteCatalogFromConfig(cfg)`

Mini App and VK Bot both consume this runtime catalog. They no longer build
provider readiness or video route catalogs separately.

Public catalog contains only:

- type
- public id/alias
- public name/description
- estimate credits
- enabled state
- quality options
- duration/resolution/aspect/reference limits

It must not expose:

- provider
- provider_model_id
- model_code
- API keys/auth headers
- resolved snapshots
- raw provider payloads
- prompts
- private URLs
- client-selected price/cost

## Mini App Flow

Mini App create screen was moved to the single public catalog endpoint.

Current UX direction:

- segmented image/video selection;
- model dropdown backed by `/miniapp/model-catalog`;
- only enabled models/routes are shown;
- image shows relevant quality/reference settings;
- video shows relevant duration/aspect/reference/start-image settings.

Backend validates all public aliases/options again. Frontend does not choose
provider ids or prices.

Important files:

- `web/miniapp/src/api/client.ts`
- `web/miniapp/src/workflow/WorkflowMode.tsx`
- `web/miniapp/src/ui/theme.css`
- `web/miniapp/src/chat/ChatScreen.tsx`
- `internal/adapter/inbound/miniapp/dto.go`
- `internal/adapter/inbound/miniapp/handler.go`
- `internal/app/miniapp/module.go`

## VK Bot Flow

VK Bot image/video menus now use Product Catalog for primary buttons.

Primary generic payloads:

- `menu.image.select`
- `menu.image.quality.select`
- `menu.video.route.select`
- `menu.video.duration.select`

Legacy model-specific commands remain only for compatibility with stale
payloads, text-mode keyboards and persisted dialog state. Tests verify legacy
commands do not appear in primary generated catalog keyboards.

Image flow:

- choose model from enabled public image models;
- choose quality only when catalog exposes `quality_options`;
- prompt step stores server-owned provider/model snapshot privately in job
  params;
- response payloads do not expose provider internals.

Video flow:

- choose public route alias from enabled public routes;
- choose duration from catalog allowed durations;
- routes requiring a start image fail closed if no image is supplied;
- aspect ratio is derived backend-side from trusted reference artifact metadata.

Important files:

- `internal/adapter/inbound/vk/menu.go`
- `internal/adapter/inbound/vk/handler.go`
- `internal/app/vkbot/module.go`
- `internal/service/commandrouter/router.go`
- `internal/domain/command.go`

## Payments And Balance

VK Bot top-up/account flow was tightened.

Changes:

- profile/account screens show balance;
- top-up catalog/pending payment screens show balance;
- payment message includes the payment confirmation URL as text fallback, not
  only as a VK open-link button;
- bot top-up return URL is server-owned and should target the VK dialog, for
  example `https://vk.com/write-<GROUP_ID>`;
- pending payment intents are reused only when their stored return URL matches
  the current bot return target.

Important files:

- `internal/adapter/inbound/vk/menu.go`
- `internal/adapter/inbound/vk/handler.go`
- `.env.dev.example`

## Async Provider Fixes

PoYo and async image provider jobs had two important recovery fixes.

PoYo submit/status parsing:

- accept task id from several provider response shapes;
- parse image output URLs from several result/output shapes;
- parse `progress` as int, decimal or string.

Worker recovery:

- async image and video provider tasks use provider polling after submit;
- early transient poll failures do not terminally fail an accepted provider task;
- terminal provider_task rows with already saved output artifacts can be replayed
  to finish job status/delivery.

Operational note:

- Existing already-failed jobs remain terminal unless manually reconciled.
- Existing orphaned provider tasks may need a safe `provider_poll` requeue after
  deploy, if the provider task and output artifact are already durable.

## Security Regression Coverage

Added/updated tests verify:

- public catalog does not expose provider internals or price/cost internals;
- Mini App rejects disabled image aliases and disabled video route aliases;
- Mini App ignores client-supplied provider/model_code/provider_model_id,
  resolved_snapshot and price/cost fields;
- commandrouter treats provider/model-code strings as free-form text;
- VK primary keyboards do not expose provider/model/auth/snapshot/private URL
  fields;
- old Mini App catalog endpoints return `404`.

## Checks Passed Before Latest Push

Final checks before `4059bacc`:

```powershell
go test ./internal/service/productcatalog ./internal/service/commandrouter ./internal/service/modelcatalog ./internal/service/videorouter ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk ./internal/adapter/provider/poyo ./internal/app/miniapp ./internal/app/vkbot ./internal/worker
go test ./...
go vet ./...
npm --prefix web/miniapp run typecheck
npm --prefix web/miniapp run lint
npm --prefix web/miniapp run test
npm --prefix web/miniapp run build
git diff --check
```

Staged secret scan was clean before commit.

Known harmless warning:

- `git diff --check` may print CRLF normalization warnings for already dirty
  Windows files, but it reported no whitespace errors.

## Next Agent Checklist

Before changing behavior:

1. Read this file, `AGENTS.md`, `.agents/state.json`, and relevant local
   `AGENTS.md`.
2. Keep `dev.env`, `.env`, real tokens and provider/private URLs out of git.
3. Prefer Product Catalog for any new user-facing model/menu work.
4. Do not reintroduce `/miniapp/image-models` or `/miniapp/video-routes`.
5. Do not add provider ids/model codes to public DTOs or VK payloads.
6. Run focused tests first, then `go test ./...`; run Mini App checks if
   frontend changed.
