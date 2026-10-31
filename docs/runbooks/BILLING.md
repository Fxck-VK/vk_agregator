# Billing Runbook

> VK Mini App was retired on 2026-10-23. The website, browser VK ID and VK bot remain. Old Mini App routes, launch-auth and UI examples below are historical only. Current replacement and migration details: [retirement runbook](../../docs/runbooks/VK_MINIAPP_RETIREMENT.md).

Billing is ledger-based and provider-verified. Redirect URLs are not proof of
payment.

## Subscription economy (local implementation, 2026-10-27)

The user approved adopting the researched Study24 subscription/shared-balance
model. Canonical commercial data is embedded in
`internal/service/economycatalog/policy.json` (version 20261027). Regenerate its
public projection with `python scripts/economy/export-web.py`; `--check` detects
drift. UI numeric offers come from `web/platform/src/config/economy.generated.json`.
No annual offer or unverified promotion is sold.
The user-owned six-card tariff presentation was restored after the user rejected
the incidental redesign. Existing badges/editorial feature copy are presentation
content, not provider admission, an extra allowance or a server discount rule.
Only the canonical base price/stars/period enter checkout. Payment terms open from
the selected card in a separate child dialog; tariff styling remains unchanged.

| Code | Kopecks | Stars | Exact period | Next plan with opt-in renewal |
| --- | ---: | ---: | ---: | --- |
| lite | 19900 | 400 | 7 UTC days | start |
| start | 54900 | 1250 | 30 UTC days | start |
| pro | 99900 | 2100 | 30 UTC days | pro |
| ultima | 199900 | 4300 | 30 UTC days | ultima |
| elite | 499900 | 11000 | 30 UTC days | elite |

The first verified Lite consumes the one-trial entitlement permanently. Trial
reuse is still denied after expiration, cancellation or refund. Existing purchased
and legacy funds are preserved; adoption does not reprice historical jobs.

Additive migrations 000063 and 000064 create credit lots/reservation allocations
and private versioned subscription state. Financial down migrations preserve
history. Grant, capture, release, adjustment and expiry still append ledger rows;
the cached total remains the committed ledger sum. Reservations spend earliest
expiry first. Expiry removes only unreserved remainder; an already-created hold
can complete, while release after expiry cannot restore spendable funds.
Public spendable reads filter expiry/holds immediately, before the bounded sweep.
Normal maintenance and provider-webhook subscription maintenance expire lots.

Current API (cookie principal, no-store; writes require Origin/CSRF):

- `GET /web/v1/subscription`: safe plan, paid-until, next charge, renewal state,
  attributable balances and required owner-scoped `trial_used`. This durable bit
  remains true after expiry, cancellation, refunds or an absent current period;
  an unpaid intent never consumes it. The UI hides consumed Lite offers and
  treats missing/failed eligibility as unknown, never as a fresh trial. Deploy
  the matching API/website contract together. Never serialize private
  subscription JSON/method ID.
- `POST /web/v1/subscription/checkout`: only `plan_code`, explicit `auto_renew`,
  account-scoped UUID `X-Idempotency-Key`; verified account email is server-owned.
- `POST /web/v1/subscription/cancel`: disables renewal, retains paid access;
  atomically cancels a never-submitted queued renewal only.

Checkout creates an immutable intent before contacting the provider. Only verified
`GetPayment` settlement in the payment transaction activates a period and grants
one subscription lot. Immediate success from CreatePayment remains pending until
verification, so a missing webhook is recoverable by reconciliation. A new period
begins when its verified successful payment is settled. Renewal is due at paid-end,
not one day earlier: competitor documents disagree; our UI shows the exact date.
No automatic proration, rollover, annual purchase or overlapping plan is invented.

