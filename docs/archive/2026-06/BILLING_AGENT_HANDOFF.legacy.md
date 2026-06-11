# Billing Agent Handoff

Дата: 2026-06-09
Ветка, на которой готовился контекст: `serega`

Этот файл нужен агенту коллеги, чтобы продолжить главы по биллингу без потери контекста и без нарушения логики VK Bot / VK Mini App.

## Что читать сначала

1. `AGENTS.md`
2. `.agents/state.json`
3. Этот файл
4. Только при необходимости по scope:
   - `RUNBOOK.md`
   - `TASKS.md`
   - `docs/ARCHITECTURE.md`

Не читать `docs/archive/**` и `.agents/logs/**` как актуальный контекст, если задача не про исторический разбор.

## Главные инварианты

- VK Bot и VK Mini App не начисляют кредиты напрямую.
- Frontend, redirect после оплаты и пользовательский callback не являются доказательством оплаты.
- Кредиты начисляются только через provider-verified path:
  `payment_events` -> `GetPayment` у провайдера -> state machine -> `billingservice.GrantWith`.
- Баланс меняется только через ledger.
- `/billing/*` operator endpoints должны fail-closed без `ADMIN_TOKEN`.
- В публичные/operator DTO нельзя отдавать raw YooKassa payload, auth headers, secrets, launch params или PII.
- YooKassa HTTP детали должны оставаться внутри `internal/adapter/payment/yookassa`.
- Внутренний ledger idempotency key и YooKassa HTTP `Idempotence-Key` не смешивать.
- Поздний `payment.canceled` не должен откатывать уже `succeeded` intent.
- Refund MVP сейчас только ручной, полный, operator-only. Не возвращать уже потраченные кредиты до lot/FIFO учета.

## Что уже реализовано

### Payment domain and storage

- `internal/domain/payment.go`
  - `PaymentProduct`
  - `PaymentIntent`
  - `PaymentEvent`
  - `PaymentRefund`
  - explicit payment intent state machine
  - provider-normalized DTOs/interfaces
- `internal/domain/repositories.go`
  - payment repository contracts
  - intent/event filters for operator lists
- `internal/adapter/storage/postgres/payment.go`
- `internal/adapter/storage/memory/payment.go`

Migrations:

- `migrations/000009_payments.*`
- `migrations/000010_payment_product_catalog.*`
- `migrations/000011_payment_intent_receipt_snapshot.*`

Important: payment intents snapshot amount, credits, price version and 54-FZ receipt fields. Do not recalculate old intents from the current product catalog.

### Billing ledger

- `internal/service/billingservice`
- `GrantWith(ctx, repo, ...)` exists for tx-aware topups.
- Payment topups must use `LedgerTopup`.
- Duplicate topup webhook/reconciliation must be a no-op, not a second grant.

### Provider layer

- `domain.PaymentProvider`
- `internal/adapter/payment/mock`
- `internal/adapter/payment/yookassa`
- `internal/adapter/payment` factory by `PAYMENT_PROVIDER`

YooKassa adapter handles:

- Basic Auth inside adapter only
- kopecks <-> `"100.00"` conversion
- short provider idempotence headers
- `capture: true`
- receipt payload for 54-FZ
- payment creation/get/cancel/refund
- webhook normalization

### Payment service

- `internal/service/paymentservice/service.go`
  - creates payment intents
  - reuses active waiting intent unless `ForceNew`
  - requires receipt email or phone
  - returns safe domain data, not provider-native payloads
- `internal/service/paymentservice/webhook_processor.go`
  - accepts parsed webhook events
  - writes/reads `payment_events`
  - verifies provider state through `GetPayment`
  - applies state machine
  - grants credits through `GrantWith`
  - reconciles stale intents
  - handles manual full refund MVP

### Runtime

- `cmd/api`
  - main API, VK Bot, Mini App BFF, protected operator endpoints
- `cmd/provider-webhook`
  - dedicated payment webhook intake/runtime
  - exposes YooKassa webhook endpoint
  - processes webhook inbox async
  - runs reconciliation loop
  - has readiness/metrics endpoints

Dev scripts:

- `scripts/dev/start-payments.ps1`
- `scripts/dev/stop-payments.ps1`
- `scripts/dev/status-payments.ps1`
- `scripts/dev/_payments-common.ps1`

### Current endpoints

Protected operator routes under `cmd/api`:

```text
GET  /billing/payment-products
POST /billing/payment-products
GET  /billing/payment-products/{id}
PATCH /billing/payment-products/{id}
POST /billing/payment-products/{id}/disable
POST /billing/payment-intents
GET  /billing/payment-intents/{id}
GET  /billing/payment-history
POST /billing/payment-intents/{id}/sync
POST /billing/payment-intents/{id}/refund
GET  /billing/payment-intents/pending
GET  /billing/payment-events/unprocessed
```

