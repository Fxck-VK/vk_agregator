# Agent Prompts: Video/Media Pipeline + Mini App Frontend Quality

This is a temporary copy-paste prompt book for staged implementation. It is not
an active machine-readable plan. Send one stage prompt at a time.

Do not paste secrets, real `.env` values, API keys, tokens, full VK launch
params, prompts, raw PII, raw provider/payment payloads or private artifact URLs
into chat.

## Stage 1 Baseline Findings

These findings were collected during the Mini App baseline audit on
2026-06-12. Use them to keep later stages focused.

Current quality gates:

- `web/miniapp/package.json` has `dev`, `build` and `preview` only.
- `npm --prefix web/miniapp run build` passes.
- `web/miniapp/tsconfig.json` already has `strict`,
  `noUnusedLocals`, `noUnusedParameters` and
  `noFallthroughCasesInSwitch`.
- No frontend `lint`, `test` or `e2e:smoke` scripts exist yet.
- No frontend unit/spec files were found under `web/miniapp`.

Current safety strengths:

- Mini App uses backend `/miniapp/*` APIs as the source of truth.
- Launch params are sent to the backend through `X-Launch-Params`.
- Artifact preview uses backend-owned artifact IDs, authenticated fetch and
  temporary blob URLs; raw launch params are not placed into media `src`.
- `localStorage` chat state is UI-only: active thread id is stored, legacy chat
  content is removed when encountered.
- `dangerouslySetInnerHTML` was not found in Mini App source.
- Write paths already use frontend idempotency keys.

Main gaps to close next:

- Add ESLint before broader refactors: React hooks, unused values, promise
  safety and TypeScript safety.
- Add unit tests for `src/api/client.ts` telemetry/route/launch-param safety,
  `src/chat/store.ts` localStorage cleanup, artifact filename/display helpers
  and status/error mapping.
- Add Playwright smoke with mock backend for VK WebView-like mobile flows:
  launch, balance, chat job, create screens, payments, error states and no
  sensitive data leaks.
- Add automated checks for `confirmation_url` handling in payment UI so frontend
  stays navigation-only and never treats redirect as balance proof.
- Add tests around chat polling lifecycle/reload recovery; current code has
  timer cleanup and job-id dedup, but no automated regression coverage.
- Add mobile overflow smoke because CSS has many overflow/scroll rules but no
  automated viewport verification.

Recommended next order:

1. Stage 2 - TypeScript Strict + ESLint.
2. Stage 3 - Frontend Unit Tests.
3. Stage 4 - Playwright Smoke.
4. Only then consider stricter TypeScript flags such as
   `noUncheckedIndexedAccess` or `exactOptionalPropertyTypes`.

## Stage 2 Implementation Notes

Status: implemented locally on 2026-06-12.

What was added:

- `web/miniapp/package.json` now has `lint`.
- ESLint flat config was added for Mini App source files.
- Dev dependencies were added for ESLint, `@eslint/js`,
  `typescript-eslint`, React Hooks linting and browser globals.
- The lint gate runs with `--max-warnings=0`.

Rules intentionally included:

- React Hooks rules: `rules-of-hooks` and `exhaustive-deps`.
- Promise safety: `@typescript-eslint/no-floating-promises` and
  `@typescript-eslint/no-misused-promises`.
- Unused values with `_` escape hatch for intentional placeholders.
- `no-console` with only `console.warn` / `console.error` allowed.
- `no-debugger`.

Initial lint findings fixed:

- Explicitly marked fire-and-forget VK bridge initialization and payment-list
  loading promises with `void`.
- Stabilized chat polling cleanup by copying refs inside effect cleanup.
- Stabilized `WorkflowMode.openExistingJob` dependency chain with
  `useCallback`.
- Kept intentionally unused `_job` / `_loading` placeholders valid through the
  `_` ignore pattern instead of deleting behavior-facing props.

Carry-forward tasks:

- Stage 3 should add unit tests around the exact safety areas exposed by lint:
  fire-and-forget telemetry/API calls, launch param handling, localStorage
  cleanup and artifact URL generation.
