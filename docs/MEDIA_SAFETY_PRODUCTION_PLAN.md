# Media Safety Production Plan

This is the working plan for turning media safety into a production-grade,
million-user-ready product layer. It is intentionally focused on CPU efficiency,
strict user-upload boundaries, provider-output risk control, billing safety and
bounded observability.

Do not treat this file as permission to weaken existing product invariants.

## Recheck Verdict

The plan is directionally correct, but "million users" must be treated as a
capacity and abuse problem, not only as a file validation problem. The
implementation must include explicit capacity budgets, edge request limits,
memory-safe upload handling, privacy sanitization for reference images,
separate worker limits for each expensive stage, kill switches, staged rollout
and load/chaos validation.

## Current Baseline

- Mini App upload path accepts user reference images through
  `internal/adapter/inbound/miniapp/upload.go`.
- User uploads are already image-only at the API boundary:
  `image/jpeg`, `image/png`, and `image/webp` are recognized from bytes.
- User video upload is not part of the product and must stay blocked unless a
  future explicit product decision changes it.
- Current upload validation reads the uploaded image bytes into memory up to
  `20 MiB`; it does not yet enforce decoded dimensions or total pixel count.
- API-proxied uploads are acceptable for MVP and small images, but at million
  user scale they can become an API memory/network bottleneck unless limits and
  concurrency are tightly bounded.
- Worker video media pipeline is worker-owned and currently couples
  `MEDIA_PIPELINE_ENABLED=true` with both `ffprobe` and `ffmpeg`.
- Current video validation/transcode code lives under:
  - `internal/service/mediaprobe/`
  - `internal/service/mediatranscode/`
  - `internal/worker/`
- Current video delivery prefers a ready VK variant when present, but raw video
  fallback still needs a clear product policy for production mode.
- Observability already has bounded media metrics; keep labels bounded.

## Production Target

Default production behavior should be:

- User uploads: images only, strict cheap validation, no user videos.
- Provider video: constrained provider request first, cheap probe second.
- `ffmpeg`: off by default; only explicit fallback or explicit model policy.
- API: never performs expensive media processing synchronously.
- Worker: owns provider calls and any media processing.
- Billing: capture only after safe delivery/access path succeeds.
- Overload behavior: backpressure and safe rejection before expensive work.
- Rollout behavior: feature flags, kill switches and dark-launch metrics before
  broad production traffic.

Main flow:

```text
Mini App/VK Bot
  -> product-level job spec
  -> worker provider contract/cost guard
  -> constrained provider request
  -> provider output stored as private artifact
  -> cheap media validation
  -> deliver existing safe output or optional fallback transcode
  -> billing capture only after safe delivery
```

## Capacity Assumptions To Validate

Use these as initial planning assumptions, not permanent product truth:

- API instances must never buffer unbounded uploads.
- Effective API memory risk is roughly
  `concurrent_uploads_per_instance * MEDIA_UPLOAD_MAX_IMAGE_BYTES`.
- If uploads stay API-proxied, default image upload size should be conservative
  unless product quality requires more.
- If product needs larger reference images or high upload concurrency, move to a
  private staged direct-to-object-storage upload flow.
- Direct upload must use owner-bound one-time sessions, server-generated object
  keys, private storage, TTL cleanup and server-side finalize validation.
- Video transcode is the only intentionally CPU-expensive media step and must be
  disabled by default.
- `ffprobe` is cheaper than transcode, but still needs concurrency limits.
- Cleanup and scanning must never compete unboundedly with user-facing delivery.

## Stage 0 - Threat Model And Capacity Budget

Load impact: none. Planning/audit only.

Goal: define safe defaults before implementation so later stages do not optimize
for a fake scale target.

Prompt:

```text
Goal: create a concrete media threat model and capacity budget before media safety implementation.

Apply Common Rules from docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

What to do:
- Audit current Mini App upload path, VK Bot media input behavior, worker media pipeline, artifact storage and delivery.
- Document current max request body sizes at frontend, API handler, reverse proxy/tunnel, object storage and provider output download.
- Propose initial limits:
  - max image upload bytes;
  - max image width/height/pixels;
  - max files per job;
  - max upload requests per user/window;
  - max active media jobs per user;
  - max provider attempts/fallback attempts;
  - max probe/transcode concurrency per worker;
  - queue degradation thresholds.
- Identify abuse cases:
  - renamed non-image files;
  - image bombs;
  - EXIF/GPS/privacy metadata;
  - repeated duplicate uploads;
  - high-concurrency upload flood;
  - provider invalid output;
  - provider success but product failure;
  - delivery success ambiguity;
  - cleanup deleting active objects.
- Decide if MVP stays API-proxied uploads or needs direct-to-object-storage staged uploads later.
- Write decisions into this plan or a follow-up doc without secrets.

Checks:
- git diff --check
- git status --short --branch

Commit:
- Commit only if a doc decision is changed: docs: define media safety capacity budget
- Push only if explicitly allowed.
```

### Stage 0 Decisions - Threat Model And Capacity Budget

Decision date: 2026-06-12.

Scope: planning only. No runtime behavior changes are made in this stage.

#### Current Audited Behavior

- Mini App reference uploads enter through
  `internal/adapter/inbound/miniapp/upload.go`.
- Frontend upload guard exists in `web/miniapp/src/api/client.ts`:
  advisory max size is `20 MiB`, and advisory MIME list is JPEG, PNG and WebP.
- API upload guard is authoritative:
  - multipart body is wrapped with `http.MaxBytesReader`;
  - current file byte limit is `20 MiB` plus `1 MiB` multipart overage;
  - files are read into memory with a bounded reader;
  - content type is detected from bytes;
  - accepted MIME classes are JPEG, PNG and WebP;
  - decoded width, height and pixel count are not yet enforced;
  - EXIF/GPS/comment metadata is not yet stripped or blocked.
