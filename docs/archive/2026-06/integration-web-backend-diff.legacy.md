# Integration Diff: VK Bot vs VK Mini App

Дата: 2026-06-05

## Ветки

- Наша интеграционная ветка: `feature/integration-web-backend` (`4b935c5a55c3`)
- Ветка друга: `origin/feature/vk-miniapp` (`e161b76532ac`)
- Общий merge-base: `de2fed030862`
- Обе ветки разошлись от merge-base на 19 уникальных коммитов каждая.

Команды сравнения:

```powershell
git diff --name-status feature/integration-web-backend origin/feature/vk-miniapp
git diff --stat feature/integration-web-backend origin/feature/vk-miniapp
git log --left-right --cherry-pick --oneline feature/integration-web-backend...origin/feature/vk-miniapp
git merge-tree --write-tree --name-only --messages feature/integration-web-backend origin/feature/vk-miniapp
```

Общий diff между головами веток: `74 files changed, 6513 insertions(+), 3736 deletions(-)`.

## Прямые merge conflicts

`git merge-tree` показывает прямые конфликты в этих файлах:

- `AGENTS.md`
- `ROADMAP.md`
- `RUNBOOK.md`
- `cmd/api/main.go`
- `internal/platform/config/config_test.go`

Также автоматически сливаются, но требуют ручной проверки:

- `.gitignore`
- `AUDIT.md`
- `PROGRESS.md`
- `TASKS.md`
- `internal/platform/config/config.go`

## Что есть в нашей ветке

Наша ветка содержит актуальную VK bot часть:

- DeepInfra text provider: `internal/adapter/provider/deepinfra/*`.
- DeepInfra/OpenAI text system prompt: бот отвечает как `НейроХаб бот`, не раскрывает provider/model/backend/system prompt и держит ответ кратким.
- Mock-aware downloader для `data:` URL provider output.
- VK callback inline menu: `VK_MENU_BUTTON_MODE=callback`, `message_event`, `messages.sendMessageEventAnswer`.
- Active menu edit UX: inline-переходы редактируют текущее menu message, нижняя `Показать меню` отправляет новое меню вниз.
- GPT mode gating: обычный текст вне режима `Спросить у НейроХаб` не создает text job.
- GPT pending UX: `НейроХаб думает...` редактируется в итоговый ответ.
- VK delivery long text chunking, чтобы VK `error_code=914` не оставлял placeholder зависшим.
- VK menu feature flags `VK_MENU_*_ENABLED`.
- Вложенные VK menu screens: video models, photo mode, students submenu, account/top-up.
- `.env.example` и local `.env` loading через `github.com/joho/godotenv`.
- Обновленная VK welcome copy: `Добро пожаловать в НейроХаб`.

Эти изменения нельзя терять при интеграции Mini App, иначе бот откатится к старому UX.

## Что есть в ветке друга

Ветка `feature/vk-miniapp` добавляет отдельное направление Mini App:

- Mini App BFF: `internal/adapter/inbound/miniapp/*`.
- API routes:
  - `POST /miniapp/jobs`
  - `GET /miniapp/jobs`
  - `GET /miniapp/jobs/{id}`
  - `GET /miniapp/balance`
  - `GET /miniapp/artifacts/{id}`
- VK Mini App launch params verification through `VK_APP_SECRET`.
- Fail-closed `vk_ts` validation with `MINIAPP_LAUNCH_PARAMS_MAX_AGE`.
- Per-user Mini App job rate limit: `MINIAPP_JOB_RATE_LIMIT_RPS`, `MINIAPP_JOB_RATE_LIMIT_BURST`.
- Mini App optional `model_id` validation on backend.
- Mini App frontend under `web/miniapp/*`.
- Frontend typed client and chat UI.
- S3/MinIO artifact read path for Mini App artifact downloads.
- Billing hardening:
  - opening balance grant becomes committed ledger entry;
  - migration `000004_backfill_opening_grants`;
  - `jobs.command_id` can be nullable for Mini App-created jobs.