- Stage 4 should smoke-test chat polling lifecycle and mobile overflow because
  lint can catch dependency shape but not runtime WebView behavior.
- Do not enable `recommendedTypeChecked` or stricter TS flags until Stage 3/4
  tests are in place; the current gate is intentionally practical, not maximal.

## Stage 3 Implementation Notes

Status: implemented locally on 2026-06-12.

What was added:

- `web/miniapp/package.json` now has `test`.
- `web/miniapp/vitest.config.ts` was added with jsdom environment.
- Dev dependencies were added for `vitest` and `jsdom`.
- Four focused test files were added under `web/miniapp/src/**`.

Tests now cover:

- `src/api/client.ts` telemetry route sanitization: query/hash stripping,
  UUID redaction and no prompt/launch/private fragments in telemetry route
  labels.
- `src/api/client.ts` launch/referral helper behavior: query/hash normalization,
  launch params only when VK identity exists, bridge launch-param serialization
  and public referral-code shape.
- `src/api/client.ts` artifact URL guard: UUID-only URLs, rejecting raw URLs and
  launch-param-bearing strings.
- `src/chat/store.ts` localStorage safety: safe active thread ids, unsafe value
  rejection, legacy chat content cleanup and full local cleanup.
- `src/utils/artifactDownload.ts` filename slugging, mime extension mapping and
  job-id fallback.
- `src/utils/jobDisplay.ts` title truncation, prompt title selection, history
  dedupe and count label stability.

Small source changes made for testability:

- Exported pure helper functions from `src/api/client.ts`:
  `telemetryRoute`, `telemetryLabel`, `normalizeRawParams`,
  `referralCodeFromRaw`, `launchParamsFromLocation` and
  `stringifyBridgeLaunchParams`.
- These exports do not change runtime behavior and do not expose secrets or
  backend authority; they only make existing pure frontend logic testable.

Carry-forward tasks:

- Stage 4 should cover runtime UI behavior that unit tests do not prove:
  VK WebView-like launch, mocked `/miniapp/*` API flows, payment unavailable
  states, media blob `src` checks, chat polling lifecycle/reload behavior and
  mobile overflow.
- Consider testing `openExternalUrl` later only if bridge/window behavior can be
  mocked without flaky browser assumptions.

## Stage 4 Implementation Notes

Status: implemented locally on 2026-06-12.

What was added:

- `web/miniapp/package.json` now has `e2e:smoke`.
- `web/miniapp/playwright.config.ts` was added with a Pixel 5-like viewport,
  `127.0.0.1:5174` base URL and failure-only traces/screenshots.
- `web/miniapp/scripts/e2e-smoke.mjs` was added as a Windows-safe runner that
  starts Vite, runs Playwright with mocked backend routes and stops the owned
  dev server.
- `web/miniapp/e2e/miniapp.smoke.spec.ts` was added.
- Dev dependency `@playwright/test` was added.

Smoke now covers:

- VK WebView-like mobile launch with fake launch params.
- App renders without blank screen.
- Narrow mobile viewport has no document/body horizontal overflow.
- Mocked balance loads in happy path and safe UI survives auth/API failures.
- Chat prompt submit creates a mocked job and polling moves it to a terminal
  safe text result.
- Create screen opens image and video modes; image generation moves through
  mocked job polling to a blob media result.
- Payment screen renders active payment continuation and payment history
  without treating redirect as balance proof.
- API auth/rate-limit/service errors keep the screen usable.
- Full launch markers/private storage URLs do not appear in UI text, media
  `src`, telemetry bodies or captured console output.

Tooling note:

- The first Playwright webServer approach passed tests but the npm command did
  not exit cleanly on local Windows. This was recorded in
  `.agents/logs/errors.jsonl` and resolved by the dedicated Node runner.

Carry-forward tasks:

- If frontend UI grows, add more smoke for active payment creation without
  navigating to YooKassa and for reload recovery from persisted pending jobs.
