# Video Generation

Operational reference for video generation across VK Bot, VK Mini App, API,
worker, billing and delivery.

## Current Runtime Model

Video generation is route-based.

Clients and surfaces use stable public route aliases. Provider model ids,
provider pricing and provider request shapes stay server-side.

Core flow:

1. VK Bot or Mini App selects a public video route alias.
2. `cmd/api` validates the request and creates a persisted Job.
3. Video route catalog resolves the alias into an immutable route snapshot.
4. Billing reserves credits before provider submission.
5. `cmd/worker` calls the provider through `internal/adapter/provider`.
6. Worker stores the result as an Artifact.
7. Delivery happens through delivery adapters, for example VK delivery.
8. Billing captures only after storage/delivery success. Provider or delivery
   failure must release the reservation.

Important boundaries:

- VK handlers, Mini App BFF and `cmd/api` must not call AI providers directly.
- Provider adapters must not know about VK delivery or billing.
- Provider ids and raw provider URLs must not be exposed to users.
- Provider response/error payloads must be normalized and redacted.
- Real provider routes must fail closed when config, pricing or secrets are
  missing.

## Providers

| Provider | Current video role | Required config |
| --- | --- | --- |
| APIMart | Seedance 2.5, Hailuo 2.3 Fast / Hailuo 2.3 Standard | `APIMART_PROVIDER_ENABLED`, `APIMART_API_KEY`, `APIMART_BASE_URL` |
| PoYo | Kling O3 Standard, Seedance 2.0 Fast, Runway Gen-4.5 | `POYO_PROVIDER_ENABLED`, `POYO_API_KEY`, `POYO_BASE_URL` |
| Runway | Runway Gen4 Turbo | `RUNWAY_PROVIDER_ENABLED`, `RUNWAYML_API_SECRET`, `RUNWAYML_BASE_URL` |
| DeepInfra | Text runtime only in the current architecture | no active video route |
| OpenAI | Optional safety moderation/scanner only, not generation | no active video generation route |
| Mock | Load-test route only | `APP_ENV=loadtest`, mock providers |

## Public Video Routes

Routes are defined in:

- `internal/domain/video_route.go`
- `internal/service/videorouter/catalog.go`
- `internal/service/productcatalog/builder.go`

| Public alias | Provider | Provider model id | Input shape | Duration | Resolution | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `video_seedance_2_5` | APIMart | `seedance-2.5` | text or up to 4 reference images | 5s, 10s, 15s, 30s | 480p, 720p, 1080p | MP4 with native audio. Human-face asset approval is not supported in this release. |
| `video_hailuo_2_3_fast` | APIMart | `MiniMax-Hailuo-2.3-Fast` | image/start image | 6s, 10s | 768p, 1080p | Requires start image. 1080p is limited to 6s. |
| `video_hailuo_2_3_standard` | APIMart | `MiniMax-Hailuo-2.3` | text or image | 6s, 10s | 768p, 1080p | Supports one reference image. 1080p is limited to 6s. |
| `video_kling_o3_standard` | PoYo | `kling-o3/standard` | text or image | 5s, 10s | 720p, 1080p | Supports 16:9, 9:16, 1:1 and one reference image. |
| `video_seedance_2_0_fast` | PoYo | `seedance-2-fast` | text, image or reference | 5s, 10s | 720p | Supports 16:9, 9:16, 1:1 and up to 4 reference images. |
| `video_runway_gen4_turbo` | Runway | `gen4_turbo` | image/start image | 5s, 10s | 720p | Official Runway route. Requires start image. Supports 16:9, 9:16, 4:3, 3:4, 1:1, 21:9. |
| `video_runway_gen4_5` | PoYo | `runway-gen-4.5` | text or optional image | 5s, 10s | 720p, 1080p | Distinct PoYo route. Text-only is valid; optional single reference image uses the provider list input and supports 16:9, 9:16, 4:3, 3:4, 1:1, 21:9. |
| `video_mock_text_to_video` | Mock | `mock-video` | text | 3s, 5s, 10s | 720p, 1080p | Load-test only. Must not be enabled in dev/staging/prod. |

## Feature Flags

The router must be enabled before any route can be exposed:

```env
FEATURE_VIDEO_ROUTER_ENABLED=true
```

Route flags:

```env
FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=false
FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=false
FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=false
FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=false
FEATURE_APIMART_SEEDANCE_2_5_ENABLED=false
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=false
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=false
FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=false
```

The production profile is stricter than the opt-in defaults above. Production
must keep both Hailuo routes disabled and expose Kling O3 Standard, Runway
Gen-4 Turbo, Seedance 2.0 Fast and Runway Gen-4.5. `check-prod-env.*` rejects a
production deploy when this profile or the required PoYo/Runway readiness is
missing.

Provider switches must match enabled routes. Example:

```env
APIMART_PROVIDER_ENABLED=true
APIMART_API_KEY=...
APIMART_BASE_URL=https://api.apimart.ai/v1

POYO_PROVIDER_ENABLED=true
POYO_API_KEY=...
POYO_BASE_URL=...

RUNWAY_PROVIDER_ENABLED=true
RUNWAYML_API_SECRET=...
RUNWAYML_BASE_URL=https://api.dev.runwayml.com/v1
```

`FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=true` is valid only for
`APP_ENV=loadtest` and mock providers.

## Seedance 2.5

[Generation contract](https://docs.apimart.ai/ru/api-reference/videos/seedance-2-5/generation)
and [public rates](https://apimart.ai/zh/model/doubao-seedance-2-5), checked 2026-09-09.
Submit uses `POST /v1/videos/generations`, model `seedance-2.5`, fixed duration,
resolution, size, optional `image_urls`, `generate_audio=true`, `output_format=mp4`,
`watermark=false`, `NSFWCheck=true`. Polling uses `GET /v1/tasks/{task_id}`.
Provider NSFW checking supplements mandatory application output moderation.

Public rollout supports text and up to four owned image artifacts. Video/audio
references, first/last-frame roles, asset library for human faces, edit/extend,
MOV and automatic duration are deferred. Mini App accepts `video_resolution`,
validates it against the route and uses it for both quote and Job snapshot.
Raw `resolution`, provider and billing fields remain forbidden. Duration controls
show every allowed duration. VK's shared catalog derives the new route automatically;
the standalone Web platform does not yet have a video-generation form.

Approved retail is provider preauthorization estimate x3, rounded up once to
five internal credits (one internal credit = $0.005). Static catalog version 7:

| Resolution | Provider estimate, USD/second | 5s credits | 10s credits | 15s credits | 30s credits |
| --- | --- | --- | --- | --- | --- |
| 480p | 0.09608 | 290 | 580 | 865 | 1730 |
| 720p | 0.216 | 650 | 1300 | 1945 | 3890 |
| 1080p | 0.38488 | 1155 | 2310 | 3465 | 6930 |

APIMart settles successful tasks by actual tokens. The second-based values are
estimates, not a guaranteed provider bill. User capture stays at the fixed reserved
snapshot with no retrospective surcharge. Route spend metadata keeps exact
millionths per second and rounds the final provider credit estimate upward;
its cap is an estimate guard, not an upstream billing cap.

The adapter coalesces concurrent same-key submits and remembers accepted or
ambiguous outcomes for its lifetime. Transport failures, HTTP 408/409/5xx and
unreadable/missing task identifiers stop automatic fresh submission using
`provider_submit_indeterminate`. Accepted tasks use existing durable worker polling.
Process crashes between acceptance and saving the task ID remain the shared
durable-intent gap; no upstream replay guarantee is assumed.
See [DEV enablement](runbooks/DEV.md#seedance-25-configuration).

## Deprecated / Legacy Video Notes

These are not active video generation paths:

- `PrunaAI/p-video`
- `DEEPINFRA_VIDEO_MODEL`
- `DeepInfra` video generation
- OpenAI video generation

Some legacy ids or command names may still exist for backward-compatible parsing
or disabled UI states. Do not re-enable them without a new route catalog entry,
pricing, provider smoke and delivery smoke.

## VK Bot And Mini App Behavior

VK Bot and Mini App should not hardcode provider model ids.

Expected behavior:

- Runtime catalog exposes only available public aliases.
- A route is hidden if router flag, route flag, provider switch, provider key,
  provider base URL or pricing is missing.
- User-facing labels may be localized, but provider ids and provider prices stay
  server-side.
- Request params may include `video_route_alias`; provider model id from client
  params is rejected.
- The resolved snapshot is stored in job params so later config changes cannot
  change already created jobs.

## Billing And Safety

Video jobs are expensive and must remain fail-closed.

Rules:

- Reserve credits before provider submission.
- Capture only after artifact storage and user-visible delivery succeed.
- Release reservation on provider failure, worker failure or delivery failure.
- User-facing price comes from pricing catalog, not from hidden provider cost
  fields.
- Provider technical failures must not charge the user.

## Storage And Delivery

Generated video output is stored as an Artifact.

Rules:

- Do not send raw provider URLs to users.
- Do not expose private storage URLs.
- VK delivery must go through `internal/adapter/delivery/vk`.
- Media probing/transcoding, when enabled, belongs to worker/services, not VK Bot
  or Mini App handlers.

## Smoke Checklist

Before enabling a real route:

1. Run mock/loadtest route first.
2. Verify route is hidden when provider config or pricing is missing.
3. Verify provider submit/poll success.
4. Verify provider failure releases billing reservation.
5. Verify artifact is stored and owner-checked.
6. Verify VK/Mini App delivery succeeds.
7. Verify user does not see provider id, raw provider URL or private artifact URL.
8. Verify metrics and logs do not contain secrets, prompt bodies or raw provider
   payloads.

## Related Docs

- `AGENTS.md`
- `.agents/state.json`
- `docs/LOAD_TESTING.md`
- `docs/DEV_CONTOUR.md`
- `docs/ARCHITECTURE.md`