Recurring checkout requests saving the method only after explicit opt-in. A
verified saved method stays private. One durable intent/key belongs to each
attempt; a proved failed attempt may open the next attempt of the same cycle.
Cancellation is rechecked under the owner/subscription lock before submit,
and a submission-start receipt commits in a separate transaction before any
provider call. Timeout or response-transaction rollback cannot make a submitted
renewal look unsubmitted. Ambiguous submissions older than 23 hours require operator reconciliation
rather than retry beyond the provider's idempotency window. A failed/canceled
renewal is shown as past due. Only policy-v1 opt-ins may retry an authenticated,
unpaid/uncaptured `canceled` payment with reason `insufficient_funds`. Other reasons
and legacy consent stop further charges and clear the saved-method reference.
The verified terminal webhook/reconciliation updates that decision atomically
with payment status and inbox acknowledgement. Only the exact
pending renewal of the same previous paid intent can change it. Old failures do
not overwrite a newer checkout/paid period or the user's explicit cancellation.
Amount/currency and unpaid/uncaptured terminal results are checked, and duplicate
delivery does not rewrite the subscription. Paid history, trial usage and ledger
grants remain unchanged. A minute worker
runs bounded renewal/expiry batches independently of webhook reconciliation.
If the provider accepted a payment but its local binding was lost, only the
authenticated `GetPayment` metadata may recover the original pending intent.
Account, intent, submission receipt, amount and currency must all agree. Webhook
metadata alone is never evidence. A late paid period does not undo cancellation.
Authenticated terminal canceled/failed responses may resolve a lost binding
without granting credits and permit a fresh checkout. Each renewal invocation
owns a private attempt UUID checked under the owner lock; a superseded invocation
cannot submit or retire a later attempt. Its original deadline is never extended.
If cancellation wins before the owning first invocation calls the provider, that
provably unsent intent is retired. A crashed or superseded ambiguous invocation
still requires operator reconciliation; absence of a response never proves that
no charge occurred.

### Daily retry policy v1 (2026-10-29, local)

Owner-approved rules live in `economycatalog/policy.json.renewal_retries` and its
deterministic public export: minimum24 hours between attempts, a14-day window
from the first verified insufficient-funds failure, and at most14 total failed
attempts including the first. The window is half-open; no new submit begins at
or after its deadline. Missed days never cause catch-up bursts.

Checkout accepts only the policy version, never caller-supplied timing/counts.
`retry_policy_version=1` requires explicit renewal opt-in and freezes the rules in
the purchase snapshot and private subscription state. Missing/0 retains legacy
no-retry behavior; accepted old terms and paid orders never expand retroactively.
Retry keys append the durable verified-failure number to the original cycle key;
ambiguous outcomes keep their existing intent/key. Prices, plan snapshots and
receipts remain the original cycle's terms. No failed attempt grants new stars or
a new paid period. Verified recovery settles exactly once, starts its period at
service confirmation and clears recovery state. Late paid outcomes still grant
their paid period, but cannot restore expired or canceled renewal permission.

Cancellation clears the retry schedule immediately under the canonical owner
lock. Both preparation and final submit recheck expiry against current time;
provably unsent first attempts can retire, while ambiguous submissions preserve
their binding for reconciliation. Stale scheduler backoff cannot shorten a newer
verified24-hour schedule or overwrite explicit cancellation. The private state
uses additive JSON fields in the existing subscription row; no migration or
application-database rewrite is required. Existing `renewal_at` exposes the actual
next attempt, without method IDs or raw cancellation data.

User agreement/payment-refund terms and the existing opt-in checkbox disclose
these rules. The legal source edition advances independently of immutable prior
consent evidence; no marketing/privacy body or layout changes are required.
Offline unit/PostgreSQL regressions cover every interval/deadline boundary,
legacy consent, cap, replay, rollback, cancellation races and successful recovery.
Evidence: `.cache/economy-complete-2026-10-28/subscription-retry-report.json`.

Full provider-side subscription refunds are fetched through `GetRefund`, bound to
the exact local payment and verified amount/currency. They revoke only unspent
credits from that grant and matching access/renewal, preserving other lots and
existing holds. Duplicates are no-ops. The old generic operator top-up refund is
blocked for subscriptions; use the provider's refund UI with support review.
Partial refunds/proration are unsupported and stay unacknowledged for operator
attention rather than silently leaving a partially refunded subscription active.
The generic top-up refund is also rejected while an account has spendable or
reserved expiring credits: that legacy refund/compensation path has no exact lot
attribution and must not turn subscription credits into permanent purchased funds.
Support must resolve these mixed-balance cases before using that operator flow.

**Availability:** web and recurring payment enablement still require the existing
development/staging YooKassa test-shop gate. This local change does not enable
live merchant charges, deploy code, migrate the application DB, or create real
payments. Local website preview returns no active subscription and never forwards
checkout/cancel writes. Real-money rollout still needs merchant/recurring readiness,
reviewed current payment documents and an explicit release with live smoke.

### First credit-lot rollout

Migration 000063 performs a one-time balance/reservation backfill. Do not run old
financial writers after that backfill: they do not maintain the lot projection.
For the first authorized rollout, pause new financial admissions, drain in-flight
financial transactions and stop the old API, worker, webhook and maintenance
writers before applying 000063/000064. Preserve inbound payment events for replay.
Start the matching new writer versions together, verify ledger totals against
lot/reservation projections, then resume admissions and webhook reconciliation.
Stateless rollback to a pre-lot writer is unsafe after this cutover; financial
recovery must retain a lot-aware writer and preserve the append-only history.
This task does not perform that application-database cutover.