- Mini App reference selection is limited to 4 artifacts per job through both
  frontend constants and backend reference validation.
- VK Bot currently does not accept user video uploads and does not create media
  artifacts from arbitrary incoming media. Video jobs are created from text in
  the active video dialog mode. This must remain true for production.
- Worker reference forwarding re-checks artifact ownership, kind, media type and
  ready status before converting image artifacts to provider inputs.
- Provider output download currently has a single service-level remote artifact
  ceiling of `256 MiB`.
- Object storage does not enforce product-specific media limits by itself; the
  application must enforce limits before storage and before provider forwarding.
- Reverse proxy, Cloudflare tunnel and deployment edge body limits are not
  defined in this repository. Production rollout must set them explicitly and
  keep them aligned with backend upload limits.
- Video media processing is worker-owned. Current config couples
  `MEDIA_PIPELINE_ENABLED=true` with both ffprobe and ffmpeg availability.
- Delivery currently prefers ready VK variants when present, but raw provider
  video fallback needs a stricter production policy.

#### Capacity Budget Defaults

These are the initial production defaults for the implementation stages. They
are intentionally conservative and can be raised after load tests and real
product-quality review.

| Budget | Initial default | Reason |
| --- | ---: | --- |
| Max image upload bytes | `10 MiB` | Keeps API-proxied upload memory bounded while preserving normal reference image quality. |
| Max image width | `4096 px` | Blocks extreme dimensions before decode-heavy work. |
| Max image height | `4096 px` | Blocks extreme dimensions before decode-heavy work. |
| Max image pixels | `16,777,216` | Caps image bombs and memory amplification. |
| Allowed user image formats | JPEG, PNG; WebP behind config | JPEG/PNG are safest baseline; WebP remains product-gated. |
| Max files per job | `4` | Matches current Mini App behavior; do not raise until capacity is proven. |
| Max upload requests per user | `6/min`, burst `4` | Uploads are heavier than normal BFF writes and need a separate limiter. |
| Max active image jobs per user | `2` | Prevents one user from occupying provider/delivery capacity. |
| Max active video jobs per user | `1` | Video is expensive and has highest provider/CPU risk. |
| Max active media jobs per user | `3` | Shared ceiling across image and video jobs. |
| Provider attempts for image | `1 primary + 1 fallback` only when cost policy allows | Avoids unbounded paid retries. |
| Provider attempts for video | `1 paid attempt` by default | Video provider retries can burn money; fallback must be explicitly budgeted. |
| Probe concurrency per worker | `2` default, `4` hard initial ceiling | ffprobe is cheaper than transcode but still consumes CPU/IO. |
| Transcode concurrency per worker | `0` default, `1` fallback ceiling | ffmpeg must not be the normal path for scale. |
| Provider image output bytes | `25 MiB` | Separate from video; fail hard above limit. |
| Provider video output bytes | `128 MiB` initial, `256 MiB` absolute hard ceiling | Current code uses 256 MiB; production should split by modality. |
| Temp/staged upload TTL | `24h` | Prevents abandoned private objects from growing forever. |
| Failed media retention | `7d` | Enough for debugging without long-term storage growth. |

Memory budget formula for API-proxied uploads:

```text
api_upload_memory_risk =
  concurrent_uploads_per_api_instance * MEDIA_UPLOAD_MAX_IMAGE_BYTES
```

With a `10 MiB` upload limit, even `20` concurrent uploads can place roughly
`200 MiB` of upload body pressure on one API instance before application
overhead. This is why upload concurrency must be limited before buffering and
why larger files should move to staged object-storage uploads.

#### Queue Degradation Thresholds

Use these starting thresholds for backpressure. They should become config and
alerts, not hardcoded hidden behavior.

- Reject new expensive media jobs before reservation/provider submit when the
  oldest media job waiting for provider submit is older than `2m`.
- Degrade or return retry-later when pending media jobs exceed the measured
  per-worker processing budget by `5m` of estimated work.
- Stop scheduling optional fallback/transcode when any media queue is degraded.
- Alert when delivery backlog age exceeds `5m`.
- Alert when successful provider output does not reach safe delivery/access
  within `10m`.
- Alert when provider success but product failure ratio exceeds `3%` over
  `10m` for any bounded provider/model class.
- Cleanup must pause or reduce batch size when delivery or provider queues are
  degraded.

#### Threat Model And Required Controls

| Abuse case | Required control |
| --- | --- |
| Renamed non-image files | Ignore extension and frontend MIME; validate magic bytes and decoded image config on the backend. |
| Image bombs | Enforce bytes, width, height and total pixels before storage/provider forwarding. |
| EXIF/GPS/privacy metadata | Raw uploads stay private; provider input must use sanitized bytes or fail closed by policy. |
| Repeated duplicate uploads | Reuse same-user artifacts by content hash plus validation policy version; keep hash internal only. |
| High-concurrency upload flood | Separate upload limiter, per-user active media limits, edge/WAF limits and API upload concurrency guard. |
| Provider invalid output | Store privately, validate against provider contract/probe before delivery, release reservation on unsafe result. |
| Provider success but product failure | Track as provider/product waste; do not capture billing unless safe delivery/access succeeds. |
| Delivery success ambiguity | Persist delivery attempts and capture only after confirmed safe delivery/access path. |
| Cleanup deleting active objects | Cleanup must be batched, indexed and limited to inactive failed/deleted/expired classes only. |

#### API-Proxied Upload Decision

MVP stays API-proxied for reference images only because the product currently
allows small image references and the backend already owns auth, owner checks,
idempotency and private artifact storage.

Move to direct-to-object-storage staged uploads when any of these becomes true:

- reference image quality requires files larger than `10 MiB`;
- upload traffic makes API memory/network pressure visible;
- multi-instance API deployment needs upload capacity independent from BFF
  request capacity;
- mobile upload retry behavior needs resumable sessions.