All require admin auth. Do not expose raw provider payloads here.

Mini App routes under `cmd/api`:

```text
GET  /miniapp/payment-products
POST /miniapp/payments/intents
GET  /miniapp/payments
GET  /miniapp/payments/{id}
```

These must use trusted Mini App auth. Do not trust `user_id` from the JSON body.

Payment webhook runtime:

```text
POST /billing/webhooks/yookassa
GET  /health
GET  /healthz
GET  /readyz
GET  /metrics
```

Production webhook must be HTTPS or behind trusted reverse proxy headers.

## Недавняя глава 4: Operator Tools

Уже добавлено:

- `GET /billing/payment-events/unprocessed`
- `GET /billing/payment-intents/pending`
- safe DTOs without `PaymentEvent.Payload`
- stale intent listing via `stale_after` / `stale_only`
- repository/service list methods for payment events and pending intents
- tests for operator lists and payload non-leakage
- runbook examples

Useful examples:

```bash
# ADMIN_AUTH_HEADER carries X-Admin-Token; OPERATOR_IDEMPOTENCY_HEADER carries
# a unique X-Idempotency-Key for the refund command.

curl "http://localhost:8080/billing/payment-intents/pending?stale_after=30s&stale_only=true" \
  -H "$ADMIN_AUTH_HEADER"

curl "http://localhost:8080/billing/payment-events/unprocessed?provider=yookassa" \
  -H "$ADMIN_AUTH_HEADER"

curl -X POST "http://localhost:8080/billing/payment-intents/<intent_id>/sync" \
  -H "$ADMIN_AUTH_HEADER"

curl -X POST "http://localhost:8080/billing/payment-intents/<intent_id>/refund" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "$OPERATOR_IDEMPOTENCY_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"reason":"manual operator refund"}'
```

## Недавняя глава 5: Product Catalog Admin

Уже добавлено:

- protected operator endpoints для `payment_products`:
  - `GET /billing/payment-products`
  - `POST /billing/payment-products`
  - `GET /billing/payment-products/{id}`
  - `PATCH /billing/payment-products/{id}`
  - `POST /billing/payment-products/{id}/disable`
- repository/service methods for list/create/update/disable catalog rows;
- safe operator DTO без provider payloads/secrets;
- validation for positive RUB packages, product code format and 54-FZ receipt
  fields;
- default 54-FZ values for new products: `vat_code=1`,
  `payment_subject=service`, `payment_mode=full_prepayment`;
- automatic `price_version` bump for future snapshots when title, amount,
  credits, currency, VAT, payment subject or payment mode changes;
- old `payment_intents` remain immutable because they already snapshot amount,
  credits, price version and receipt fields;
- constant-time `X-Admin-Token` comparison on operator/admin surfaces;
- tests for CRUD, active/inactive lists and snapshot behavior.

Useful examples:

```bash
# ADMIN_AUTH_HEADER carries X-Admin-Token for local operator examples.

curl "http://localhost:8080/billing/payment-products?active=true" \
  -H "$ADMIN_AUTH_HEADER"

curl -X POST "http://localhost:8080/billing/payment-products" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"code":"credits_250","title":"NeiroHub 250 credits","amount":20000,"currency":"rub","credits":250,"vat_code":1,"payment_subject":"service","payment_mode":"full_prepayment"}'

curl -X PATCH "http://localhost:8080/billing/payment-products/<product_id>" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"amount":21000,"credits":260}'

curl -X POST "http://localhost:8080/billing/payment-products/<product_id>/disable" \
  -H "$ADMIN_AUTH_HEADER"
```

## Недавняя глава 6: Mini App Payment History UI

Уже добавлено:

- Mini App Settings/Profile loads authenticated `GET /miniapp/payments`;
- safe payment history UI shows status, amount, credits and creation date;
- active `waiting_for_user` intents show a continuation action using only the
  safe `confirmation_url`;
- pending-payment UX keeps "continue payment" and explicit "create new payment"
  paths;
- user-facing copy says the balance updates after payment through backend state;
- provider raw payloads, provider payment IDs and user IDs are not rendered;
- redirects remain navigation-only and do not grant balance.

## Что осталось доделать

### Глава 7. Live YooKassa Smoke

Цель: доказать, что тестовый магазин YooKassa работает end-to-end.

Текущий статус на 2026-06-10: частичный live smoke пройден на локальном
`_env` с тестовым магазином YooKassa. Успешный test-card checkout завершился,
публичный webhook в локальный `cmd/provider-webhook` не пришел и был
подхвачен provider-verified reconciliation без повторного top-up. Replay
`payment.succeeded` дедуплицировался, manual full refund через защищенный
operator endpoint дважды с одним `X-Idempotency-Key` вернул один refund и один
ledger debit, replay `refund.succeeded` дедуплицировался по
`provider_refund_id`. Mini App history отдает safe DTO без provider-native
payload. Не считать smoke полностью закрытым до проверки публичного HTTPS
webhook из кабинета YooKassa и сценария, где provider действительно переводит
payment в `canceled`, а не оставляет checkout в `pending` retry-state.

