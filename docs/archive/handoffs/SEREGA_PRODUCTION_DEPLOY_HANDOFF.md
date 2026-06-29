# Merge Handoff: serega Production Deploy Automation

Дата: 2026-06-15

Назначение файла: быстро ввести второго агента в контекст изменений ветки `serega` перед merge. После завершения merge файл можно удалить, если он не нужен как историческая заметка.

## Source Context

- Рабочая ветка: `serega`
- Последний важный коммит: `4f69129 deploy: automate production VPS rollout`
- Цель изменений: довести production deployment до повторяемого VPS rollout вместо ручной настройки.
- Секреты намеренно не указаны. Не переносить в git `.env`, `.runtime`, токены, пароли, Cloudflare/YooKassa/VK/DeepInfra ключи.

## Что Было Сделано

### Production Docker / Deploy

- Добавлен GitHub Actions workflow для сборки Docker images в GHCR:
  - `api`
  - `worker`
  - `provider-webhook`
  - `miniapp`
  - `migrate`
  - `backup`
- Обновлен `docker-compose.prod.yml` под production runtime.
- Deploy-скрипты теперь поддерживают два режима:
  - основной быстрый путь: `docker compose pull && docker compose up -d`;
  - fallback для первого тестового VPS: сборка на VPS через `--build-on-vps`.
- Добавлены/обновлены скрипты:
  - `scripts/deploy/deploy-prod.sh`
  - `scripts/deploy/deploy-prod.ps1`
  - `scripts/deploy/rollback-prod.sh`
  - `scripts/deploy/rollback-prod.ps1`
  - `scripts/deploy/smoke-prod.sh`
  - `scripts/deploy/smoke-prod.ps1`
  - `scripts/deploy/check-prod-env.sh`
  - `scripts/deploy/check-prod-env.ps1`

### Env / Production Validation

- Обновлен `.env.prod.example`.
- Добавлен `.env.staging.example`.
- Production validation оставлена fail-closed.
- `ARTIFACT_SCANNER=none` в production теперь разрешен только при явном флаге:
  - `ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true`
- Production example больше не включает выключенную генерацию изображений как дефолтный конфликтный режим.
- Deploy/rollback-скрипты больше не пытаются логиниться в GHCR, если `GHCR_USERNAME` / `GHCR_TOKEN` пустые или являются placeholder-значениями (`CHANGE_ME`, `placeholder`, `example`).

### Cloudflare / Reverse Proxy

- Добавлена документация по Cloudflare routes:
  - `deployments/cloudflare/README.md`
- Обновлен пример cloudflared config:
  - `deployments/cloudflare/cloudflared.prod.example.yml`
- Ожидаемая схема:
  - `vk.neiirohub.ru/webhooks/vk` -> `cmd/api:8080`
  - `app.neiirohub.ru` -> Mini App frontend
  - `app.neiirohub.ru/miniapp/*` -> `cmd/api:8080`
  - `neiirohub.ru/billing/webhooks/yookassa` -> `cmd/provider-webhook:8082`
- Наружу не должны торчать:
  - `/admin/*`
  - `/metrics`
  - `/debug/*`
  - широкие `/billing/*`, кроме точного YooKassa webhook route.

### CI / Infra Validation

- Расширен `scripts/ci/validate-infra.ps1`.
- Обновлен `web/miniapp/package-lock.json`, чтобы CI/deploy могли воспроизводимо ставить зависимости.
- Обновлен `.gitleaks.toml`, чтобы placeholder-значения не считались реальными секретами.
- `.env` и runtime-артефакты остаются игнорируемыми.

## VPS Deploy Smoke

Тестовый VPS был использован только для проверки автоматизации. Доступы в этот файл не заносились.

Что подтвердилось:

- Docker и compose ставятся автоматически.
- Проект можно загрузить на чистый VPS.
- Первый deploy может собирать образы прямо на VPS через `--build-on-vps`.
- После исправления серверного `.env` повторный deploy занял около 21 секунды.
- На VPS поднялись и стали healthy:
  - `api`
  - `worker`
  - `provider-webhook`
  - `miniapp`
  - `reverse-proxy`
  - `postgres`
  - `redis`
  - `minio`