Future staged upload design must use owner-bound one-time upload sessions,
server-generated private object keys, strict TTL cleanup, no public bucket
access and a server-side finalize step that performs the same validation before
the artifact can be used by a job.

#### Follow-Up Implementation Notes

- Split upload limits into config instead of hardcoded Mini App constants.
- Add backend `image.DecodeConfig` validation for dimensions and pixels.
- Decide whether WebP is a production default or config-gated format.
- Add reference image metadata sanitization before provider forwarding.
- Split provider output download limits by modality.
- Make remote provider downloads fail hard above limit; do not store truncated
  artifacts.
- Avoid base64 data URL decoding paths that can allocate large buffers before
  limit enforcement.
- Split video probe policy from transcode policy so ffmpeg is not required for
  the default production path.
- Add multi-instance-safe upload/job/backpressure guards. Process-local limits
  are acceptable only for local development.

## Common Rules For Every Stage

Use this block at the top of every implementation prompt:

```text
Режим: IMPLEMENT.

Перед началом:
- Сверься с AGENTS.md.
- Сверься с .agents/state.json.
- Если трогаешь директорию с локальным AGENTS.md, прочитай его.
- Если меняешь запуск/env/deploy, сверяйся с RUNBOOK.md.
- Если меняешь архитектурные границы, сверяйся с docs/ARCHITECTURE.md.
- Если ловишь нетривиальную повторяемую ошибку, запиши sanitized entry в .agents/logs/errors.jsonl строго по .agents/schemas/error.schema.json.

Безопасность:
- Не логируй и не коммить секреты, токены, auth headers, prompts, full VK launch params, raw PII, raw provider payloads, raw payment payloads, private artifact URLs, raw URLs или полные idempotency keys.
- VK Bot и Mini App не должны вызывать AI providers, ffprobe или ffmpeg напрямую.
- Provider/media processing только через worker/services/adapters.
- Billing только через ledger. Не mutate balance напрямую.
- Inbound events, job creation, provider submit/poll, delivery и payment flows должны оставаться idempotent.
- /metrics, Grafana, Prometheus, Loki, Tempo, Alertmanager, OTel и exporters не должны становиться публичными.

Качество:
- Делай минимальный scoped diff.
- Не трогай unrelated изменения.
- Не ломай local mock-backed dev runtime.
- После этапа покажи changed files, checks run/skipped, security/architecture impact и git status.

Commit/push:
- После крупного завершенного этапа проверь git status, diff, отсутствие секретов и unrelated изменений.
- Делай один логический commit только после зеленых релевантных проверок.
- Push делай только если явно разрешено в текущем этапе.
- Если проверки падают, не пушь; объясни что упало и следующий fix step.
```

## Stage 1 - Media Policy Config Split

Load impact: low. This is config and tests only.

Goal: split expensive video transcode from cheap validation and make the default
production policy CPU-safe.

Prompt:

```text
Цель этапа: разделить media pipeline policy так, чтобы ffmpeg не был default path, а ffprobe/cheap validation и transcode управлялись отдельно.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи текущие MEDIA_* config в internal/platform/config/config.go, cmd/worker/main.go, RUNBOOK.md и .env.example.
- Добавь отдельные policy/env поля:
  - MEDIA_VIDEO_PROBE_POLICY=disabled|trusted_provider|probe_required
  - MEDIA_VIDEO_TRANSCODE_POLICY=never|fallback|always
  - MEDIA_DELIVER_RAW_PROVIDER_VIDEO=never|if_probe_passed|always_dev_only
- Production default:
  - probe_required
  - transcode never или fallback, но не always
  - raw provider delivery only if probe passed and provider contract says output is delivery-ready
- Local dev default must still run without ffprobe/ffmpeg when media pipeline is disabled.
- Не требуй FFMPEG_PATH, если MEDIA_VIDEO_TRANSCODE_POLICY=never.
- Не требуй FFPROBE_PATH, если MEDIA_VIDEO_PROBE_POLICY=disabled или trusted_provider только для dev/mock.
- Обнови config validation tests.
- Обнови RUNBOOK и .env.example только для новых env-переменных.

Ограничения:
- Не меняй provider adapters в этом этапе.
- Не запускай ffmpeg/ffprobe.
- Не меняй billing/delivery behavior пока policy только конфигурируется.

Проверки:
- gofmt -l .
- go test ./internal/platform/config
- go vet ./internal/platform/config
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: media: split video processing policy
- Push только если явно разрешено.
```

## Stage 2 - User Upload Hardening

Load impact: low to medium. Validation is cheap; avoid full decode/re-encode.

Goal: make user reference uploads safe and scalable without CPU-heavy processing.

Additional production requirements:

- Edge/proxy request body limit must be aligned with the backend upload limit.
- Upload handling must reject before buffering many large bodies concurrently.
- Missing/suspicious image dimensions must fail closed.
- If product needs larger images or high upload concurrency, use private staged
  object uploads instead of unlimited API-proxied uploads.
- Frontend `accept` is advisory only; backend byte validation is authoritative.

Prompt:

```text
Цель этапа: сделать production-grade upload validation для пользовательских reference images без ffmpeg/ffprobe и без тяжелой синхронной обработки.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи internal/adapter/inbound/miniapp/upload.go, references.go, handler_test.go и web/miniapp/src/workflow/WorkflowMode.tsx.
- User uploads должны оставаться image-only.
- Запретить video/audio/document/archive/svg/gif/heic/tiff/bmp/pdf.
- Решить список allowed image formats для production:
  - минимум: image/jpeg, image/png
  - image/webp оставить только если это реально нужно продукту; иначе сделать config flag.
- Добавить централизованный upload policy/config:
  - MEDIA_UPLOAD_MAX_IMAGE_BYTES
  - MEDIA_UPLOAD_MAX_IMAGE_WIDTH
  - MEDIA_UPLOAD_MAX_IMAGE_HEIGHT
  - MEDIA_UPLOAD_MAX_IMAGE_PIXELS
  - MEDIA_UPLOAD_ALLOWED_IMAGE_MIME
  - MEDIA_UPLOAD_MAX_FILES_PER_JOB
- Проверять magic bytes/content type from bytes, not extension.
- Добавить image.DecodeConfig-based dimension/pixel validation without full decode.
- Защититься от image bombs через max pixels.
- Не читать файл сверх лимита; не логировать filename/raw mime/raw payload.
- Сохранить owner-checked artifact access.
- Mini App accept attribute должен соответствовать backend policy, но frontend остается advisory only.
- Добавить tests:
  - valid jpg/png
  - webp policy if enabled/disabled
  - fake extension ignored
  - wrong magic rejected
  - oversize bytes rejected
  - too many pixels rejected
  - huge dimensions rejected
  - video/mp4 renamed to png rejected
  - svg/gif/pdf/zip rejected
  - rate limit/idempotent replay still works

Ограничения:
- Не добавлять тяжелую image normalization/re-encode в API.
- Не принимать пользовательские видео.
- Не отправлять raw upload bytes в logs/errors/metrics.

Проверки:
- gofmt -l .
- go test ./internal/adapter/inbound/miniapp ./internal/platform/config
- go vet ./internal/adapter/inbound/miniapp ./internal/platform/config
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run build
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: miniapp: harden reference image uploads
- Push только если явно разрешено.
```

## Stage 2A - Reference Image Privacy Sanitization

Load impact: low to medium depending on implementation. Prefer bounded metadata
stripping; avoid mandatory full re-encode in the API request path.

Goal: prevent user reference images from leaking EXIF/GPS/comments or private
metadata to providers or other users.

Prompt:

```text
Goal: add privacy sanitization policy for reference images before provider use.

Apply Common Rules from docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

What to do:
- Audit how input reference artifacts are stored, resolved and converted to provider InputURLs.
- Add policy:
  - raw uploaded bytes stay private;
  - provider input should use sanitized reference bytes where feasible;
  - public DTOs never expose metadata, storage keys or raw URLs.
- For JPEG/PNG/WebP, decide safe metadata handling:
  - strip EXIF/GPS/comments where cheap and deterministic;
  - if safe stripping is not implemented for a format, reject metadata-bearing files or block provider-forwarding by policy.
- Do not do expensive re-encode in the API request path.
- If full normalization is needed, make it worker-owned, bounded and optional.
- Add tests:
  - JPEG with EXIF/GPS is not forwarded raw to provider;
  - PNG text chunks/comments do not leak where supported;
  - unsupported metadata policy fails closed or sanitizes;
  - sanitized artifact remains owner-checked;
  - no private storage key/hash appears in DTOs/logs.

Checks:
- gofmt -l .
- go test ./internal/adapter/inbound/miniapp ./internal/worker ./internal/service/artifactservice
- go vet ./internal/adapter/inbound/miniapp ./internal/worker ./internal/service/artifactservice
- git diff --check
- git status --short --branch

Commit:
- If checks are green, commit: media: sanitize reference image metadata
- Push only if explicitly allowed.
```

## Stage 3 - Provider Media Contract Registry

Load impact: low. This is preflight logic and config.

Goal: avoid sending requests to provider/model combinations that cannot produce
safe, cost-controlled outputs.

Additional production requirements:

- Contract must record whether provider submit supports idempotency keys and
  what guarantee the provider gives.
- Provider retries must not create duplicate paid tasks when provider-side
  idempotency exists.
- When provider-side idempotency does not exist, fallback/retry budget must be
  conservative and observable as business risk.
- Raw provider model ids may be stored in config, but metrics labels must use a
  curated bounded `model_class`.

Prompt:

```text
Цель этапа: добавить provider media contract/risk guard до provider call, чтобы не отправлять video jobs в заведомо рискованные provider/model.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи internal/domain/provider.go, internal/adapter/provider/**, internal/worker/worker.go, config и tests.
- Добавь provider/model media contract на product-level, не frontend-level:
  - provider
  - model
  - modality
  - allowed durations
  - allowed aspect ratios
  - allowed resolutions
  - expected container
  - expected codec
  - expected max bytes
  - delivery_ready_output bool
  - requires_probe bool
  - requires_transcode bool
  - transcode_allowed bool
  - max_provider_attempts
  - max_fallback_attempts
  - max_provider_cost_credits или internal cost units
- Worker до submit должен validate job spec against contract.
- Frontend/VK Bot не должны прокидывать provider-native unsafe params.
- Unsupported/risky request должен fail before provider call and before expensive work.
- Add tests:
  - valid video spec passes
  - unsupported duration rejected before provider
  - unsupported aspect rejected before provider
  - unsupported model rejected before provider
  - contract requiring transcode is rejected if transcode policy never
  - contract delivery_ready_output + probe_required allows cheap probe path
  - provider adapter does not receive unsafe native params

Ограничения:
- Не логировать prompt/user id/job id in metric labels.
- Не хардкодить реальные секреты или live provider data.
- Не ломать mock provider local dev.

Проверки:
- gofmt -l .
- go test ./internal/domain ./internal/worker ./internal/adapter/provider/...
- go vet ./internal/domain ./internal/worker ./internal/adapter/provider/...
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: worker: add provider media contracts
- Push только если явно разрешено.
```

## Stage 4 - CPU-Safe Worker Fast Path

Load impact: high positive impact. This stage removes unnecessary ffmpeg CPU
work for already-safe provider outputs.

Goal: use ffprobe/metadata as cheap validation and skip ffmpeg when provider
output is already delivery-ready.

Prompt:

```text
Цель этапа: добавить fast path для provider video output: если provider contract + probe подтверждают VK-ready mp4/h264 в лимитах, не запускать ffmpeg.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи internal/worker/worker.go, delivery.go, mediaprobe, mediatranscode, artifactservice.
- Реализовать decision flow:
  1. Store provider output as private original artifact.
  2. If video probe policy requires probe, run ffprobe only.
  3. If metadata matches provider contract and delivery target, mark original as delivery-ready or create metadata-only VK-ready variant reference without copying bytes if architecture supports it safely.
  4. If not delivery-ready and MEDIA_VIDEO_TRANSCODE_POLICY=fallback and contract allows transcode, run ffmpeg.
  5. If transcode policy never or fallback not allowed, fail closed and release reservation.
- Не запускать ffmpeg when output already satisfies:
  - mp4 container
  - h264 codec
  - duration/size/resolution/bitrate limits
  - provider contract delivery_ready_output=true
- Add metrics:
  - media_video_fast_path_total{result,operation,modality,provider,model_class}
  - labels must be bounded; model_class must be curated, not raw model id if unbounded.
- Add tests:
  - safe mp4/h264 skips transcoder
  - unsafe codec with fallback disabled fails closed
  - unsafe codec with fallback enabled transcodes
  - missing prober in production policy fails closed
  - dev/mock disabled policy keeps local tests simple
  - delivery chooses safe original/variant according to policy
  - billing capture only after safe delivery

Ограничения:
- Не доставлять raw provider video unless policy says probe-passed delivery-ready original is allowed.
- Не плодить duplicate variants on retry.
- Не делать CPU-heavy work in API/Mini App/VK Bot.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/service/mediaprobe ./internal/service/mediatranscode ./internal/service/artifactservice
- go vet ./internal/worker ./internal/service/mediaprobe ./internal/service/mediatranscode ./internal/service/artifactservice
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: worker: skip transcode for safe provider video
- Push только если явно разрешено.
```

## Stage 5 - Backpressure, Concurrency And Cost Guard

Load impact: medium implementation, high scale protection.

Goal: prevent million-user traffic from overwhelming API, workers, storage,
providers or CPU.

Additional production requirements:

- Prefer separate queue classes or worker pools for provider submit, provider
  poll, media probe, media transcode, delivery and cleanup.
- Add upload concurrency limits before the API buffers many large request
  bodies.
- Add delivery backlog/capture-gap guards so billing capture cannot happen
  early while delivery is overloaded.
- Production limits must be Redis/Postgres-backed or otherwise multi-instance
  safe; process-local-only limits are dev/single-instance only.

Prompt:

```text
Цель этапа: добавить scale guards для media pipeline: concurrency limits, queue backpressure, provider cost budget и controlled degradation.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи Redis queue/worker pool config, job orchestrator, worker generation/poll/delivery flows, current rate limits.
- Добавить config/policy:
  - MEDIA_MAX_CONCURRENT_PROBES
  - MEDIA_MAX_CONCURRENT_TRANSCODES
  - MEDIA_MAX_PENDING_VARIANTS
  - MEDIA_MAX_ACTIVE_VIDEO_JOBS_PER_USER
  - MEDIA_PROVIDER_MAX_ATTEMPTS_PER_JOB
  - MEDIA_PROVIDER_FALLBACK_BUDGET_PER_JOB
  - MEDIA_QUEUE_DEGRADE_THRESHOLD
- Backpressure behavior:
  - if queue/backlog over threshold, reject new expensive jobs before reservation/provider call with safe retry-later error;
  - never create paid job if platform already knows it cannot process it soon;
  - do not retry provider media failures endlessly.
- Provider cost guard:
  - estimate worst-case provider/fallback attempts before submit;
  - refuse jobs over product max cost;
  - never exceed per-job fallback budget.
- Add tests:
  - overloaded media queue rejects before provider call
  - active video job limit per user
  - transcode concurrency limiter
  - provider fallback budget stops extra paid attempts
  - retry-safe idempotency remains intact

Ограничения:
- Do not use high-cardinality metrics labels.
- Do not introduce process-local-only limits for multi-instance production unless documented as dev-only.
- Prefer Redis/Postgres-backed coordination for production limits where needed.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/service/joborchestrator ./internal/platform/config
- go vet ./internal/worker ./internal/service/joborchestrator ./internal/platform/config
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: worker: add media backpressure guards
- Push только если явно разрешено.
```

## Stage 6 - Image Reference Dedupe And Storage Lifecycle

Load impact: medium. Hashing is cheap; lifecycle saves storage cost.

Goal: reduce duplicate validation/storage work and bound long-term storage.

Additional production requirements:

- Dedupe lookup and cleanup predicates must be backed by additive database
  indexes.
- Cleanup must be paginated/batched and must not full-scan large artifact
  tables.
- Content hashes are internal only; never expose hashes as public identities.
- Retention policy must distinguish input references, provider originals,
  delivery variants, failed artifacts and deleted artifacts.

Prompt:

```text
Цель этапа: добавить dedupe/lifecycle для пользовательских reference images и media artifacts без нарушения owner access.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи artifact domain/storage/service and maintenance cleanup.
- Compute content hash while reading bounded upload bytes.
- For same user + same hash + same policy version, reuse existing valid input artifact where safe.
- Add policy version to validation metadata if needed, so future stricter rules do not incorrectly reuse old unsafe artifacts.
- Add lifecycle classes:
  - temp upload
  - input reference
  - provider original
  - delivery variant
  - failed/deleted artifact
- Add retention config:
  - MEDIA_INPUT_RETENTION_DAYS
  - MEDIA_FAILED_RETENTION_DAYS
  - MEDIA_ORIGINAL_RETENTION_DAYS
  - MEDIA_VARIANT_RETENTION_DAYS
- Cleanup must never delete active ready artifacts still referenced by jobs or accessible history unless policy explicitly allows it.
- Add tests for dedupe, owner isolation, retention safety, cleanup metrics.

Ограничения:
- Do not expose storage keys or hashes in public DTOs.
- Hash must not be used as public artifact identity.
- Do not delete data destructively without additive migrations and safe predicates.

Проверки:
- gofmt -l .
- go test ./internal/service/artifactservice ./internal/service/maintenance ./internal/adapter/storage/...
- go vet ./internal/service/artifactservice ./internal/service/maintenance ./internal/adapter/storage/...
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: media: dedupe reference artifacts
- Push только если явно разрешено.
```

