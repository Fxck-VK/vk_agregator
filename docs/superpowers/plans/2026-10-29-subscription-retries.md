# Subscription retry policy implementation plan

> **For agentic workers:** Use bounded subagent tasks for legal/frontend work and
> review; the primary agent owns backend integration and all verification.

**Goal:** Daily insufficient-funds renewal retries within14 days, with matching
documents and opt-in notice, preserving existing financial and layout contracts.

**Architecture:** Canonical public policy plus immutable purchase consent; private
JSON subscription recovery state uses existing owner locks/due_at indexing.
Authenticated provider cancellation reason gates new durable payment attempts.

**Tech Stack:** Existing Go/PostgreSQL, YooKassa adapter, shared Next frontend and
embedded legal catalog; no new dependency or database migration.

## Global constraints

Use only the active checkout and localhost7159. Preserve all earlier dirty work.
No commit/push/deploy/live-money or public tunnel. Approved design:
`docs/superpowers/specs/2026-10-29-subscription-retries-design.md`.
Legacy consent stays no-retry. No price/layout/provider-admission change.

## Task1: financial policy and state transitions (primary agent)

- [x] Add failing Go regressions for a verified insufficient-funds cancellation,
  unchanged balance, no retry before24 hours, new stable key after due time,
  exactly one grant after recovery and no retry at/after14 days or14 failures.
- [x] Extend ProviderPayment's private normalized cancellation reason and test
  the authenticated YooKassa GET mapping. Do not trust webhook payload reasons.
- [x] Add versioned14/24/14 rules to the economic policy/public export, additive
  private subscription retry fields and immutable purchase policy snapshot.
- [x] Implement exact-attempt retry scheduling/expiry/reset and terminal proof
  checks in subscription_failures.go/subscriptions.go; preserve all existing
  ambiguous submission and cancellation locks. Immediate terminal creation
  responses remain pending until GET confirms their outcome.
- [x] Cover legacy/no-opt-in, hard/unknown reasons, stale events, rollback,
  cancellation, delayed worker, deadline and successful recovery using actual
  PostgreSQL transactions in dedicated TEST_DATABASE_URL schemas.

## Task2: client consent and schedule (bounded frontend task)

- [x] Existing checkout sends retry_policy_version1 only with renewal consent;
  RU consent discloses24 hours/14 days from canonical generated policy.
- [x] Existing renewal_at shows the actual scheduled retry, preserving DTO
  ownership and private fields; no CSS or structural redesign.
- [x] Add request/copy/profile regressions and update the narrow UI contract.

## Task3: legal documents (bounded legal task)

- [x] Update existing user agreement and payment/refund terms; bump reviewed
  revision/date/hash with current test/helper conventions. No operator data,
  privacy/marketing changes or manufactured historical acceptance.
- [x] Legal catalog tests bind the interval/window to canonical policy and
  preserve shared server/browser rendering and publication gates.

## Task4: integration and verification (primary agent)

- [x] Run focused red/green tests, full payment/billing/adapter/webhook suites,
  PostgreSQL retry cases, affected web/legal tests, export checks, types/lint,
  go vet, formatting and diff checks. Reuse valid unrelated results.
- [x] Request a bounded independent review of failure proof, state/lock/key
  transitions, legacy consent and legal/client consistency; resolve findings.
- [x] Update BILLING.md, WEB_LEGAL.md, CP1251 architecture and current state;
  preserve earlier TEST-shop receipts as historical evidence. Save a separate
  sanitized local schedule-verification report, then report deployment limits.

Commands use existing D-drive Go caches and isolated epifera_payments_test_20261029.
No financial tests may point at the application or production databases.