### Exact rates and remaining product gaps

The complete local policy is `catalog-policy.json` version2026102802:108 registered
models/routes,104 with paid rates, one sponsored chat and three explicitly
unavailable routes. Compatible Study24 references remain in `retail-policy.json`
version20261028. Other implemented products use stored provider cost times3,
rounded once up to5 stars, or an explicit approved operator price. Every new
quote records its basis. Provider floors/conversions/admission/spend caps and
historical Job prices stay intact. Nano Banana Pro output admission remains one.
See the [current inventory](../research/epifera-economy-2026-10-28.md) and
[operational contract](../../docs/runbooks/MODEL_ECONOMY.md).

The dated [2026-10-28 pricing research](../research/study24-pricing-2026-10-28.html)
now covers all 94 current catalogue/comparison entries and their individual pages:
43 published media pricing configurations with 88 conditional rate branches,
35 dynamic text ranges, eight free models, two fixed legacy image tools,
one legacy per-second rate and five embedded apps. The
[machine-readable data](../research/study24-pricing-2026-10-28.json) retains
parameters, dependencies, required/accepted inputs, multipliers, source dates and
hashes. Six current application surfaces were inspected, including Image Studio:
11 selectable image models, bundle-aware Grok pricing, 14 published Suno operation
rates (four source-only), photoshoot, slides, coder and seller.

Do not flatten these findings into one family minimum. Image Studio identifies
GPT Image 2.5 Flare at 52 per image, while photoshoot is a separate 25/35-per-frame
scenario. Grok returns two images per 20-token task in Studio. The legacy tariff
table is stale for photoshoot, slides, coder and seller. Disabled Studio constants
for Flux/Midjourney/Topaz are not active offers. Public media rounding applies to
the full unrounded rate times duration, not the rounded per-second display: the
verified Seedance 2.0 720p/reference/5-second example is 312, not 315.

The research does not verify actual provider debits. Its94 competitor positions
and16 compatible source mappings describe source equivalence, not missing local
prices: all104 locally priced models/routes now have explicit rules. Unknown
closed text coefficients are not invented; our text product is a fixed bounded
reply with full input/output cost caps. SunoV6 non-Max track requests guarantee
at least one playable track; all additional alternatives remain free. This
condition is frozen with the Job price, independent of later policy changes.

Reference quotes must cover nominal provider cost even at the cheapest paid
subscription star price. Otherwise cost times3 applies. The three unsafe Flare
base1:1 modes now quote220/165/355 stars; reference/aspect/batch quotes are checked
as complete requests. GPT batch prices share input once and reach both UI
consumers through the server's `price_by_count` map. New active top-ups cannot
undercut the minimum income ratio; existing intents and settlement remain frozen.
Run `go run ./scripts/models/check` or `go run ./scripts/economy/check` for the
mandatory whole-catalogue audit. `retail-audit` remains a nominal floor report.

TTS and two Hailuo routes remain unavailable because their execution/cost units
are not admitted. Pure photoshoot/slides/seller calculators remain separate until
their workflows exist. Competitor-only dynamic formulas, missing model versions,
four-output Nano batches, Suno Max/Wild/Mini integrations, promotional/partner
terms, team billing, upgrade/proration and partial subscription refunds are
separate product gaps, not unpriced admitted operations. Existing top-ups remain
separately purchased non-expiring funds; no invented149 RUB pack. Real merchant
enablement and the financial-table cutover still require a separate release.