## Stage 7 - Circuit Breakers And Provider Quality Scoring

Load impact: low runtime overhead, high business protection.

Goal: automatically stop using provider/model combinations that burn money or
return unsafe media too often.

Prompt:

```text
Цель этапа: добавить provider/model quality guard: если provider/model часто дает probe/transcode/delivery failures, автоматически снижать риск через fallback/disable.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи provider router/fallback/circuit breaker and product observability metrics.
- Add bounded quality states per provider/model_class/modality:
  - healthy
  - degraded
  - disabled
- Trigger degraded/disabled on:
  - provider rate limit spike
  - provider invalid output spike
  - media_probe_failed spike
  - delivery_failed spike
  - provider_success but product_failure ratio
  - provider cost burn without capture
- Implement runtime policy if existing circuit breaker supports it; otherwise add metrics/alerts first and leave automatic disable behind config flag.
- Add metrics/alerts:
  - product_media_waste_total
  - provider_output_invalid_ratio
  - media_delivery_capture_gap
- Telegram alert text must say where to look and what to fix without secrets.

Ограничения:
- Labels bounded only.
- Do not put raw model ids in labels unless model ids are from curated config.
- Do not auto-disable all providers at once without safe fallback/degradation state.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/adapter/provider/... ./internal/platform/metrics
- go vet ./internal/worker ./internal/adapter/provider/... ./internal/platform/metrics
- promtool check rules observability/prometheus/rules/*.yml if rules changed
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: observability: add media provider quality guard
- Push только если явно разрешено.
```

## Stage 8 - Moderation And Safe User UX

Load impact: configurable. Keep expensive scanning async/worker-owned.

Goal: make media failures safe and understandable without leaking sensitive
details or causing user billing loss.

Prompt:

```text
Цель этапа: улучшить UX и safety при media failures: пользователь не платит за недоставленный безопасный результат, а продукт получает actionable internal reason.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи job status/error mapping, Mini App job display, VK delivery/user messages, moderation/scanning paths.
- Standardize user-safe error classes:
  - media_upload_invalid
  - media_upload_too_large
  - media_upload_unsupported
  - media_provider_output_invalid
  - media_processing_unavailable
  - media_delivery_failed
  - media_overloaded_retry_later
- User-facing text:
  - no raw provider details;
  - no prompts;
  - explicitly say credits were not charged when reservation was released.
- Internal logs/traces:
  - bounded error class;
  - correlation id;
  - no raw URL/payload/prompt/PII.
- If moderation/scanning for generated images/videos is enabled, keep it worker-owned and fail closed before delivery.
- Add tests for Mini App and VK Bot safe error rendering where feasible.

Ограничения:
- Do not let frontend infer billing/payment/job truth.
- Do not expose provider-native errors to users.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk
- go vet ./internal/worker ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run build
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: media: improve safe failure UX
- Push только если явно разрешено.
```

## Stage 9 - Production Observability And Load Readiness

Load impact: low if labels stay bounded and sampling is sane.

Goal: make media safety measurable without turning observability into a new
scaling or privacy problem.

Additional production requirements:

- Track policy decisions, not only failures: accepted, rejected, degraded,
  skipped, fallback, kill-switch.
- Track provider waste: provider success/charge without product capture.
- Track upload pressure and rejection reasons without raw filenames, user ids or
  object keys.
- Alerts must mention whether a kill switch is active and whether product money
  is at risk.

Prompt:

```text
Цель этапа: довести media observability до production scale: видеть CPU-risk, provider waste, queue pressure, validation failures и delivery/capture gap без high-cardinality labels.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Изучи internal/platform/metrics, observability/prometheus/rules, Grafana dashboards, Alertmanager templates if present.
- Add or refine metrics:
  - upload_validation_total{result,reason,surface,mime_class}
  - upload_bytes_bucket{surface,mime_class}
  - upload_pixels_bucket{surface,mime_class}
  - media_probe_total{result,error_class,provider_class,model_class}
  - media_transcode_total{policy,result,error_class}
  - media_transcode_cpu_seconds or duration proxy
  - media_fast_path_total{result}
  - media_queue_backlog{queue_class}
  - media_provider_waste_total{provider_class,model_class,reason}
  - media_delivery_capture_gap_total{reason}
- Add alerts:
  - ffmpeg unexpectedly high
  - probe failures spike
  - provider waste spike
  - queue backpressure active
  - media delivery/capture gap
  - upload rejection spike
- Alert text must include:
  - what happened;
  - impact;
  - where to look in Grafana/Prometheus;
  - safe facts to send in chat;
  - what not to send.

Ограничения:
- No user_id, vk_user_id, job_id, artifact_id, idempotency key, prompt, raw URL, raw error in labels.
- Do not expose dashboards/metrics publicly.

Проверки:
- gofmt -l .
- go test ./internal/platform/metrics ./internal/worker ./internal/adapter/inbound/miniapp
- go vet ./internal/platform/metrics ./internal/worker ./internal/adapter/inbound/miniapp
- promtool check rules observability/prometheus/rules/*.yml
- promtool check config observability/prometheus/prometheus.yml
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, commit: observability: harden media safety metrics
- Push только если явно разрешено.
```

## Stage 10 - Load, Chaos And Rollout Drill

Load impact: test-only, but may be heavy. Run locally with bounded synthetic
loads or in staging. Never run against production without explicit approval.