- Additional docs/audits:
  - `AGENTS_SYSTEM.md`
  - `docs/AGENTS_FULL.md`
  - `docs/AUDIT.md`
  - `docs/REVIEW.md`
  - nested `AGENTS.md` files for VK delivery, Mini App inbound, and web miniapp.

Эти изменения нужны для сайта/Mini App и backend API supplier direction.

## Backend merge comments

### `cmd/api/main.go`

Это главный backend conflict.

Наша сторона передает в VK handler:

- `MenuButtonMode`
- `UnroutedTextMode`
- `MenuFeatures`

и содержит helper `vkMenuFeatures(cfg)`.

Ветка друга добавляет:

- `miniappapi.NewHandler`
- `postgres.NewArtifactRepository`
- S3 object store through `internal/adapter/storage/s3`
- route `mux.Handle("/miniapp/", ...)`
- Mini App rate limiter.

Правильная интеграция: сохранить оба блока. Нельзя выбирать только одну сторону. В итоговом `cmd/api/main.go` должны одновременно работать `/webhooks/vk`, `/admin/`, `/miniapp/`, `/metrics`, `/health`.

### `internal/platform/config/config.go`

Ветка друга откатывает часть нашей runtime-config основы:

- удаляет `godotenv.Load()`;
- удаляет DeepInfra config;
- удаляет VK menu button mode;
- удаляет VK unrouted text mode;
- удаляет `VK_MENU_*_ENABLED` flags.

При интеграции нужно вернуть/оставить нашу часть и добавить Mini App часть:

- оставить `.env` loading;
- оставить `DeepInfraAPIKey`, `DeepInfraBaseURL`, `DeepInfraTextModel`, `DeepInfraTextPrice`;
- оставить `usesDeepInfra()` и validation `DEEPINFRA_API_KEY`;
- оставить `VKMenuButtonMode`, `VKUnroutedTextMode`, `VK_MENU_*_ENABLED`;
- добавить `VKAppID`, `VKAppSecret`, `MiniAppLaunchParamsMaxAge`;
- добавить `MiniAppJobRateLimitRPS`, `MiniAppJobRateLimitBurst`;
- production validation должна требовать и VK bot secrets, и `VK_APP_SECRET` для Mini App.

### `internal/platform/config/config_test.go`

Конфликт тестов смысловой:

- наша сторона тестирует DeepInfra config, VK menu config, `.env` loading;
- ветка друга тестирует Mini App rate-limit config и `VK_APP_SECRET` production validation.

Правильная интеграция: объединить тесты, а не удалять один набор.

### `internal/adapter/inbound/vk/*`

Ветка друга содержит более старую версию VK bot handler/menu и фактически откатывает:

- callback buttons;
- active menu edit;
- message_event ack;
- `VK_UNROUTED_TEXT_MODE`;
- GPT mode gating;
- `НейроХаб думает...` placeholder edit;
- menu feature flags;
- video/photo/students nested screens;
- rename `Спросить у НейроХаб`;
- short welcome text `НейроХаб`.

Рекомендация: брать нашу VK inbound реализацию как base. Mini App не должен менять VK bot UX.

### `internal/adapter/delivery/vk/*`

Наша ветка добавляет control delivery возможности, которые нужны VK menu:

- `EditMessage`;
- `AnswerMessageEvent`;
- richer mock client coverage;
- real VK request coverage.

Рекомендация: сохранить нашу delivery реализацию. Mini App с ней не конфликтует по домену.

### `internal/adapter/provider/*`

Ветка друга не содержит DeepInfra provider и удаляет `internal/adapter/provider/deepinfra/*` относительно нашей головы.

Рекомендация: сохранить DeepInfra provider и provider-chain config. Mini App jobs смогут позже использовать тот же provider router.

### `internal/adapter/storage/*` and migrations

