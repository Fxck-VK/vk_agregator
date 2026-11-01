# Web referral rewards

Goal: award the inviter 50 permanent stars 24 hours after each invited account's first verified live payment; cancel or reverse the entire reward on any successful refund.

Architecture: immutable provider-verified payment/refund evidence, account-native referral qualification, bounded transactional maintenance, append-only ledger and dedicated bonus lots. One reward per referred account; test/mock payments do not qualify. Previously settled payments are historical evidence, not retroactive grants. UI and legal terms describe the same policy.

Tech stack: Go, PostgreSQL/pgx, existing YooKassa payment processor, Next.js profile and shared legal catalog.

Global constraints: preserve existing uncommitted work and layouts; no publication, real payments or provider calls; no synthetic VK users, unrelated balances or generic FEFO debit for reward reversal.

- [x] Add policy/evidence contracts and irreversible additive migration.
- [x] Record verified payment/refund evidence, including partial/external refunds, with tests.
- [x] Implement delayed exactly-once award/reversal and permanent bonus/debt projection; test real PostgreSQL concurrency, rollback and held/spent funds.
- [x] Wire canonical maintenance, API policy/statistics, profile rules and reviewed legal terms.
- [x] Verify focused backend/frontend checks and all required affected profile widths; obtain independent review.

Refund accounting: remove only the original bonus lot's available remainder, retain reservations, and append nonexpiring legacy debt for spent/held credits. A later release restores reserved credits against that debt without a second expiry debit. Purchased and subscription lots are preserved.

Verification completed locally on 2026-10-31: nine actual PostgreSQL referral tests (including mixed-owner concurrency, spent/held reversal and refund/grant serialization), actual migration runner up/resume/refused-down, eight focused Go package suites, targeted vet including API/worker, 149 frontend tests, scoped ESLint/types, full platform production build, actionlint, documentation/release-contract/infrastructure checks and dependency audits. Actual profile preview and paid-access section 8 passed all eight required widths; enabled/error states additionally use 32 exact-component SSR/CSS fixtures. Independent financial and whole-feature reviews have no remaining findings. No live payment, commit, push or deployment.

Evidence: `.cache/referral-rewards-20261031/` and `.cache/responsive-qa/2026-10-31-referral-rewards/`. Local application databases were not migrated; integration tests created/dropped a separate temporary database. Apply ordered migrations including 000066 when publishing the reviewed application.