Goal: prove the design does not collapse under concurrency, bad files, provider
failures or queue pressure.

Prompt:

```text
Goal: add production-readiness load/chaos validation for media safety without using live paid providers.

Apply Common Rules from docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

What to do:
- Add or document synthetic tests using mock providers/storage:
  - many concurrent small uploads;
  - oversized uploads;
  - invalid file floods;
  - duplicate upload bursts;
  - provider returns invalid video;
  - provider timeout/rate limit;
  - probe timeout;
  - transcode policy disabled;
  - delivery retry backlog;
  - cleanup under existing active artifacts.
- Tests must not require real VK, YooKassa or paid providers.
- Capture expected safe outcomes:
  - bounded memory;
  - no provider call before validation;
  - no billing capture before safe delivery;
  - backpressure returns safe retry-later;
  - metrics are emitted with bounded labels.
- Add rollout checklist:
  - dark launch metrics;
  - enable upload validation first;
  - enable provider contracts;
  - enable probe;
  - keep transcode disabled;
  - enable fallback transcode only for one model if needed;
  - monitor provider waste and capture gap;
  - rollback/kill switch commands.

Checks:
- gofmt -l .
- go test ./...
- go vet ./...
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run build
- git diff --check
- git status --short --branch

Commit:
- If checks are green, commit: test: add media safety readiness checks
- Push only if explicitly allowed.
```

### Stage 10 Readiness Drill Coverage And Rollout Checklist

Decision date: 2026-06-12.

Scope: test-only and operational documentation. No live VK, YooKassa or paid
provider calls are part of this stage.

#### Bounded Local Synthetic Coverage

The local readiness drill uses in-memory repositories, mock providers and mock
object storage only:

- `internal/adapter/inbound/miniapp` covers:
  - concurrent small reference-image uploads;
  - invalid upload floods that must not store objects;
  - duplicate reference upload bursts that reuse the same-user artifact;
  - oversized upload rejection with a safe public error.
- `internal/service/joborchestrator` covers:
  - capacity/backpressure rejection before job persistence, reservation and
    outbox enqueue;
  - active video job limit rejection before reservation;
  - idempotent existing job replay bypassing transient capacity pressure.
- `internal/worker` covers:
  - provider invalid video output fails closed before delivery;
  - provider timeout/rate-limit behavior remains bounded;
  - required probe missing or failed probe does not enqueue delivery;
  - transcode policy disabled fails closed;
  - transcode concurrency pressure returns a safe retry-later class;
  - provider calls receive only safe product-level params;
  - reservations are released on terminal media failure.
- `internal/worker/delivery` covers:
  - delivery retry failures do not capture credits early;
  - terminal delivery failure releases without duplicate capture;
  - raw provider video is not delivered unless policy and probe allow it.
- `internal/service/maintenance` covers:
  - cleanup deletes only repository-selected inactive media candidates;
  - cleanup is disabled when retention policy is disabled.

#### Expected Safe Outcomes

- API upload memory remains bounded by request body limits and concurrency
  guards.
- Invalid uploads are rejected before storage, job creation, reservation or
  provider submission.
- Duplicate same-user reference uploads reuse policy-compatible artifacts
  without exposing hashes or storage keys.
- Backpressure/capacity rejection returns a safe retry-later path before paid
  work.
- Provider, probe, transcode and delivery failures do not capture credits before
  safe delivery.
- Metrics are emitted only with bounded labels; no user ids, artifact ids, job
  ids, prompts, raw URLs, object keys or raw errors are label values.

#### Production Rollout Checklist

1. Dark-launch metrics first:
   - deploy dashboards and alerts;
   - verify `/metrics`, Grafana, Prometheus, Alertmanager and exporters stay
     private;
   - watch upload validation, media policy decisions, queue backlog, provider
     waste and delivery/capture gap.
2. Enable upload validation before provider rollout:
   - keep user uploads image-only;
   - keep edge/proxy body limits at or below backend limits;
   - monitor upload rejection reasons and MIME classes.
3. Enable provider contracts:
   - allow only explicit provider/model/duration/aspect/resolution contracts;
   - keep provider attempts and fallback attempts conservative;
   - reject unsupported video specs before provider submit.
4. Enable probe:
   - set `MEDIA_VIDEO_PROBE_POLICY=probe_required` for production;
   - verify ffprobe path, timeout and probe concurrency;
   - monitor `media_probe_total` and probe failure alerts.
5. Keep transcode disabled by default:
   - set `MEDIA_VIDEO_TRANSCODE_POLICY=never` for default production rollout;
   - use raw provider output only when contract + probe prove delivery-ready
     media and policy allows it.
6. Enable fallback transcode only for one model if needed:
   - switch one curated model class to fallback;
   - set strict concurrency and backlog limits;
   - watch ffmpeg usage, CPU, queue backlog and provider waste.
7. Monitor business safety:
   - provider waste must stay near zero;
   - delivery/capture gap must stay zero;
   - reservations must release on fail-closed terminal media errors.
8. Rollback and kill switches:
   - stop new risky media jobs with queue/capacity degradation;
   - set `MEDIA_VIDEO_TRANSCODE_POLICY=never`;
   - set raw provider delivery to the strictest safe policy;
   - disable or degrade a bad provider/model class through provider quality
     guard;
   - keep billing repair ledger-safe and never mutate balances manually.

## Stage 11 - Final Production Audit

Load impact: none. Audit only.

Goal: verify the whole media safety chain as a production product system.

Additional final audit requirements:

- Upload limits must exist at frontend, API and edge/proxy layers.
- Reference image metadata privacy must be handled by policy.
- Backpressure must reject before expensive work, reservation or provider
  submit when the system is saturated.
- Kill switches must exist for reference uploads, provider video, probe and
  transcode.
- Load/chaos drill findings must be addressed or explicitly accepted before
  production rollout.

Prompt:

```text
Цель этапа: финально проверить production media safety на архитектуру, безопасность, CPU/load и бизнес-риски.

Применяй Common Rules из docs/MEDIA_SAFETY_PRODUCTION_PLAN.md.

Что сделать:
- Провести architecture audit:
  - VK Bot/Mini App не вызывают providers/ffprobe/ffmpeg directly.
  - User uploads image-only.
  - User video uploads blocked.
  - API does only cheap validation/storage/job creation.
  - Worker owns provider/media processing.
  - ffmpeg is not default for safe provider video.
  - ffprobe/probe policy is explicit.
  - provider media contracts guard unsafe/costly requests before submit.
  - raw provider video is not delivered unless policy + probe + contract allow it.
  - billing capture only after safe delivery/access.
  - retries/fallbacks are bounded by cost budget.
  - cleanup cannot delete active artifacts.
  - metrics labels are bounded.
  - no secrets/raw payloads/prompts/private URLs are logged or committed.
- Check docs/env/runbook are consistent with actual behavior.
- Check git diff for unrelated changes.

Финальные проверки:
- gofmt -l .
- go test ./...
- go vet ./...
- golangci-lint run ./...
- gosec ./...
- govulncheck ./...
- gitleaks detect --redact
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run typecheck
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run build
- npm --prefix web/miniapp run e2e:smoke
- promtool check rules observability/prometheus/rules/*.yml
- promtool check config observability/prometheus/prometheus.yml
- docker compose config if compose changed
- git diff --check
- git status --short --branch

Commit/push:
- Если есть финальные cleanup изменения, commit: media: finalize production safety policy
- Push только если явно разрешено.
- Если хоть одна релевантная проверка падает, не пушь; дай failures и next fix step.
```

### Stage 11 Audit Verdict

Decision date: 2026-06-12.

Verdict: repository-level media safety is production-ready for a gated rollout
after the external edge/proxy upload body limit is configured and verified. Do
not enable public reference-image uploads without that edge control.

Evidence:

- VK Bot, Mini App frontend and Mini App BFF do not call providers, ffprobe or
  ffmpeg directly. Provider and media processing remain worker-owned.
- User uploads are image-only. User video uploads are not accepted.
- Mini App reference uploads are protected by:
  - `MEDIA_REFERENCE_UPLOADS_ENABLED` kill switch;
  - per-API-instance `MEDIA_MAX_CONCURRENT_UPLOADS`;
  - backend byte, width, height and pixel limits;
  - backend JPEG/PNG-only default with WebP behind `MEDIA_REFERENCE_WEBP_ENABLED`.
- API upload handling performs cheap validation and private artifact storage
  only. It does not run providers, ffprobe or ffmpeg.
- Reference image privacy is handled by worker-owned policy: raw uploaded bytes
  remain private, JPEG/PNG references are sanitized before provider forwarding,
  and unsupported metadata-bearing formats fail closed before provider submit.
- Provider media contracts guard video model, duration, aspect, resolution and
  cost before provider submit.
- `ffmpeg` is not the default path. `MEDIA_VIDEO_TRANSCODE_POLICY=never` is the
  CPU-safe default; fallback transcode is explicit and concurrency-limited.
- Probe policy is explicit. Production defaults to `probe_required`; local/dev
  can stay disabled for mock-backed runs.
- Raw provider video delivery is allowed only when raw delivery policy, probe
  metadata and provider contract prove a delivery-ready output.
- Backpressure rejects expensive media jobs before persistence, reservation,
  outbox enqueue and provider submit when queue pressure is degraded.
- Billing capture remains after safe delivery/access. Unsafe media, moderation,
  provider, probe, transcode and terminal delivery failures release reservation
  without capture.
- Retry, fallback, probe and transcode attempts are bounded by config and tests.
- Cleanup is lifecycle-specific, batched by repository predicates and must not
  delete active artifacts referenced by jobs or deliveries.
- Metrics and alerts use bounded labels only and explicitly forbid user ids,
  job ids, artifact ids, prompts, raw URLs, private storage keys and raw errors.

Accepted rollout condition:

- The repository cannot enforce Cloudflare/nginx/tunnel body limits by itself.
  Production rollout must configure an external request body limit at or below
  `MEDIA_MAX_IMAGE_UPLOAD_BYTES`, then verify oversized uploads are rejected at
  the edge before enabling `MEDIA_REFERENCE_UPLOADS_ENABLED=true`.

## Default Product Policy Decisions

Use these defaults unless the owner explicitly changes them:

- User videos: not accepted.
- User image uploads: JPEG/PNG by default; WebP only behind
  `MEDIA_REFERENCE_WEBP_ENABLED` and product policy.
- SVG/GIF/PDF/archive/audio/video: rejected.
- Upload validation: cheap, bounded, no full decode/re-encode in API.
- Upload limits must exist at browser, API and edge/proxy layers.
- Public reference-image uploads must stay behind
  `MEDIA_REFERENCE_UPLOADS_ENABLED` until edge/proxy body limits are verified.
- For large-scale/high-size uploads, prefer private staged direct-to-object
  upload over API-proxied buffering.
- Reference image metadata must not be forwarded to providers unless sanitized
  or explicitly allowed by policy.
- Image normalization/re-encode: optional future async worker feature only.
- Provider video output: contract-constrained before submit.
- Probe: required for production provider video unless a future trusted-provider
  mode is explicitly accepted.
- Transcode: never by default, fallback only when provider/model contract allows
  it and cost/concurrency budget permits.
- Raw provider video delivery: never unless probe passed and contract says output
  is delivery-ready.
- Billing: capture only after safe delivery/access; release reservation on
  unsafe media or processing failure.
- Backpressure: reject/degrade before expensive work when queues or workers are
  saturated.
- Rollout: dark-launch metrics first; use kill switches for risky media paths.
- Observability: bounded labels only; no raw ids, prompts, URLs or payloads.