- Consider adding accessibility assertions once stable labels/text encoding are
  normalized across Windows console output.

## Common Rules To Include In Every Stage

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
- Inbound events, job creation, delivery и payment flows должны оставаться idempotent.
- /metrics, Grafana, Prometheus, Loki, Tempo, Alertmanager, OTel и exporters не должны становиться публичными.

Качество:
- Делай минимальный scoped diff.
- Не трогай unrelated изменения.
- Не делай визуальный редизайн, если этап не про UI.
- Не ломай local mock-backed dev runtime.
- После этапа покажи changed files, checks run/skipped, security/architecture impact и git status.

Логи:
- Не записывай routine work.
- В .agents/logs/errors.jsonl пиши только reusable, non-obvious, repeated errors.
- Запись должна быть sanitized: без секретов, prompts, launch params, raw payloads, private URLs, raw PII.

Commit/push:
- После крупного завершенного этапа проверь git status, diff, отсутствие секретов и unrelated изменений.
- Делай один логический commit только после зеленых релевантных проверок.
- Push делай только если я явно разрешил push в этом этапе.
- Если проверки падают, не пушь; объясни что упало и что нужно сделать.
```

## Stage 1 - Frontend Quality Baseline

```text
Цель этапа: провести baseline-аудит качества VK Mini App без изменения поведения продукта.

Применяй Common Rules.

Что сделать:
- Изучи web/miniapp/package.json, tsconfig.json и ключевые файлы src/api, src/chat, src/settings, src/workflow.
- Проверь, какие quality gates уже есть: build, TypeScript, telemetry safety, localStorage safety.
- Найди слабые места: launch params, payment URLs, artifact URLs, client telemetry, error states, mobile viewport.
- Не меняй код, если не найдешь очевидную маленькую безопасную правку.

Проверки:
- npm --prefix web/miniapp run build
- git status --short --branch

