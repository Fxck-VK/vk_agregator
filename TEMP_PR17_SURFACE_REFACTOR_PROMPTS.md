# TEMP - PR-17 App Surface Refactor Prompts

Use this file as a copy-paste queue for Codex prompts. Send one PR prompt at a
time. Do not start the next PR until the previous one is committed, pushed and
green.

Current intended target branch: `feature/integration-web-backend`.

Core idea:

- Backend core remains the source of truth.
- VK text bot and Mini App are app surfaces above the same backend core.
- No direct provider calls from VK inbound or Mini App BFF.
- No frontend/client-side billing, balance mutation, moderation decisions,
  provider choice or trusted job status.

---

## PR-17.1 Prompt - Architecture ADR / Refactor Plan

```text
MODE: IMPLEMENT (docs only)

Task:
PR-17.1: зафиксировать план архитектурного refactor для разделения app surfaces:
VK text bot и Mini App поверх общего backend core. Код не менять.

Target branch: feature/integration-web-backend.

Language:
Отвечай на русском. Имена файлов, функций, env, команды, commit message — English.

Context:
После merge fastlife_dev в feature/integration-web-backend в ветке есть оба входа:
VK text bot и VK Mini App. Сейчас wiring частично сосредоточен в cmd/api/main.go.
Нужно сначала зафиксировать архитектурное решение, чтобы дальше делать маленькие
PR без изменения поведения.

Goal:
Есть ADR и backlog, где описано:
- backend core = source of truth;
- VK bot и Mini App = surfaces поверх core;
- целевые папки internal/app/vkbot и internal/app/miniapp;
- что нельзя переносить в surface modules.

Allowed scope:
- DECISIONS.md
- PROGRESS.md
- TASKS.md
- AUDIT.md (только если нужно зафиксировать риск)
- TEMP_PR17_SURFACE_REFACTOR_PROMPTS.md (только если нужно уточнить план)

Safety / architecture:
- Не менять runtime code.
- Не трогать backend behavior, frontend, env, migrations.
- Не ослаблять auth, idempotency, billing, rate limiting, moderation, ownership.
- Не коммитить .env / .env.ps1 / secrets.
- Provider calls остаются только через worker/job flow.

Steps:
1. Прочитай AGENTS.md, последние записи PROGRESS.md и DECISIONS.md.
2. Добавь ADR в DECISIONS.md:
   - VK bot и Mini App как app surfaces.
   - backend core: domain, service, worker, provider adapters, storage.
   - cmd/api/main.go должен стать thin bootstrap.
   - app modules only wire handlers/deps, не владеют billing/provider logic.
3. Обнови TASKS.md планом PR-17.2-17.5.
4. Обнови PROGRESS.md краткой записью PR-17.1.
5. Checks:
   - docs-only, build можно не запускать; если не запускаешь — явно скажи почему.
6. Если green:
   git add DECISIONS.md PROGRESS.md TASKS.md AUDIT.md TEMP_PR17_SURFACE_REFACTOR_PROMPTS.md (только реально изменённые)
   git commit -m "docs: plan app surface refactor"
   git push origin feature/integration-web-backend
7. git rev-parse --short HEAD

Acceptance criteria:
- [ ] ADR описывает app surfaces и backend core.
- [ ] TASKS.md содержит план PR-17.2-17.5.
- [ ] Runtime code не менялся.
- [ ] Секреты не затронуты.
- [ ] Commit/push в feature/integration-web-backend.

Final response format:
1. PR-17.1 status: completed / blocked
2. Что зафиксировано в ADR
3. Checks
4. Commit/push + SHA
5. Notes
```

---

## PR-17.2 Prompt - Extract VK Bot Module Wiring