Перед smoke:

- реальные/test credentials только в local `.env`;
- не коммитить `.env`;
- проверить, что `PAYMENT_PROVIDER=yookassa`;
- проверить `YOOKASSA_RETURN_URL`;
- проверить webhook в кабинете YooKassa:

```text
https://neiirohub.ru/billing/webhooks/yookassa
```

events:

```text
payment.succeeded
payment.canceled
refund.succeeded
```

Smoke checklist:

1. Start API.
2. Start `cmd/provider-webhook`.
3. Create payment intent from Mini App or operator route.
4. Open `confirmation_url`.
5. Complete test payment.
6. Confirm YooKassa webhook reaches `payment_events`.
7. Confirm async processor marks `processed_at`.
8. Confirm user receives one ledger `topup`.
9. Replay webhook and confirm no duplicate topup.
10. Test canceled payment.
11. Test manual refund with same `X-Idempotency-Key` twice.
12. Confirm refund is idempotent.
13. Confirm reconciliation handles stale intent when webhook is missed.

### Глава 8. Lot/FIFO Refund Attribution

Это не MVP, но нужно перед настоящими автоматическими/partial refunds.

Сделать только если явно согласовано:

- lot/FIFO attribution for topup credits;
- track which topup credits were spent;
- allow partial refunds only for unspent credited lots;
- define automatic refund webhook policy.

Пока этого нет:

- automatic refund balance reversal не делать;
- partial refund не делать;
- full manual refund remains conservative and refuses spent credits.

### Глава 9. Production Rollout Checks

Перед реальными платежами:

- verify admin endpoints fail closed;
- verify no secrets in logs/docs/git;
- verify webhook HTTPS guard behind Cloudflare/nginx;
- verify metrics:
  - `payments_created_total`
  - `payments_succeeded_total`
  - `payments_canceled_total`
  - `payment_webhooks_total`
  - `payment_webhook_unprocessed_events`
  - `payment_webhook_oldest_unprocessed_age_seconds`
  - `payment_provider_errors_total`
  - `payment_refunds_total`
  - `payment_reconciliation_mismatches`
- verify SQL checks from `RUNBOOK.md`;
- verify backup/rollback plan for migrations.

## Env names

Do not put actual values into git or docs.

```text
PAYMENT_PROVIDER
YOOKASSA_SHOP_ID
YOOKASSA_SECRET_KEY
YOOKASSA_BASE_URL
YOOKASSA_RETURN_URL
YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED
PAYMENT_WEBHOOK_REQUIRE_HTTPS
PAYMENT_WEBHOOK_ADDR
PAYMENT_WEBHOOK_POLL_INTERVAL
PAYMENT_WEBHOOK_BATCH_LIMIT
PAYMENT_RECONCILIATION_INTERVAL
PAYMENT_RECONCILIATION_LIMIT
PAYMENT_RECONCILIATION_STALE_AFTER
ADMIN_TOKEN
```

## Checks to run

For backend billing work:

```bash
gofmt -w <changed-go-files>
go test ./internal/adapter/inbound/billing ./internal/adapter/inbound/miniapp ./internal/service/paymentservice ./internal/adapter/payment/yookassa ./internal/adapter/storage/memory ./internal/adapter/storage/postgres
go test ./...
go vet ./...
git diff --check
```

For Mini App UI work:

```bash
npm --prefix web/miniapp run build
```

If a test fails outside trivial formatting or fixture drift, stop and report before pushing.

## Files likely to touch next

Product catalog admin:

```text
internal/adapter/inbound/billing/handler.go
internal/adapter/inbound/billing/handler_test.go
internal/service/paymentservice/service.go
internal/domain/repositories.go
internal/adapter/storage/postgres/payment.go
internal/adapter/storage/memory/payment.go
migrations/*
RUNBOOK.md
TASKS.md
docs/ARCHITECTURE.md
```

Mini App payment history UI:

```text
web/miniapp/src/api/client.ts
web/miniapp/src/settings/SettingsScreen.tsx
web/miniapp/src/*
internal/adapter/inbound/miniapp/handler.go
internal/adapter/inbound/miniapp/dto.go
internal/adapter/inbound/miniapp/handler_test.go
```

Smoke/runtime:

```text
scripts/dev/start-payments.ps1
scripts/dev/status-payments.ps1
scripts/dev/stop-payments.ps1
cmd/provider-webhook/main.go
RUNBOOK.md
```

## Current repository caution

At the time this handoff was created, the branch had many uncommitted billing and runtime changes from the chapter work. Before continuing:

```bash
git status --short
git diff --stat
```

Do not revert unrelated user/agent changes. Do not commit or push unless explicitly requested.
