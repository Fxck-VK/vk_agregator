# Study24 economic model adoption

Status: local implementation and bounded review completed after the 27 October research.
No commit, push, deployment or real-money transaction is authorized in this task.

**Goal:** adopt the researched subscription/shared-star business model in the
existing account-native billing system, with truthful availability for missing
or unverified operations.

**Architecture:** one versioned economic policy owns five subscriptions and
researched operation prices. Existing payment intents and provider verification
remain the only way a payment grants stars. Expiring grants become attributable
credit lots under the existing ledger/reservation locks. Subscription state and
grant settlement commit in the same payment transaction. Website DTOs expose
safe prices and availability; provider credentials/method IDs stay server-only.

**Global constraints**

- Single existing checkout/localhost:7159; preserve pre-existing login CSS edit.
- Public stars retain denomination v2 (1 star nominally 50 kopecks), bonus 60.
- Copy live base offers: Lite 19900 kopecks/400 stars/7 days, one trial per
  account, next Start+; Start+ 54900/1250/30 days; Pro 99900/2100/30 days;
  Ultima 199900/4300/30 days; Elite 499900/11000/30 days.
- No annual discount, synthetic successful purchase, hidden recurring consent,
  retroactive repricing, backfilled payment acceptance or provider admission.
- Expiring subscription lots spend earliest expiry first. Existing balances and
  purchased topups remain separately attributable; no blanket balance reset.
  Held amounts survive expiry only for their already-created reservations;
  release after expiry expires that amount rather than restoring spendable stars.
- New rates only affect newly quoted work. Preserve floors, caps, provider
  mappings and existing paid-job snapshots. Unknown variants do not inherit a
  competitor's minimum as an exact quote.
- Research covers 94 public entries, including 8 free-category entries. Those
  entries are not authorization to enable missing models or free provider calls.
- Additional pack sizes, coupon terms, upgrade/proration and actual text formula
  are unverified: retain explicit availability/gap records, do not invent them.
- No secrets or payment method IDs in public DTOs, logs or browser storage.
- Test provider traffic uses httptest/local mocks only. Production enablement
  still requires real shop and recurring-payment readiness; do not bypass it.

## Task 1 — credit-lot accounting

Files: domain credit-lot types and optional ledger metadata; additive migration
000063; memory/postgres billing repositories and targeted tests. Existing
BillingRepository method signatures remain compatible.

- [x] Add failing tests: FEFO reserve, exact replay, expiry with pending hold,
  late capture, late release, purchased balance preservation, cross-owner denial,
  negative adjustments/refunds and concurrency under one account.
- [x] Implement lots and reservation allocations, including legacy migration.
- [x] Ensure all existing grant/capture/refund/adjustment paths preserve ledger
  sum and lot projection. Expiry is an append-only negative adjustment.
- [x] Wire bounded expiry maintenance and run memory + Postgres coverage.

## Task 2 — canonical economic policy and operation mapping

