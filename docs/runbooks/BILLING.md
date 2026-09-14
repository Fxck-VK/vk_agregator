# Billing Runbook

Billing is ledger-based and provider-verified. Redirect URLs are not proof of
payment.

## Core Invariants

- Balance changes go through ledger entries only.
- The public currency is stars. Denomination v2 is fixed at
  `1 star = 50 kopecks`; money conversion uses integers, never floating point.
- Payment top-ups require payment intent, webhook inbox/dedup, provider
  `GetPayment` verification and idempotent ledger top-up.
- Mini App/VK redirect return never credits balance by itself.
- Replayed webhooks must be no-op, not double credit.
- Refunds must use safe backend/operator flow.
- Do not expose raw YooKassa payloads, auth headers or customer PII in logs or
  operator DTOs.

## Star Denomination

Migration `000041_star_denomination` introduces denomination v2:

| RUB | Stars |
| ---: | ---: |
| 10 | 20 |
| 99 | 198 |
| 150 | 300 |
| 250 | 500 |
| 400 | 800 |
| 700 | 1400 |

- One legacy credit is converted to two current stars.
- Existing balances are doubled through an idempotent append-only ledger
  adjustment. Historical ledger rows are not rewritten.
- New accounts receive 60 stars, preserving the former 30 RUB signup value.
- Generation prices are doubled in stars so their RUB value stays unchanged.
- Products and payment intents snapshot `credit_denomination_version`.
  Historical v1 intents are converted only when displayed, credited or
  refunded; they are never recalculated from the current product catalog.
- Refund debits use the intent denomination that was active at purchase time.
- The migration down file is intentionally a no-op. Never attempt to reverse
  financial denomination changes by rewriting balances or ledger history.

## YooKassa Webhook

Production/test webhook URL:

```text
https://neiirohub.ru/billing/webhooks/yookassa
```

DEV webhook URL:

```text
https://dev.neiirohub.ru/billing/webhooks/yookassa
```

Events:

- `payment.succeeded`
- `payment.canceled`
- `refund.succeeded`

The route must reach `cmd/provider-webhook`, not `cmd/api`.

## Web platform test checkout

The web uses the existing account-native payment service and ledger pipeline:

- `GET /web/v1/payment-products`: authenticated active server catalog, amount in
  kopecks, current-denomination credits, and `checkout_available`.
- `POST /web/v1/payments/intents`: cookie session + Origin/CSRF + UUID
  `X-Idempotency-Key`; accepts only `product_code`.
  Keys are namespaced by account. Creation is limited to 10 requests/min/account.
  The Account Layer resolves the authenticated account's verified email for the
  receipt server-side; the browser cannot supply a contact. No usable email
  returns 422 before payment creation. Contacts never enter browser DTOs/logs.
- `GET /web/v1/payments/{id}`: exact account ownership, safe status DTO.
  Only approved HTTPS YooKassa checkout hosts may reach the browser.

This rollout permits **test checkout only**: `APP_ENV=development|staging`,
`PAYMENT_PROVIDER=yookassa`, configured `YOOKASSA_SHOP_ID`, a test secret key
(`test_` prefix), and a valid `WEB_ORIGIN`. Live/mock or incomplete configurations
leave checkout unavailable. Credentials belong in runtime secrets, never the
frontend or chat. The existing provider-webhook runtime must use the same shop.

Account-native web payments return to `WEB_ORIGIN/app/payment-return?payment_id=...`.
This server-owned return URL does not change VK/Mini App return URLs. The ID is
not payment proof. The authenticated page polls the stored status every 3 seconds
for up to 2 minutes, with a manual retry after timeout/network errors. It refreshes
the server-owned workspace balance only after `succeeded`. Pending IDs and request
keys may survive refresh in sessionStorage; receipt email, payment credentials and
checkout URLs are not stored there.

Deploy `platform` and `api` together. The existing `provider-webhook` processing
and reconciliation must be running; no new migration is needed. For a smoke test,
use an account with a verified email, select a server package, complete the test checkout and
verify exactly one ledger top-up even after a repeated webhook/status reload.
Cancel a second checkout and verify no top-up. Use the
[official test-payment instructions](https://yookassa.ru/developers/payment-acceptance/testing-and-going-live/testing).

The current `/payments` receipt integration requires a customer contact before
redirecting to checkout. Removing the web email field does not move contact
collection to YooKassa: the existing verified account email supplies the receipt.
See [receipt requirements](https://yookassa.ru/developers/payment-acceptance/receipts/54fz/yoomoney/payments).

`NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1` is read-only UI preview: its sample packages
always have `checkout_available=false`, and payment requests are never forwarded.
Disable preview and use the real authenticated API for a provider integration test.

## Payment Smoke

Before real-money confidence, verify:

1. create payment intent through VK/Mini App flow;
2. complete YooKassa test payment;
3. receive public HTTPS webhook;
4. `payment_events.processed_at` is set;
5. `payment_intents.status=succeeded`;
6. exactly one ledger top-up entry exists;
7. balance increased once;
8. replay `payment.succeeded` does not duplicate credits.

## Canceled Payment Smoke

Use a protected operator/test scenario:

1. create intent with `capture=false`;
2. move provider payment to `waiting_for_capture`;
3. call provider cancel through operator endpoint;
4. verify terminal `payment.canceled`;
5. verify no ledger top-up;
6. verify late `payment.canceled` cannot undo an already succeeded intent.

## Refund MVP Policy

Until lot/FIFO attribution exists:

- full refund only;
- manual/operator only;
- current user balance must cover the top-up credits;
- no automatic partial refunds for already-spent credits.

## Operator Endpoints

Use protected operator endpoints only. Responses must be safe DTOs:

- payment intents;
- ledger entries;
- refunds;
- webhook events;
- pending/stale intents;
- unprocessed events;
- operator sync;
- operator refund.

Never use direct SQL as the normal operator path.
