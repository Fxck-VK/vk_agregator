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
| APIMart | Gemini Omni 1.1 Flash / Flash EXT, Seedance 2.5, Hailuo 2.3 Fast / Hailuo 2.3 Standard | `APIMART_PROVIDER_ENABLED`, `APIMART_API_KEY`, `APIMART_BASE_URL` |
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
| `video_gemini_omni_1_1_flash` | APIMart | `gemini-omni-1.1-flash` | text, optional 1–10 images | automatic 3–10s | 360p, 720p, 1080p, 4k | Native audio; 16:9 or 9:16. Fixed resolution tariff. |
| `video_gemini_omni_1_1_flash_ext` | APIMart | `gemini-omni-1.1-flash-ext` | text, optional 1 or 3 images | 4s, 6s, 8s, 10s | 360p, 720p, 1080p, 4k | Native audio; 16:9 or 9:16. Two images are invalid. |
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
FEATURE_APIMART_OMNI_1_1_FLASH_ENABLED=false
FEATURE_APIMART_OMNI_1_1_FLASH_EXT_ENABLED=false
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

## Gemini Omni 1.1 Flash and Flash EXT

Sources checked 2026-09-12: [Flash generation and examples](https://docs.apimart.ai/ru/api-reference/videos/gemini-omni-1.1-flash/generation),
[EXT generation and examples](https://docs.apimart.ai/ru/api-reference/videos/omni-flash-ext/generation),
[APIMart prices](https://apimart.ai/ru/pricing).

Both models submit `POST /v1/videos/generations` and poll `GET /v1/tasks/{id}`.
Submission reads `data[0].task_id`; completion reads `data.result.videos[].url[]`.
The worker stores and moderates the MP4 before delivery and ledger capture.
Only the worker prepares owned reference images and uploads them for APIMart.
Input video, task extension, frame-role controls and last-frame controls are not
exposed in this release.

Flash sends `model`, `prompt`, `resolution`, `aspect_ratio`, optional `image_urls`.
It never sends `duration`: the internal 10-second dimension selects the fixed
tariff, while the UI explains the automatic 3–10s result. Output validation
accepts that duration range and portrait/landscape 4K. EXT additionally sends
`duration`, `nsfw_check: true`, and `generation_type: frame` for one image or
`reference` for three. Request validation rejects two EXT images before reserve
or submission. Both retain local input/output moderation.

Each model uses the durable `provider_tasks` claim before the paid call. A
worker restart with an unrecorded outcome waits for the sender's bounded timeout,
then fails with `submit_indeterminate` and releases credits; it does not submit
another paid request. Known accepted tasks resume polling from their stored id.

One APIMart credit is $0.10; one internal credit is $0.005. Retail is provider
cost ×3 rounded up to five internal credits. Static catalog version 12 adds
20 exact keys. Flash uses APIMart's 10-second estimate because its duration and
token-settled upstream charge are variable; this is a fixed retail price, not
reconciliation against actual provider cost. Shorter videos retain the quoted
price. Recheck provider rates before activation.

| Flash resolution | Provider estimate, 10s (USD) | Internal credits |
| --- | ---: | ---: |
| 360p | 0.296 | 180 |
| 720p | 0.88 | 530 |
| 1080p | 1.32 | 795 |
| 4k | 2.64 | 1585 |

| EXT resolution | 4s | 6s | 8s | 10s |
| --- | ---: | ---: | ---: | ---: |
| 360p, USD | 0.15 | 0.175 | 0.20 | 0.225 |
| 360p, internal credits | 90 | 105 | 120 | 135 |
| 720p/1080p, USD | 0.25 | 0.30 | 0.35 | 0.40 |
| 720p/1080p, internal credits | 150 | 180 | 210 | 240 |
| 4k, USD | 0.75 | 0.80 | 0.85 | 0.90 |
| 4k, internal credits | 450 | 480 | 510 | 540 |

See [DEV activation](runbooks/DEV.md#gemini-omni-video-configuration). Paid live
availability/playback has not been tested by the local integration checks.

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

## Kling V3, Motion Control 2.6 and Veo 3.1 (2026-09-12)

Contracts: [Kling V3](https://docs.apimart.ai/ru/api-reference/videos/kling-v3/generation),
[Motion Control](https://docs.apimart.ai/ru/api-reference/videos/kling-v2-6/kling-v2-6-motion-control-generation),
[Veo 3.1](https://docs.apimart.ai/ru/api-reference/videos/veo3/generation).
All submit to `POST /v1/videos/generations` and poll `GET /v1/tasks/{id}`.
Every route uses a persisted submit claim before paid network traffic; ambiguous
acceptance never causes automatic resubmission. Outputs retain moderation,
private artifact storage and ledger capture/release boundaries.

- `video_kling_v3` → `kling-v3`: 3–15 s, 720p/1080p/4K, 16:9/9:16/1:1,
  zero to two ordered frames. Public `video_audio` selects a separately priced
  variant; upstream `audio` defaults false. Resolution maps to `mode=std/pro/4k`.
- `video_kling_2_6_motion_control` → `kling-v2-6-motion-control`: one image and
  one owned video artifact. Mini App uploads MP4/MOV, at most 100 MiB, through
  authenticated, concurrency-limited `POST /miniapp/video-artifacts`. API probes
  the bytes using ffprobe (pipe-only protocols), validates 3–30 s, H264/HEVC,
  dimensions and bitrate, then persists private input metadata. Orientation
  `image` caps length at 10 s; `video` allows 30 s. Estimates and submissions
  derive billable ceil-seconds from this metadata and reject client mismatches.
  `std/pro` are quality modes, not promised output pixel sizes. Upstream uses
  `image_url`, `video_url`, `character_orientation`, `keep_original_sound=yes/no`;
  duration, resolution and aspect ratio are never sent. VK chat has no video
  upload flow and does not offer this route.
- `video_veo_3_1_lite`, `video_veo_3_1_fast`, `video_veo_3_1_quality` use exact
  `veo3.1-lite/fast/quality` model IDs: 8 s, 720p/1080p/4K, 16:9/9:16.
  Lite is text-only; Fast supports up to three images, Quality up to two.
  One/two frames select `generation_type=frame`, three images `reference`.
  Fast/Quality explicitly disable `official_fallback`; Lite omits that field.
  NSFW checking is enabled for all five models.

Provider prices checked on APIMart's [pricing page](https://apimart.ai/ru/pricing):
Kling per-second APIMart credits: 720p 0.672 / 1.008 with audio;
1080p 0.896 / 1.344 with audio; 4K 4.2856 with or without audio.
Motion std/pro: 0.5712 / 0.9144 per second.
Veo Lite/Fast/Quality: 0.7 / 1.4 / 10 per 8-second call at 720p or 1080p;
4K: 5.7 / 6.4 / 15. One APIMart credit is $0.10; one internal credit is
$0.005. Static catalog version 13 adds 143 variants at cost ×3, rounded up to
five internal credits. Billing uses the exact fixed-point pricing snapshot;
whole provider-credit ceilings are only worker cost budgets.

Motion input fetches use `/provider-references/{job}/{artifact}.mp4` on the
public Mini App host, with an expiring HMAC-SHA256 signature. The API checks
signature, expiry, active job state, owner and input binding before serving
GET/HEAD/Range. URLs are worker-created, ephemeral and never persisted or
returned by the BFF. Disable the Motion flag to hide new requests; retain the
signer while existing tasks finish. Delay key rotation until those tasks finish;
early rotation invalidates outstanding URLs.