Files: internal/service/economycatalog/**; pricingcatalog retail integration;
scripts/economy/export-web.py; generated web policy JSON.

- [x] Test exact five offers, unit/period math, defensive copies and references.
- [x] Promote sanitized 94-entry research facts into canonical tracked policy.
- [x] Implement exact supported operation mapping; preserve provider floors and
  key variants. Return explicit unsupported/ambiguous entries for the rest.
- [x] Generate shared frontend policy deterministically; verify export drift.
- [x] Test changed quotes, no new admission, duration/count units and old snapshot
  compatibility. Do not invent a dynamic text price or use a fixed 5 everywhere.

## Task 3 — subscription payment lifecycle

Files: domain subscription contracts; payment-service intent/verified settlement;
postgres/memory subscription storage; additive migration 000064; gateway adapter
only after current YooKassa documentation verification.

- [x] Add service tests for initial payment, trial reuse, duplicate/late events,
  second concurrent subscribe, next period, cancellation and failed renewal.
- [x] Persist immutable plan/period snapshots with payment intent. Activate and
  issue one expiring lot only inside provider-verified payment transaction.
- [x] Add safe current-subscription read and cancellation; period access remains
  until paid end. No initial purchase directly mutates balance.
- [x] Recurring payments use saved method only after explicit checkout consent;
  renewal work needs durable idempotency/lease and rechecks cancellation before
  provider submit. A cancellation race never silently re-enables renewal.
- [x] Refunded subscription cannot leave spendable credited lots/access. Reject
  unsupported refund/proration paths explicitly.

## Task 4 — authenticated website and public pricing

Files: websession subscription HTTP/DTOs; shared payment BFF routes; pricing
and existing choose-plan/profile components and focused tests.

- [x] Source displayed prices/stars and period examples from canonical policy.
- [x] Remove annual purchasing from the commercial contract. Preserve the user's
  original six-card tariff presentation; existing editorial badges/features do
  not define server allowances or discounts. The user explicitly rejected the
  incidental tariff redesign and required its restoration.
- [x] Connect only server-advertised available checkout. Explain next charge,
  expiry and trial conversion in normal checkout UI.
- [x] Profile shows true active plan, paid-until, next amount/auto-renew state,
  balance breakdown and cancel action; pending/errors are explicit.
- [x] Preserve session owner, Origin/CSRF and idempotency; do not expose raw data.
- [x] Test preview cannot charge, failed/canceled payment cannot activate, and
  current account owns all status/cancel operations.

## Task 5 — review, documentation and local verification

- [x] Record exact copied/unsupported/ambiguous features in billing runbook;
  update architecture and active routing only for durable behavior.
- [x] Review money arithmetic, idempotency, concurrent expiration/capture/refund,
  recurring consent, owner authorization, and provider/job boundaries.
- [x] Run focused Go/frontend checks, infrastructure/database tests when
  available, lint/typecheck and relevant packaging checks. Report skipped checks.
- [x] Reconcile final diff with authorized scope. No publication in this task.

User-facing completion must distinguish implemented local behavior from shop
configuration, missing provider operations and verified live deployment.

## Owner follow-up — hide consumed Lite offer (2026-10-28)

Approved: hide the Lite tariff for the current account after its first verified
Lite settlement, including after expiry/cancellation/refund and page reload.
Preserve every other card and existing CSS. No release or real payment.

- [x] Expose the durable server-owned `trial_used` flag in the safe read DTO,
  including states without an active paid period; test never-used/pending,
  active, expired and refunded representations. Do not infer from plan name.
- [x] Require the flag in the frontend DTO and add false to the read-only local
  simulator. Use the existing account context/read-resource for eligibility;
  no browser-storage flag. Hide authenticated unknown/error states until proven
  eligible, keep guest offers visible, recheck on checkout close/profile refresh.
- [x] Filter only the Lite card and its workspace promotion; preserve the full
  presentation of all other tariffs. Disable an already-open Lite checkout when
  the server says the trial was consumed. Keep keyboard focus when it vanishes.
- [x] Verify HTTP ownership/DTO tests, frontend eligibility/checkout regression,
  cancellation/refund persistence, types/lint/build, and update the UI/billing
  contracts. Report local-only state.

## Verification record

- Dependency admission: both npm audits reported zero vulnerabilities;
  govulncheck reported no reachable/imported-package vulnerabilities (one unused
  required-module advisory); lockfile URL/integrity and route/doc guards passed.
- Focused Go package checks and regression tests include immutable offers,
  provider verification, exact-once settlement, trial reuse, cancellation,
  durable submit receipts/fencing, lost-binding success and terminal recovery,
  historical replay, full refunds and mixed-expiry refund rejection.
- Actual isolated PostgreSQL coverage includes lot/reservation migration,
  concurrency and rollback, subscription CAS/settlement/cancellation, failed
  submit/commit recovery, generic negative debit after expiry and signup bonus
  origin. The application database was not migrated or used for these tests.
- Frontend focused tests, TypeScript, targeted ESLint, public translation/source
  checks, production build and packaging passed. Desktop/mobile browser checks
  confirm exact five offers, selection, period copy and read-only preview.
- Final bounded review resolved all original findings and its terminal-recovery
  follow-up. No new blocker remained. Crashed/superseded ambiguous attempts may
  still need operator reconciliation; private receipts do not prove no charge.
- Exact competitor variant prices, added provider admission, mixed-balance
  generic refunds, partial refunds/proration, promotions and team billing remain
  explicitly unsupported/unverified as detailed in the billing runbook.
- No live provider/payment smoke, application DB migration or release performed.
  Local evidence is retained in ignored `.cache/study24-economy-2026-10-27/`.

Lite follow-up verification2026-10-28: new HTTP eligibility cases first failed on the absent flag, then passed for new/unpaid/active/expired/refunded states and authenticated owner query isolation. Existing verified-settlement, late-cancellation and full-refund persistence checks passed. All64 focused frontend regressions passed across8 files (eligibility, remount/profile refresh, checkout blocking/focus, guest privacy, DTO currency normalization, simulator and unchanged tariff styles). Full lint initially found one ref-cleanup warning, corrected by capturing the fallback node; final affected lint and production build/type checking passed. No CSS, ledger rule, DB migration, live payment or publication changed.
