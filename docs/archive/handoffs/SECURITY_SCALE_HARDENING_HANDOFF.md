# Контекст Security/Scale Hardening

Этот файл объясняет, что было сделано в рамках hardening-прохода на ветке
`fastlife_dev`, зачем это было нужно и какие риски остаются. Он предназначен
для человека-ревьюера и для следующего агента.

Файл не содержит секреты, launch params, prompt bodies, provider payloads,
private URLs или raw PII.

## Коротко

В hardening-проходе закрыты риски R1-R10: утечки в логах, публичная нагрузка на
payment redirect, рост process-local памяти, давление upload-пути, retention
сырых payload/text, antispam degraded behavior, прямые provider payment links и
безопасность load-test контура.

Основные архитектурные инварианты сохранены:

- VK handlers, Mini App BFF и `cmd/api` не вызывают AI providers напрямую.
- Billing остается append-only ledger/payment flow.
- Provider/payment/VK secrets и raw sensitive payloads не логируются.
- Пользовательские generation-запросы остаются асинхронными Jobs.
- Retention cleanup сохраняет idempotency, billing, job и audit records.

## Что Изменилось И Что Было Бы Без Этого

| Область | Что изменилось | Что было бы, если не изменить |
|---|---|---|
| Runtime logging | Runtime errors теперь логируются через bounded normalized codes/classes, без raw external details. | Provider/payment/VK ошибки могли бы утащить в логи чувствительные детали или high-cardinality external strings. |
| Payment redirect | Добавлен отдельный public payment redirect handler, app-level limits и nginx edge limits до повторных lookup. | Публичный `/payments/vk/{id}` можно было бы проще сканировать и создавать лишнее DB pressure. |
| VK top-up links | VK bot теперь отдает server-owned redirect link или fail-closed safe message. | При плохой public redirect config бот мог бы показать пользователю прямую provider confirmation link. |
| Rate limiting | Local limiter получил idle TTL, bounded sweep и hard bucket cap. | High-cardinality ключи могли бы постепенно раздувать process memory под публичным трафиком. |
| VK inbound payload retention | Новые VK inbound rows хранят metadata-only payload; legacy raw payloads получают batched expiry/redaction. | Raw callback payloads могли бы жить бессрочно и хранить user text, URLs или другие sensitive details. |
| Mini App uploads | Upload path переведен на streaming multipart parsing, pre-read size checks, concurrency limits и nginx body limit. | Одновременные большие uploads могли бы буферизоваться в памяти и легче выжимать API instance. |
| VK local UI state | Best-effort local active menu/dialog caches получили TTL и peer caps; critical dialog mode остается в Redis с TTL. | Process-local maps могли бы расти без понятного лимита, а stale callbacks вели бы себя менее предсказуемо. |
| Antispam degradation | При dependency errors expensive generation candidates блокируются, cheap control commands продолжают работать, denied/degraded events завершаются без retry amplification. | Antispam outage мог бы fail-open для дорогих операций или усиливать retries во время abuse wave. |
| Load-test safety | Load-test config и k6 scripts по умолчанию отказываются работать с known production hosts и остаются mock-backed. | Generic load tests могли бы случайно попасть в production, real VK delivery, YooKassa или paid providers. |
| Command raw text retention | `commands.raw_text` классифицирован как `user_content`; добавлены retention columns, cleanup и tests для redaction после safe linked jobs. | Нормализованный user command text мог бы храниться бессрочно без TTL/redaction policy. |

## Ключевые Файлы Для Ревью

- `internal/platform/logging/logging.go`
- `internal/adapter/inbound/paymentredirect/handler.go`
- `internal/adapter/inbound/vk/handler.go`
- `internal/adapter/inbound/vk/menu.go`
- `internal/platform/ratelimit/ratelimit.go`
- `internal/adapter/inbound/miniapp/upload.go`
- `internal/service/antispam/service.go`
- `internal/service/maintenance/service.go`
- `internal/adapter/storage/postgres/maintenance.go`
- `migrations/000024_command_raw_text_retention.up.sql`
- `docs/DATA_RETENTION_POLICY.md`
- `docs/LOAD_TESTING.md`
- `docs/DEV_CONTOUR.md`

## Новые Или Измененные Настройки

Example env-файлы обновлены только non-secret knobs:

- `RETENTION_COMMAND_RAW_TEXT_DAYS=30`
- `COMMAND_RETENTION_BATCH_SIZE=500`
- Mini App upload size/concurrency controls.
- Rate limiter capacity/TTL controls.
- Payment redirect public-route limits.
- Load-test guard, например `K6_ALLOW_PRODUCTION_LIVE_SMOKE=false`.

Production secrets должны оставаться только в approved secret stores и runtime
env files. Их нельзя переносить в committed docs или examples.

## Что Уже Проверено

Перед push основного hardening-коммита были выполнены:

- `git diff --check`
- `docker compose config`
- `go test ./...`
- names-only diff secret scan
- temporary/backup/build artifact scan

Полный Go suite прошел после unsandboxed rerun: локальный sandbox блокировал Go
build cache и `httptest` localhost listeners. Это было ограничение окружения, а
не ошибка тестов.

После установки `k6` был выполнен безопасный local/mock micro-smoke на
`127.0.0.1:18080` с `APP_ENV=loadtest`, mock providers, mock payments и без
real VK delivery:

- `k6 inspect` прошел для всех checked-in scripts.
- `basic-api` прошел: health/readiness, VK webhook, Mini App balance/job create.
- `vk-bot` прошел: synthetic webhook journey, duplicate replay и маленький
  rate-limit burst.
- `job-worker` прошел в text-only mock mode: jobs создавались и доходили до
  terminal status.
- `billing-payments` прошел в mock mode: intent create, idempotent replay,
  mock top-up и refund.
- Redis DLQ после micro-smoke остался `0`.

Во время запуска найден и исправлен k6 v2 env-name conflict: script-level
duration для `basic-api` теперь называется `K6_BASIC_DURATION`, потому что
process env `K6_DURATION` воспринимается самим k6 как global option и ломает
custom scenarios.

## Rollback Notes

Основной hardening-коммит:

```text
80d8337 security: harden public and retention surfaces
```

Follow-up commit с handoff-файлом:

```text
df5a4c0 docs: add security hardening handoff
```

Варианты rollback:

- До применения DB migrations достаточно code-only revert основного commit.
- После применения `000024_command_raw_text_retention` schema rollback требует
  ручного запуска down migration в контролируемом окружении.
- После command/VK payload redaction восстановить raw text/payloads можно только
  из DB backup.
- При расследовании retention behavior сначала остановить worker maintenance
  loop, затем смотреть данные или откатывать поведение.
- Production schema rollback не должен выполняться автоматически вместе с
  application rollback.

## Остаточные Риски И Follow-Ups

Это не reopened R1-R10, но это важные пункты до широкого публичного production
traffic:

1. Production webhook routing нужно подтвердить: YooKassa route должен идти в
   `cmd/provider-webhook` / `PAYMENT_WEBHOOK_ADDR`, а не в `cmd/api`.
2. Live YooKassa smoke еще нужен для `payment.succeeded`, `payment.canceled` и
   `refund.succeeded` через dashboard-delivered webhooks.
3. Production deployment shape нужно довести и проверить: static Mini App,
   `cmd/api`, `cmd/worker`, dedicated payment webhook runtime, TLS/proxy
   headers и service units.
4. Credential-bound smoke для real OpenAI, DeepInfra и VK delivery нужен до
   внешних пользователей.
5. Edge/proxy body-size limits нужно проверить на реальном public path перед
   публичными reference uploads.
6. Для automatic, partial или already-spent credit refunds нужна lot/FIFO
   attribution.
7. Local rate limiter теперь memory-bounded, но cross-instance fairness все еще
   зависит от edge/shared rate limiting strategy.
8. Payment redirect защищен route/edge limits; opaque unguessable redirect
   tokens остаются возможным future hardening.
9. k6 scripts защищены от production по умолчанию, и local/mock micro-smoke уже
   прошел. Полный load/report run, Postgres/Redis diagnostics и optional
   provider-webhook replay еще не выполнялись.

## Стартовая Точка Для Следующего Агента

Следующий агент должен читать в таком порядке:

```text
AGENTS.md
.agents/state.json
docs/SECURITY_SCALE_HARDENING_HANDOFF.md
docs/DATA_RETENTION_POLICY.md
docs/LOAD_TESTING.md
docs/DEV_CONTOUR.md
```

Не читать `.agents/logs/**` по умолчанию. Читать эти логи только для повторной
known-error debugging или если пользователь явно попросит историю.