- Локальные health checks на VPS вернули 200:
  - reverse proxy health
  - API health
  - provider-webhook health
  - worker health
  - Mini App frontend

Что не было включено:

- Публичный Cloudflare tunnel, потому что на сервере не было настоящего `CLOUDFLARED_TUNNEL_TOKEN`.
- Публичные VK/YooKassa smoke через домены после переустановки сервера.

## Важные Production Env Детали

Для Docker production на VPS должны использоваться service names, а не локальные `localhost`:

```text
DATABASE_URL=postgres://...@postgres:5432/...
REDIS_ADDR=redis:6379
S3_ENDPOINT=minio:9000
S3_PUBLIC_ENDPOINT=
S3_USE_SSL=false
```

Cloudflare token должен жить только в серверном `.env`:

```text
CLOUDFLARED_TUNNEL_TOKEN=...
```

Не коммитить его и не переносить в docs.

## Hot Zones Для Merge

Особо внимательно смотреть конфликты в:

- `.github/workflows/docker-images.yml`
- `docker-compose.prod.yml`
- `.env.prod.example`
- `.env.staging.example`
- `.gitignore`
- `.gitleaks.toml`
- `RUNBOOK.md`
- `internal/platform/config/config.go`
- `internal/platform/config/config_test.go`
- `scripts/ci/validate-infra.ps1`
- `scripts/deploy/*.sh`
- `scripts/deploy/*.ps1`
- `deployments/cloudflare/*`
- `web/miniapp/package-lock.json`

## Инварианты, Которые Нельзя Сломать

- VK Bot, Mini App и `cmd/api` не вызывают AI providers напрямую.
- Provider calls идут только через worker/job flow.
- Billing остается ledger-based: никаких direct balance mutation.
- YooKassa top-up credits начисляются только после provider-verified webhook/reconciliation.
- Payment redirect/confirmation URL не является доказательством оплаты.
- Artifact access должен оставаться owner-checked.
- `/admin/*`, `/metrics`, observability, private health/debug endpoints не должны становиться публичными.
- Secrets, auth headers, tokens, launch params, raw provider payloads, raw PII, private artifact URLs не логировать и не коммитить.
- Если сохраняется production scanner bypass, он должен быть только явным флагом `ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true`, а не неявным дефолтом.

## Проверки, Которые Уже Выполнялись Перед Пушем `4f69129`

```text
gofmt check
go test ./...
npm --prefix web/miniapp run build
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1
git diff --check
gitleaks git --staged --config .gitleaks.toml --redact --no-banner --no-color --max-target-megabytes 5
```

## Что Проверить После Merge

1. Запустить быстрые проверки:

```text
gofmt -l .
go test ./...
npm --prefix web/miniapp run build
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1
git diff --check
```

2. Проверить, что `.env` не появился в git:

```text
git status --short
git ls-files .env .env.* _env
```

3. Проверить GitHub Actions:

```text
Backend
Mini App
Infrastructure
Secret Scan
Docker Images
```

4. После настройки `CLOUDFLARED_TUNNEL_TOKEN` на VPS запустить:

```text
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env --with-cloudflare
bash scripts/deploy/smoke-prod.sh --env-file .env
```

5. Публичный smoke после DNS/Cloudflare:

```text
https://vk.neiirohub.ru/health
https://app.neiirohub.ru
https://neiirohub.ru/billing/webhooks/yookassa
```

6. Проверить, что закрыты снаружи:

```text
/admin/*
/metrics
/debug/*
```

## Merge Advice

Если будут конфликты, сохранять обе production-ветки additive:

- не удалять чужие deploy/runtime улучшения;
- не откатывать CI quality gates;
- не возвращать секреты в tracked files;
- не заменять service names (`postgres`, `redis`, `minio`) на `localhost` в production Docker env;
- не превращать Cloudflare tunnel token в tracked config.

Если конфликт затрагивает billing, worker, provider boundaries или auth/security, лучше остановиться и отдельно разобрать файл, чем решать автоматически.