Implementation/verification: [adoption plan](../superpowers/plans/2026-10-27-study24-economy.md).
Primary sources: [Study24 pricing](https://study24.ai/pricing),
[account terms](https://study24.ai/license/),
[subscription terms](https://study24.ai/license-subscribe/),
[YooKassa recurring API](https://yookassa.ru/developers/payment-acceptance/scenario-extensions/recurring-payments/basics).

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
- Browser top-up checkout compares the returned amount, currency and stars with
  the selected package before navigation or pending-payment storage. A mismatch
  refreshes the catalogue and clears that attempt; updated terms require another
  purchase click. Ordinary uncertain network retries retain their existing key.
- The browser payment boundary normalizes the Go API currency `rub` and ISO
  `RUB` to `RUB` for catalog, checkout and status responses. Other currencies
  remain rejected; normalization never changes amounts or granted stars.
- After an image activation402, the UI hides stale balance/remaining figures
  until a no-store balance read succeeds. Failed/malformed reads remain unknown;
  the frozen Job price is unchanged. Subscription discount badges derive whole
  percentages rounded down from current and displayed previous prices, without
  changing the actual offers or card layout.

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
https://epifera.com/billing/webhooks/yookassa
```

DEV webhook URL:

```text
https://dev.epifera.com/billing/webhooks/yookassa
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

## Local TEST-shop verification (2026-10-29)

The browser subscription and top-up buttons were exercised against the real
YooKassa TEST shop using an isolated local database and the normal session/BFF
flow. Verified public HTTPS notifications: `payment.succeeded`,
`payment.canceled`, `refund.succeeded`. The receiver kept HTTPS enforcement,
the published YooKassa source-IP allowlist and authenticated provider GET
verification. Reconciliation was delayed for this test so it could not hide a
broken notification path. Temporary tunnels expose only the webhook route;
disable their URL in the TEST merchant profile and close them after testing.
No live-money or paid AI-provider call was made.

Manual-refund correction (2026-10-29, local): an older unused top-up can be fully
refunded after a wholly unused subscription has been refunded. The history scan
excludes only a committed adjustment that the transaction-bound PostgreSQL
repository proves reverses that exact subscription grant in full. Proof checks
the canonical owner/account, source grant/lot and reserved reversal key, current
denomination, empty retired subscription lot, immutable purchase snapshot,
refunded YooKassa intent and processed, provider-verified refund event. A key
prefix, reason text or aggregate balance alone never authorizes this exception.

Actual later spend, pending debits, held/partially spent subscription stars,
ordinary expiry without a verified refund, unrelated adjustments and unknown or
incomplete attribution remain blocking. Missing proof support fails closed;
storage errors abort the refund. The canonical owner lock serializes proof and
debit with other billing writes. Existing non-expiring-balance, bounded history,
full/operator-only refund, idempotency and provider-failure compensation rules
are preserved. This is not general lot-aware automatic or partial refund support.

PostgreSQL regressions cover exact replay, concurrent refund requests and a
concurrent committed spend, proof errors, ownership/snapshot/denomination faults,
holds, partial consumption, rollback and failed-provider compensation. No new
migration, API route, frontend layout or live-money call was required. Regression
receipt: `.cache/economy-complete-2026-10-28/refund-reversal-report.json`.

Sanitized local evidence: `.cache/economy-complete-2026-10-28/test-shop-report.json`
and `test-shop-ui-https-report.json` in the same ignored folder. The test keys,
synthetic-account mail and raw provider events remain private runtime data.

## Local recurring TEST-shop verification (2026-10-29)

Historical evidence below exercised the original legacy/no-retry consent before
the owner's later14-day policy amendment. It is not evidence that fourteen daily
v1 attempts were sent to the payment provider. The v1 schedule is verified with
accelerated isolated fixtures and the existing authenticated provider contract.

Real YooKassa TEST-shop recurring payments were exercised with new synthetic
accounts in a separate local database. The initial Lite payment saved an official
test card; only that fixture's period/lot expiry was advanced. The stock minute
maintenance worker submitted the Start+ renewal for 54900 kopecks/1250 stars.
Actual HTTPS `payment.succeeded` delivered and settled it once. The prior 400
subscription stars expired; the 60-star opening bonus remained, giving 1310
spendable stars. Duplicate notification delivery did not change the balance.

The official recurring-decline card saved successfully, then its renewal was
provider-confirmed `canceled` with `insufficient_funds`. It granted no stars and
disabled further attempts. A delay until the next scheduler pass was reproduced
and corrected: failed-renewal state now commits with the verified webhook.
A fresh provider test after the fix confirmed the subscription update within
one webhook transaction (about 6 milliseconds from the acknowledgement stamp).
Explicit cancellation preserved paid access; advancing that canceled fixture's
period and rerunning maintenance created no extra payment.

All six actual HTTPS events were processed; no due subscriptions remained.
Reconciliation was delayed 24 hours so it could not mask absent callbacks.
PostgreSQL regressions, full payment/billing/memory/webhook package tests,
`go vet` and scoped independent review passed. The temporary tunnel/receiver
were closed; the existing localhost7159 preview remained available. Disable the
temporary notification URL in the TEST merchant settings after each run.
No live-money charge, paid AI call, UI change, commit, push or deployment occurred.
Sanitized evidence: `.cache/economy-complete-2026-10-28/recurring-report.json`.
