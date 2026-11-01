# First-purchase subscription promo codes

> Execute in the existing active checkout. The owner approved percentage discounts on the first tariff purchase, account eligibility, admin/chat management and implementation. No real promo values were supplied: create no live campaign. No commit, push, deployment or real payment.

**Goal:** Apply a server-verified percentage discount to an eligible account's first subscription payment, with protected operator management and unchanged tariff layout.

**Architecture:** A dedicated promotion domain/service and transaction-bound stores own code configuration, revisioned quotes, first-purchase eligibility and durable reservation/consumption. Existing payment intents freeze the discount alongside the full plan/renewal snapshots; existing verified settlement grants full plan stars and consumes once. Website forms share an account-scoped applied quote; protected admin APIs/UI/CLI share the same mutation path and audit/idempotency rules.

## Approved rules and implementation choices

- Only accounts without any previous successful purchase; refunded historical purchases still disqualify. Any tariff (including eligible Lite) may be configured per code. No promo on top-ups or renewals, no extra stars.
- One code per checkout; each code once per account. Different codes can be previewed/changed before the first payment. A code is used only after authenticated provider success. Pending/unknown payments reserve capacity until verified terminal resolution, never a blind timeout.
- Preserve full base plan/stars and regular next-renewal price. Monetary math uses integer kopecks: discount is floor(base * percent / 100), charged amount is base minus discount. Percent 1..99; no zero-price payment path is added.
- ASCII case-insensitive codes, 3..64 characters from A-Z/0-9/underscore/hyphen. Configurable enabled flag, inclusive UTC start, optional exclusive UTC end, nonempty allowed plan codes, global max uses (0 unlimited). Codes are immutable after creation; revisions protect edits and stale quotes.
- Account-first lock order, then promo lock. Prevent competing payable purchases while a discounted first-purchase intent is reserved; refuse reservation if another unresolved payment already exists. Never reject verified paid money after accepting it or strand settlement due to a later code edit.
- Existing tariff markup/styles and features remain. Show authoritative discounted prices and compare checkout amount/stars with the accepted quote before navigating. Ordinary checkout keeps its current checks.
- Server owns eligibility/discount. Public API never permits promo administration; admin remains private with explicit permissions, durable audited/idempotent mutations and confirmation.

## API contract shared by implementers

- `POST /web/v1/subscription/promo` (cookie/Origin/CSRF): `{code}` -> `{code,revision,percent,plans:[{plan_code,base_amount,amount,currency:"RUB",stars}]}`. Quote only; no reservation or consumption. Invalid 400, unavailable/ineligible/conflict 409, disabled infrastructure 503. No PII.
- Existing subscription checkout accepts optional `promo_code` and `promo_revision` together. Canonical server revalidates, freezes the price and reservation atomically. Response keeps the existing payment DTO; `amount` is actually charged, `credits` is full tariff stars. Replay compares normalized promo identity and original terms.
- Protected `GET /admin/promotions?limit=50&offset=0` -> `{items:[Promotion],has_more}`. Promotion: `{id,code,percent,enabled,revision,starts_at,ends_at?,max_uses,plan_codes,reserved_count,redeemed_count,created_at,updated_at}`.
- Protected `POST /admin/promotions` creates from config fields; `POST /admin/promotions/{id}` updates with `revision` and mutable config fields. Mutations require a UUID `X-Idempotency-Key`, bounded canonical body, request fingerprint and same-key response replay; stale/conflicting payloads return409.
- Narrow assistant CLI calls these protected HTTP routes using token from private process environment, never token command arguments or direct SQL. No campaign is created during implementation.

## Tasks

### 1. Backend and durable integration

Files: new `internal/domain/promotion.go`, `internal/service/promotionservice/**`, PostgreSQL/memory promotion stores, migration000067; existing payment/subscription/websession/admin/DI integration and focused tests.

