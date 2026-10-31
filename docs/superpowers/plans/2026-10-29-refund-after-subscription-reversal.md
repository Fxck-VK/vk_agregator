# Refund after subscription reversal implementation plan

**Goal:** Permit the full refund of an unused older top-up when the intervening
negative entry is proven to reverse a wholly unused, provider-refunded
subscription. Preserve every other spend, hold, expiry and refund guard.

**Architecture:** Add an optional typed, transaction-bound storage proof. The
payment service may exclude only an exactly attributed reversal from its
conservative history scan. No new HTTP endpoint, migration, frontend change,
provider call path, automatic refund or partial refund is introduced.

**Constraints:** Single active checkout and localhost7159. Preserve existing
work. No commit/push/deploy, public tunnel or live-money payment. Use existing
isolated TEST_DATABASE_URL and D-drive Go caches. PostgreSQL fixture setup only;
never mutate the application database to manufacture success.

## Implementation and verification

- [x] Add `internal/adapter/storage/postgres/refund_reversal_integration_test.go`.
  Reproduce Lite settlement -> older top-up settlement -> authenticated refund
  verification -> exact subscription revocation -> older top-up full refund.
  Use the real repositories and payment services, with an in-memory provider.
  Assert exact balance, lot projection, one provider refund and replay safety.
- [x] Run that test before production edits and confirm ErrRefundCreditsSpent.
- [x] Add the optional proof contract and PostgreSQL read projection. Inputs:
  credit-account ID and negative-entry ID; output `(bool, error)`. Require same
  canonical owner/account, exact grant/lot/reversal key, current denomination,
  full grant-sized committed adjustment, no job/reservation linkage, an empty
  retired subscription lot, matching immutable subscription purchase snapshot,
  refunded YooKassa intent and processed refund notification. Unknown proof
  returns false; database errors propagate. Never trust key prefix alone.
- [x] Use that proof only for committed adjustment entries in
  `paymentservice.ensureTopupCreditsRefundable`; adapters without it keep the
  conservative rejection. All other negative entries remain blocking.
- [x] Add PostgreSQL counterexamples: arbitrary/forged adjustment, incomplete
  provider proof, foreign ownership, malformed source snapshot, consumed or
  held subscription funds, and incompatible denomination/attribution.
- [x] Verify rollback before provider submit, failed-provider compensation,
  exact replay and serialization with owner-locked concurrent billing writes.
- [x] Run focused PostgreSQL and full payment/billing/memory package tests,
  relevant `go vet`, formatting and diff checks. Request bounded static review.
- [x] Update BILLING.md, the architecture addendum (preserve CP1251), and
  current state with the verified scope. Preserve the earlier live-test report
  as historical evidence and write a separate local regression receipt.

Commands use TEST_DATABASE_URL pointing at `epifera_payments_test_20261029`:

```powershell
go test -p 2 -count=1 -v ./internal/adapter/storage/postgres -run TestTopupRefundAfterSubscriptionReversal
go test -p 2 ./internal/service/paymentservice ./internal/service/billingservice ./internal/adapter/storage/memory
go vet ./internal/service/paymentservice ./internal/adapter/storage/postgres
```