```text
MODE: IMPLEMENT

Task:
PR-17.2: вынести wiring VK text bot из cmd/api/main.go в отдельный app surface
module internal/app/vkbot. Поведение не менять.

Target branch: feature/integration-web-backend.

Language:
Отвечай на русском. Имена файлов, функций, env, команды, commit message — English.

Goal:
cmd/api/main.go перестаёт знать детали сборки VK bot. Он только создаёт shared
repos/services и монтирует handler, который возвращает vkbot module.

Allowed scope:
- cmd/api/main.go
- internal/app/vkbot/**
- internal/adapter/inbound/vk/** (только если нужен экспорт/тип для wiring, без изменения поведения)
- PROGRESS.md
- AUDIT.md / TASKS.md / DECISIONS.md (только если нужно зафиксировать follow-up)

Do NOT move:
- internal/service/joborchestrator
- internal/service/billingservice
- internal/worker
- internal/adapter/provider
- internal/domain
- internal/adapter/storage

Safety / architecture:
- VK inbound handler не вызывает provider напрямую.
- VK bot создаёт Jobs через joborchestrator.
- Billing/referral rewards только через billingservice ledger methods.
- VK callback/menu/dialog/anti-spam/referral behavior не менять.
- Не менять env names.
- Не коммитить .env / secrets.

STEP 0 - разведка без кода:
1. Найди в cmd/api/main.go текущий VK bot wiring:
   - vkdelivery.ControlClient / UserProfileClient
   - dialogstate
   - antispam
   - referralservice
   - vkinbound.NewHandler
   - vkMenuFeatures
2. Доложи кратко, что будет перенесено, какие deps останутся shared.

Implementation:
1. Создай internal/app/vkbot/module.go.
2. Вынеси туда тип Config/Deps или Module factory так, чтобы cmd/api/main.go
   передавал shared deps и получал http.Handler.
3. Перенеси vkMenuFeatures в vkbot module, если это уменьшает cmd/api.
4. cmd/api/main.go должен сохранить routes:
   - /webhooks/vk
   - /admin/
   - /miniapp/
   - /metrics
   - /health
   - /healthz
5. Не менять публичные тексты/кнопки VK bot.
6. Обнови PROGRESS.md.

Checks:
- gofmt -w <changed Go files>
- go test ./internal/adapter/inbound/vk ./internal/service/antispam ./internal/service/dialogstate ./internal/service/referralservice
- go test ./...
- go build ./...

If green:
git add <only PR-17.2 files explicitly>
git commit -m "refactor: extract vk bot api module"
git push origin feature/integration-web-backend
git rev-parse --short HEAD

Acceptance criteria:
- [ ] VK bot wiring moved to internal/app/vkbot.
- [ ] cmd/api/main.go thinner, but all routes remain.
- [ ] VK bot behavior unchanged.
- [ ] Provider only through job/worker flow.
- [ ] Billing append-only unchanged.
- [ ] go test ./... and go build ./... exit 0.

Final response format:
1. PR-17.2 status: completed / blocked
2. STEP 0 summary
3. What moved
4. Checks
5. Commit/push + SHA
6. Architecture notes
```

---

## PR-17.3 Prompt - Extract Mini App Module Wiring

```text
MODE: IMPLEMENT

Task:
PR-17.3: вынести wiring Mini App BFF из cmd/api/main.go в отдельный app surface
module internal/app/miniapp. Поведение и BFF contracts не менять.

Target branch: feature/integration-web-backend.

Language:
Отвечай на русском. Имена файлов, функций, env, команды, commit message — English.

Goal:
cmd/api/main.go перестаёт знать детали сборки Mini App BFF. Mini App остается
тонкой surface над общим backend core.

Allowed scope:
- cmd/api/main.go
- internal/app/miniapp/**
- internal/adapter/inbound/miniapp/** (только если нужен экспорт/тип для wiring, без изменения contracts)
- PROGRESS.md
- AUDIT.md / TASKS.md / DECISIONS.md (только если нужно)

Do NOT change:
- web/miniapp/**
- endpoint contracts
- launch params auth
- estimate/job/chat request/response DTO semantics
- billing/provider/worker logic

Safety / architecture:
- Mini App не вызывает provider напрямую.
- Price/balance/status остаются backend-owned.
- Auth through VK launch params remains backend-verified.
- Estimate does not create jobs, reserve credits, call provider or write ledger.
- Artifact URLs only through backend artifact route.
- No raw launch params/prompts/PII in logs.

STEP 0 - разведка без кода:
1. Найди текущий Mini App wiring в cmd/api/main.go:
   - miniappapi.NewHandler
   - ratelimit.New for Mini App job/estimate/chat
   - object store deps
   - users/jobs/artifacts/moderation/billing/orchestrator deps
2. Доложи кратко, что переносится, какие deps остаются shared.

Implementation:
1. Создай internal/app/miniapp/module.go.
2. Вынеси туда factory для Mini App handler.
3. cmd/api/main.go должен только получить handler и mount `/miniapp/`.
4. Убедись, что internal/adapter/inbound/miniapp Routes still expose:
   - POST /miniapp/estimate
   - POST /miniapp/chat/messages
   - POST /miniapp/jobs
   - GET /miniapp/jobs
   - GET /miniapp/jobs/{id}
   - GET /miniapp/balance
   - GET /miniapp/artifacts/{id}
5. Обнови PROGRESS.md.

Checks:
- gofmt -w <changed Go files>
- go test ./internal/adapter/inbound/miniapp
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build

If green:
git add <only PR-17.3 files explicitly>
git commit -m "refactor: extract miniapp api module"
git push origin feature/integration-web-backend
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Mini App wiring moved to internal/app/miniapp.
- [ ] BFF endpoint contracts unchanged.
- [ ] Auth/rate limiting preserved.
- [ ] Provider only through job/worker flow.
- [ ] npm build, go test, go build green.

Final response format:
1. PR-17.3 status: completed / blocked
2. STEP 0 summary
3. What moved
4. Endpoint preservation
5. Checks
6. Commit/push + SHA
7. Architecture notes
```