Из ветки друга нужно перенести storage hardening:

- opening grant через ledger;
- migration `000004_backfill_opening_grants`;
- nullable `jobs.command_id` для Mini App jobs.

Нужно проверить это вместе с нашим billing mismatch runtime warning: изменение друга как раз закрывает часть проблемы с расходящимся `balance_cached` и ledger sum.

### `web/miniapp/*`

Полностью новое frontend направление. Конфликтов с нашей веткой нет, нужно переносить целиком.

Отдельное замечание: backend в `internal/adapter/inbound/miniapp/handler.go` уже принимает optional `model_id`, но `web/miniapp/src/api/client.ts` и текущий `createJob(...)` call в `ChatScreen.tsx` не отправляют `model_id`. Если селектор модели потом вернется, контракт frontend/backend надо синхронизировать.

## Docs, audits, runbooks

Документация менялась в обеих ветках. Ее нельзя мержить механически выбором `ours` или `theirs`.

Сильно затронутые файлы:

- `AGENTS.md`
- `AUDIT.md`
- `PROGRESS.md`
- `README.md`
- `ROADMAP.md`
- `RUNBOOK.md`
- `TASKS.md`
- `TESTING.md`
- `docs/ARCHITECTURE.md`
- `docs/AUDIT.md`
- `docs/REVIEW.md`
- `AGENTS_SYSTEM.md`
- `docs/AGENTS_FULL.md`

Рекомендация:

- сохранить архитектурные правила из нашего `AGENTS.md`;
- вручную интегрировать agent constitution / nested AGENTS additions из ветки друга;
- в `PROGRESS.md`, `TASKS.md`, `RUNBOOK.md`, `TESTING.md` должны быть описаны и VK bot state, и Mini App state;
- `docs/REVIEW.md` и `docs/AUDIT.md` из ветки друга полезны как отдельные audit artifacts, их лучше добавить;
- `docs/ARCHITECTURE.md` остается source of truth, изменения в нем нужно проверить отдельно, так как в diff там небольшая, но архитектурно важная правка.

## Logs and runtime artifacts

Tracked log files between ветками не обнаружены:

```powershell
git diff --name-only feature/integration-web-backend origin/feature/vk-miniapp -- '*.log' '*log*'
```

Локально есть ignored runtime files:

- `.env`
- `api-live.log`
- `api-live.err`
- `worker-live.log`
- `worker-live.err`
- `cloudflared-live.log`

Они игнорируются `.gitignore` и не входят в branch diff. Это правильно: `.env` содержит секреты, а live logs могут содержать runtime details. Если команде нужны логи для ручной передачи, их нужно передавать отдельно и предварительно проверять на секреты.

## Recommended integration order

1. Начать merge из `feature/integration-web-backend`, не из старого `feature/vk-bot`.
2. Добавить Mini App BFF/frontend files из `origin/feature/vk-miniapp`.
3. В `cmd/api/main.go` вручную объединить VK webhook wiring и Mini App wiring.
4. В `config.go` объединить весь config surface: VK bot, DeepInfra/OpenAI, `.env`, Mini App.
5. Оставить нашу VK inbound/delivery реализацию.
6. Перенести billing/storage/migration изменения друга.
7. Объединить docs/audit/runbook/task files вручную.
8. Запустить проверки:

```powershell
go test ./...
go vet ./...
gofmt -l .
docker compose config
cd web/miniapp
npm install
npm run build
```

9. После проверки сделать отдельный интеграционный PR/merge commit.

## Current conclusion

Ветки совместимы по направлению развития: VK bot и Mini App можно объединить в один backend. Главный риск не в frontend, а в том, что ветка Mini App была сделана от более старой backend версии и откатывает значительную часть нашей VK bot логики. Интеграция должна быть ручной: Mini App добавлять поверх нашей текущей bot/backend версии, а не заменять нашу backend часть веткой друга.