Результат:
- Дай короткий список конкретных файлов/мест риска.
- Предложи порядок включения strict/lint/test/e2e.
- Не делай commit/push на этом этапе, если не было изменений.
```

## Stage 2 - TypeScript Strict + ESLint

Baseline findings to carry into this stage:

- `web/miniapp/package.json` currently has no `lint` script.
- `web/miniapp/tsconfig.json` is already strict enough for baseline:
  `strict`, `noUnusedLocals`, `noUnusedParameters`,
  `noFallthroughCasesInSwitch`.
- Do not enable noisy TypeScript flags first. Add ESLint and make it green
  before considering `noUncheckedIndexedAccess` or
  `exactOptionalPropertyTypes`.
- Prioritize rules that protect the existing weak spots: React hook deps,
  unhandled promises, unsafe browser globals, accidental console/debug output,
  unused values and accidental unsafe `any`.
- Expected touched files: `web/miniapp/package.json`,
  `web/miniapp/package-lock.json`, a new ESLint config, possibly small fixes
  under `web/miniapp/src/**`.

```text
Цель этапа: добавить практичные frontend quality gates для Mini App.

Применяй Common Rules.

Что сделать:
- Усиль TypeScript настройки в web/miniapp/tsconfig.json без большого рискованного переписывания.
- Добавь ESLint для React + TypeScript.
- Добавь npm script: lint.
- Включи правила для React hooks, unused values, unsafe/floating promises и базовых type-safety проблем.
- Исправь только нужные lint/type ошибки, без UI-редизайна и unrelated refactor.

Проверки:
- npm --prefix web/miniapp run build
- npm --prefix web/miniapp run lint
- git diff --check
- git status --short --branch

Commit:
- Если все проверки зеленые, сделай commit с сообщением:
  frontend: add miniapp lint baseline
- Push только если я явно разрешил.
```

## Stage 3 - Frontend Unit Tests

Baseline findings to carry into this stage:

- No frontend unit/spec files exist yet under `web/miniapp`.
- Highest-value test targets from Stage 1:
  - `src/api/client.ts` launch params parsing/cache/dev fallback.
  - `src/api/client.ts` telemetry route normalization and payload sanitization.
  - `src/api/client.ts` artifact URL guard: UUID-only, authenticated fetch,
    blob URL output, no launch params in media `src`.
  - `src/chat/store.ts` active thread id validation and legacy unsafe storage
    cleanup.
  - `src/utils/artifactDownload.ts` filename/slug generation.
  - `src/utils/jobDisplay.ts` and API status/error label mapping.
  - `src/utils/openExternalUrl.ts` fallback behavior if it can be tested
    without flaky bridge/browser behavior.
- Add tests that prove frontend telemetry never includes prompts, full launch
  params, raw user ids, cookies, raw URLs or private artifact URLs.
- Payment UI should stay navigation-only: test helper-level behavior around
  `confirmation_url` where practical, but do not make frontend infer payment
  success from redirects.

```text
Цель этапа: добавить быстрые unit tests для sensitive frontend логики Mini App.

Применяй Common Rules.

Что сделать:
- Добавь lightweight test runner для Vite/React/TypeScript.
- Добавь npm script: test.
- Покрой focused tests:
  - API route normalization и telemetry payload safety.
  - Запрет prompts/full launch params/user ids/cookies/raw URLs/private artifact URLs в telemetry.
  - Artifact filename generation.
  - Chat localStorage safety и cleanup unsafe legacy fields.
  - Job display/status mapping.
  - openExternalUrl fallback, если это удобно тестировать без flaky browser behavior.

Ограничения:
- Tests не должны требовать VK, YooKassa, live providers или реальные секреты.
- Не делай snapshots с sensitive данными.

Проверки:
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run build
- git diff --check
- git status --short --branch

Commit:
- Если все проверки зеленые, сделай commit с сообщением:
  frontend: add miniapp safety tests
- Push только если я явно разрешил.
```

## Stage 4 - Playwright Smoke

Baseline findings to carry into this stage:

- CSS has many overflow/scroll protections, but no automated mobile viewport
  check. Add explicit narrow viewport assertions.
- Chat polling has timer cleanup and job-id dedup, but no automated coverage
  for lifecycle/reload recovery. Smoke should cover a mocked pending job moving
  to terminal state.
- Artifact media is intended to use authenticated fetch plus blob URLs. Smoke
  should assert media `src` does not contain full launch params or private
  backend/storage URLs.
- Payment redirect is UI navigation only. Smoke should cover payment product
  loading, active payment continuation and provider-unavailable/error state
  without treating redirect as balance proof.
- Use route interception or mock backend responses for `/miniapp/*`; do not
  require VK bridge, YooKassa or live providers.

```text
Цель этапа: добавить стабильный Playwright smoke для Mini App в VK WebView-like режиме.

Применяй Common Rules.

Что сделать:
- Добавь Playwright config и npm script: e2e:smoke.
- Используй mock backend или route interception. Не дергай real VK, YooKassa, paid providers.
- Проверяй mobile viewport / VK WebView-like размеры.
- Smoke scenarios:
  - App renders without blank screen.
  - No horizontal overflow on narrow viewport.
  - Balance loads or safe error state is shown.
  - Chat prompt submit creates/polls a mocked job state.
  - Image/video create screens open.
  - Payment products/history render or safe unavailable state is shown.
  - Auth/rate-limit/API errors do not break the screen.
  - Full launch params do not appear in UI, media src, telemetry body or console output captured by tests.

Проверки:
- npm --prefix web/miniapp run e2e:smoke
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run build
- git diff --check
- git status --short --branch

Commit:
- Если все проверки зеленые, сделай commit с сообщением:
  frontend: add miniapp smoke tests
- Push только если я явно разрешил.
```

## Stage 5 - Media Config + Artifact Metadata

```text
Цель этапа: подготовить backend schema/config для безопасного video/media pipeline.

Применяй Common Rules.

Что сделать:
- Изучи current video_generate flow через VK Bot, Mini App, worker, provider adapters, artifacts и VK delivery.
- Добавь безопасный env/config слой:
  - MEDIA_PIPELINE_ENABLED
  - FFPROBE_PATH
  - FFMPEG_PATH
  - MEDIA_MAX_VIDEO_SIZE_BYTES
  - MEDIA_MAX_VIDEO_DURATION_SEC
  - MEDIA_MAX_VIDEO_WIDTH
  - MEDIA_MAX_VIDEO_HEIGHT
  - MEDIA_MAX_VIDEO_BITRATE
  - MEDIA_ALLOWED_VIDEO_CONTAINERS
  - MEDIA_ALLOWED_VIDEO_CODECS
  - MEDIA_PROBE_TIMEOUT
  - MEDIA_TRANSCODE_TIMEOUT
- Local dev должен работать без ffmpeg/ffprobe, если media pipeline выключен.
- Добавь/расширь artifact metadata для safe media facts: size, duration, width, height, codec, container, bitrate, probe status.
- Если нужны artifact variants, добавь additive migration/model без разрушения старых данных.

Ограничения:
- Миграции только additive.
- Не храни private storage paths в публичных DTO.
- Не меняй VK/Mini App handlers так, чтобы они знали про ffmpeg/ffprobe.

Проверки:
- gofmt -l .
- go test ./internal/domain ./internal/platform/config ./internal/service/artifactservice ./internal/adapter/storage/...
- go vet ./internal/domain ./internal/platform/config ./internal/service/artifactservice ./internal/adapter/storage/...
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, сделай commit с сообщением:
  media: add video pipeline metadata config
- Push только если я явно разрешил.
```

## Stage 6 - Video Probe

```text
Цель этапа: worker должен fail-closed проверять видео перед delivery/capture.

Применяй Common Rules.

Что сделать:
- Добавь media probe service/port для ffprobe.
- Запускай ffprobe с context timeout.
- Парси JSON output в bounded internal metadata.
- Валидируй:
  - file size
  - duration
  - video stream exists
  - container
  - codec
  - width/height
  - bitrate
- Интегрируй probe в worker после получения provider video output и до delivery/capture.
- Unsafe/probe failed video должно завершаться safe internal error class, без доставки и без capture.

Ограничения:
- Не логируй raw file path, private URL, provider raw response или prompt.
- Если ffprobe отсутствует, поведение должно быть controlled: disabled local или fail-closed production по config.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/service/artifactservice ./internal/platform/config
- go vet ./internal/worker ./internal/service/artifactservice ./internal/platform/config
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, сделай commit с сообщением:
  worker: validate video media before delivery
- Push только если я явно разрешил.
```

## Stage 7 - Video Transcode + VK-Ready Variants

```text
Цель этапа: worker создает VK-ready video variant и не доставляет raw provider output.

Применяй Common Rules.

Что сделать:
- Добавь media transcode service/port для ffmpeg.
- Запускай ffmpeg с context timeout и bounded output.
- Определи VK-ready profile:
  - mp4
  - h264
  - faststart
  - bounded resolution
  - bounded bitrate
  - audio aac или no-audio, по текущей логике проекта
- Сохраняй original и VK-ready artifact variants.
- Сохраняй safe metadata variant.
- Worker/delivery должен выбирать VK-ready variant, если pipeline enabled и variant готов.

Ограничения:
- Transcode CPU/IO high load. Не делай бесконтрольные параллельные ffmpeg процессы.
- Сохрани retry/idempotency: повтор worker task не должен плодить дубликаты variants.
- Не добавляй большие бинарники в git.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/service/artifactservice ./internal/adapter/storage/...
- go vet ./internal/worker ./internal/service/artifactservice ./internal/adapter/storage/...
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, сделай commit с сообщением:
  media: add vk-ready video variants
- Push только если я явно разрешил.
```

## Stage 8 - Delivery + Billing Safety

```text
Цель этапа: связать media pipeline с VK delivery и ledger без нарушения инвариантов.

Применяй Common Rules.

Что сделать:
- VK delivery должен брать vk_document/vk_video variant, а не raw original, когда variant готов.
- Проверить idempotent delivery: random_id, delivery records, retry behavior.
- Проверить billing:
  - capture только после успешной safe delivery.
  - probe/transcode failures не capture.
  - retries не создают duplicate ledger entries.
- Проверить Mini App artifact reads: owner-checked, без private storage key leaks.
- Добавить/обновить worker/delivery tests для success, retry, probe fail, transcode fail, idempotency.

Ограничения:
- VK handler и Mini App BFF не должны знать про media internals.
- Provider adapters не должны знать про VK delivery или billing.

Проверки:
- gofmt -l .
- go test ./internal/worker ./internal/adapter/delivery/vk ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk
- go vet ./internal/worker ./internal/adapter/delivery/vk ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, сделай commit с сообщением:
  worker: deliver safe video variants
- Push только если я явно разрешил.
```

## Stage 9 - Media Cleanup + Observability

```text
Цель этапа: добавить lifecycle cleanup и private observability для media pipeline.

Применяй Common Rules.

Что сделать:
- Добавь bounded cleanup policy для old original media, variants и thumbnails.
- Не удаляй active artifacts и не ломай owner access.
- Добавь private bounded-label metrics:
  - probe result
  - transcode result
  - transcode duration
  - media bytes
  - variant backlog
  - cleanup deleted
- Labels только bounded: result, operation, modality, variant_type, error_class.
- Нельзя labels: user_id, vk_user_id, job_id, artifact_id, prompt, raw_url, raw_error.
- Обнови private Grafana/Prometheus alerts/dashboards, если уместно.
- Обнови RUNBOOK/.env.example/docs только если изменился запуск/env/operation behavior.

Проверки:
- gofmt -l .
- go test ./internal/service/maintenance ./internal/platform/metrics ./internal/worker
- go vet ./internal/service/maintenance ./internal/platform/metrics ./internal/worker
- promtool check rules observability/prometheus/rules/*.yml если rules изменились
- docker compose config если compose изменился
- git diff --check
- git status --short --branch

Commit:
- Если проверки зеленые, сделай commit с сообщением:
  observability: add media pipeline monitoring
- Push только если я явно разрешил.
```

## Stage 10 - Final Audit + Push

```text
Цель этапа: финально проверить frontend quality + media pipeline вместе и подготовить безопасный push.

Применяй Common Rules.

Что сделать:
- Проведи architecture audit:
  - VK Bot/Mini App не вызывают providers/ffprobe/ffmpeg напрямую.
  - Worker owns provider/media pipeline.
  - Billing ledger invariant сохранен.
  - Artifact owner checks сохранены.
  - Observability private и bounded labels.
  - Frontend telemetry safe.
- Проверь docs/env/runbook соответствуют фактическому поведению.
- Проверь git diff на unrelated changes.

Финальные проверки:
- gofmt -l .
- go test ./...
- go vet ./...
- golangci-lint run ./... если доступен
- gosec ./... если доступен
- govulncheck ./... если доступен
- gitleaks detect --redact
- npm --prefix web/miniapp run build
- npm --prefix web/miniapp run lint
- npm --prefix web/miniapp run test
- npm --prefix web/miniapp run e2e:smoke
- promtool check config/rules если observability менялась
- docker compose config если compose менялся
- git diff --check
- git status --short --branch

Commit/push:
- Если есть незакоммиченные изменения после финального аудита, сделай один final cleanup commit только если это логически нужно.
- Push в текущую рабочую ветку только если я явно разрешил push.
- Если хоть одна релевантная проверка падает, не пушь; дай точный список failures и следующий fix step.
```

## Quick Commit Message Map

```text
frontend: add miniapp lint baseline
frontend: add miniapp safety tests
frontend: add miniapp smoke tests
media: add video pipeline metadata config
worker: validate video media before delivery
media: add vk-ready video variants
worker: deliver safe video variants
observability: add media pipeline monitoring
docs: update media pipeline runbook
```