- [x] Write failing business/HTTP/transaction tests before implementation.
- [x] Implement code normalization/config validation, immutable quote/discount values, admin management, permissions and atomic audit/idempotency.
- [x] Add additive schema with usage reservation constraints/indexes. Serialize the first-purchase gate across account payment writers; preserve legacy non-promo behavior.
- [x] Integrate discounted checkout, verified success, verified unpaid cancellation, replay, ambiguous provider recovery, refunds and ordinary renewal.
- [x] Verify real isolated PostgreSQL concurrency/rollback/global-cap/owner/replay/refund and payment integration; no application DB mutation.

### 2. Website application of codes

Files: account-scoped promotion state/API helpers, existing PromoCodeDialog, ProfileWorkspace, subscription checkout hook/surfaces, safe BFF/preview contracts, i18n and tests.

- [x] Add failing form/price/checkout tests and strict quote schema.
- [x] Wire existing entrypoints into one account-scoped applied quote, with loading/error/apply/remove and cancellation of stale requests. No browser persistence of account eligibility.
- [x] Update displayed amounts using quote data; bind the same code/revision to purchase and check actual checkout amount/stars. Preserve card structure/style.
- [x] Keep local preview explicitly no real promo validation/payment; BFF must not proxy preview mutations.

### 3. Existing admin and chat tooling

Files: `web/admin/src/api/promotions.ts`, promotion screen/tests/nav and shared protected client; narrow `cmd/promotions` or existing scripts convention.

- [x] Implement list/create/edit/enable/disable/counters with explicit confirmation and retained idempotency keys on retry.
- [x] Validate permissions, conflicts, safe errors and no token rendering/persistence.
- [x] Provide usage instructions for the protected CLI without supplying or printing credentials.

### 4. Integration and review

- [x] Run dependency/security and infrastructure checks before long build/source verification; this is local feature verification, not a release.
- [x] Run scoped Go/frontend/admin checks, vet, type/lint, platform/admin builds, asset/packaging/docs and financial regression tests.
- [x] Check affected website/admin states at 360/390/768/1024/1440/1920/2560x1440/3840x2160 CSS px using isolated samples, without another active site or live payment.
- [x] Independently review feature and correct material findings. Update billing/admin/UI/legal contracts as appropriate; preserve immutable prior legal evidence.
- [x] Report what was added, what was tested and local-only status. No live promotion, commit, push or deployment.


## Verified result (local only)

- Backend domain/service/HTTP/memory/PostgreSQL/app/API suites passed, with normal Go vet enabled; final admin/legal tests and vet passed after the effective-role fix and edition change.
- Six isolated real PostgreSQL promotion tests passed (capacity races, account serialization, rollback, verified settlement replay, historical/refunded eligibility, admin audit/revision/idempotency, missing schema). Temporary database dropped.
- Platform:179 affected tests passed; final78 scoped tests also covered the revised legal parity fixture. Admin43 tests and CLI8 tests passed. Platform/admin types, lint and production builds passed; asset, packaging and documentation validation passed.
- Fresh npm audits both0, infrastructure validation passed. Go vulnerability analysis found0 reachable/imported vulnerabilities (3 existing module-only advisories, not called by this code).
- Browser:88 website states and48 admin states passed at360/390/768/1024/1440/1920/2560x1440/3840x2160 CSS px, DPR1/zoom1. Actual localhost preview plus isolated exact-component fixtures; no live provider or admin writes. Evidence: `.cache/responsive-qa/2026-10-31-promotions/report.html` and JSON/screenshots in that folder.
- Independent backend and whole-feature review completed. Fixed and regression-tested the checkout response arriving after logout/same-account reauthentication; reviewer rechecked and closed the finding. Long64-character promo wrapping fixed without redesigning tariff cards.
- Payment document edition2026-10-31.2 records discount/account/renewal/refund rules; privacy/marketing bodies and historical consent evidence stay unchanged.
- New schema is additive migration067 and seeds no campaign. Application databases were not migrated; no real code, payment, commit, push or deployment was performed. Existing local changes were preserved.