---

## PR-17.4 Prompt - Thin cmd/api Bootstrap

```text
MODE: IMPLEMENT

Task:
PR-17.4: сделать cmd/api/main.go тонким bootstrap-файлом после выноса vkbot и
miniapp modules. Поведение не менять.

Target branch: feature/integration-web-backend.

Language:
Отвечай на русском. Имена файлов, функций, env, команды, commit message — English.

Goal:
cmd/api/main.go содержит только:
- config load/validate
- tracing
- db/redis/s3 init
- shared repos/services
- app modules wiring calls
- route mounting
- health/admin/metrics
- graceful shutdown

Allowed scope:
- cmd/api/main.go
- internal/app/**
- PROGRESS.md
- AUDIT.md / DECISIONS.md / TASKS.md (only if needed)

Safety / architecture:
- No behavior changes.
- No endpoint removal.
- No env rename.
- No provider calls from app surfaces.
- No billing mutation outside billingservice.
- No .env/secrets.

STEP 0 - без кода:
1. Сравни cmd/api/main.go после PR-17.2/17.3.
2. Перечисли оставшийся wiring noise и что можно безопасно упростить.

Implementation:
1. Убери из cmd/api/main.go детали, которые теперь принадлежат app modules.
2. Сохрани shared repos/services в одном месте.
3. Сохрани final route map:
   - /webhooks/vk
   - /admin/
   - /miniapp/
   - /metrics
   - /health
   - /healthz
4. Не переносить health/admin unless это явно нужно.
5. Обнови PROGRESS.md.

Checks:
- gofmt -w <changed Go files>
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build
- git grep conflict markers
- git diff --check

If green:
git add <only PR-17.4 files explicitly>
git commit -m "refactor: simplify api bootstrap wiring"
git push origin feature/integration-web-backend
git rev-parse --short HEAD

Acceptance criteria:
- [ ] cmd/api/main.go visibly thinner.
- [ ] Routes unchanged.
- [ ] VK bot and Mini App behavior unchanged.
- [ ] go test/build + Mini App build green.

Final response format:
1. PR-17.4 status: completed / blocked
2. STEP 0 summary
3. What simplified
4. Route map
5. Checks
6. Commit/push + SHA
7. Notes
```

---

## PR-17.5 Prompt - Architecture Docs / Runbook Update

```text
MODE: IMPLEMENT (docs + verification)

Task:
PR-17.5: обновить документацию после refactor app surfaces. Зафиксировать, где
живёт VK bot, где Mini App, где backend core, и как добавлять новые features.

Target branch: feature/integration-web-backend.

Language:
Отвечай на русском. Имена файлов, функций, env, команды, commit message — English.

Goal:
Новый агент должен быстро понять:
- VK bot и Mini App — surfaces.
- backend core — source of truth.
- cmd/api/main.go — bootstrap.
- provider/billing/job/worker logic не принадлежит surfaces.

Allowed scope:
- ARCHITECTURE.md
- RUNBOOK.md
- README.md (если краткая ссылка нужна)
- PROGRESS.md
- TASKS.md
- DECISIONS.md
- MERGE_HANDOFF.md / MERGE_CHECKLIST.md (если нужно)
- docs/**

Safety:
- Runtime code не менять.
- Не коммитить .env/secrets/logs.
- Не выдумывать endpoints; документировать только фактическое состояние.

Steps:
1. Прочитай актуальные internal/app/vkbot и internal/app/miniapp modules.
2. Обнови ARCHITECTURE.md:
   - app surfaces
   - backend core
   - request flow diagrams/text
   - forbidden shortcuts
3. Обнови RUNBOOK.md:
   - где запускать API/worker
   - где искать VK bot wiring
   - где искать Mini App BFF wiring
   - smoke checklist for both entrances
4. Обнови PROGRESS.md.
5. Если появились follow-ups, обнови TASKS.md.

Checks:
- go test ./...
- go build ./...
- npm --prefix web/miniapp run build

If green:
git add <only PR-17.5 files explicitly>
git commit -m "docs: document app surface architecture"
git push origin feature/integration-web-backend
git rev-parse --short HEAD

Acceptance criteria:
- [ ] Docs explain app surfaces vs backend core.
- [ ] Docs say where to add VK bot command.
- [ ] Docs say where to add Mini App endpoint.
- [ ] Docs preserve invariants.
- [ ] Checks green.

Final response format:
1. PR-17.5 status: completed / blocked
2. Docs updated
3. Architecture summary
4. Checks
5. Commit/push + SHA
6. Notes
```
