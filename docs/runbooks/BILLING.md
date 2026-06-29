# Billing Runbook

Billing is ledger-based and provider-verified. Redirect URLs are not proof of
payment.

## Core Invariants

- Balance changes go through ledger entries only.
- Payment top-ups require payment intent, webhook inbox/dedup, provider
  `GetPayment` verification and idempotent ledger top-up.
- Mini App/VK redirect return never credits balance by itself.
- Replayed webhooks must be no-op, not double credit.
- Refunds must use safe backend/operator flow.
- Do not expose raw YooKassa payloads, auth headers or customer PII in logs or
  operator DTOs.

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
