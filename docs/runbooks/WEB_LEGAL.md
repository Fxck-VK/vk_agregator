# Website documents and account choices

Status: active operational contract. Publication is controlled explicitly by the
runtime flag; it does not certify external infrastructure or processor contracts.

The current local reviewed edition is `2026-10-29.1`, effective 2026-10-29.
It discloses the separately accepted subscription renewal retry policy below;
the privacy and optional marketing document bodies and marketing version remain
unchanged. Test operations remain distinct from real payments. This source
revision does not enable real payments or certify a DEV/public deployment.

## Source and public pages

`web/platform/legal/catalog.json` contains the four reviewed Epifera drafts and a
separate optional email marketing consent. It contains operator placeholders, not
personal requisites. The catalogue is shared by the Go API and server-only Next.js
renderer. Text changes require updating `revision_hash`, document versions/date and
reviewing the resulting documents. The source hash covers document titles,
sections, paragraphs and tables. Both runtimes validate it before use.

## Subscription renewal consent and retries

The agreement and payment/refund terms disclose the canonical
`internal/service/economycatalog/policy.json` renewal policy: version 1, at least
24 hours between attempts, a 14-day window from the first authenticated
insufficient-funds failure, and at most 14 failed attempts including that first
failure. No new attempt starts at or after the deadline, and a delayed worker
does not catch up missed attempts. The payment-details document links to these
same terms without describing source changes as an enabled live service.

Only authenticated provider verification of a canceled, unpaid attempt with the
exact amount/currency and `insufficient_funds` can authorize another attempt.
Webhook reason fields, immediate create responses and unknown outcomes do not
establish this proof. Other or unknown reasons, missing proof, exhausted limits,
user cancellation or refusal to use the payment method stop new attempts. The
existing cancellation action and written support requests remain available;
neither account deletion nor accepting a new legal edition is a prerequisite.

New opt-ins explicitly retain retry policy version 1 with the immutable purchase
consent. Existing consent without that version stays without retries; publishing
new terms never adds retry permission to old opt-ins. Each attempt keeps the
original renewal plan, price, period and star grant. No new stars or period access
is granted before confirmed success; the paid period begins when the service
confirms successful payment using its verified settlement time, rather than the
provider's capture timestamp or the first failure.
Historical purchases, consent snapshots and financial evidence remain immutable.

The terms implement the owner's approved 14-day limit, rather than a provider
guarantee. Primary references checked 2026-10-29:
[YooKassa saved-method recurring payments](https://yookassa.ru/developers/payment-acceptance/scenario-extensions/recurring-payments/pay-with-saved)
and [Rospotrebnadzor on stopping charges after refusal](https://zpp.rospotrebnadzor.ru/news/federal/571840).
The provider supports saved-method charges; withdrawal must stop further use of
the method. Runtime activation, live-money testing and publication remain separate
authorized operations under the billing and release runbooks.

## Public rendering and runtime configuration

Localized entry: `/ru/legal` or `/en/legal`; individual documents are under
`/legal/{user-agreement,privacy-policy,paid-access,payment-details,marketing-consent}`.
Russian is the authoritative text; English navigation does not imply a translated
contract. Pages render escaped text, are dynamic, public and noindex. Links exist
in registration, public/marketing footers, checkout and account profile.

The local 2026-10-25 presentation rebuild uses saved Magnific reference 23:
`LegalPageFrame` owns its icon rail, compact header, grouped document navigation
and reading pane. Mobile navigation uses the existing modal primitives. The old
test layout and page-only CSS were removed; registration/profile styling stays
separate. `LegalPageContent` still renders the same server-resolved catalog,
without copying operator fields into client navigation props. This UI change
does not revise document texts, checksums, publication flags or acceptance rules.
Its deployment must be verified separately before describing it as live.

Operator fields are runtime-only configuration, never frontend public env/build
arguments, source, logs or browser storage:

- `LEGAL_OPERATOR_NAME`: operator name approved for publication.
- `LEGAL_OPERATOR_INN`: approved 12-digit identifier.
- `LEGAL_DOCUMENTS_PUBLISHED`: defaults to `false`.

Configure identical values in API and platform environments. Both DEV Compose
services read the same optional protected `.env.dev-legal` overlay, which takes
precedence over API's base `APP_ENV_FILE`. The DEV workflow reads encrypted repository
secrets `DEV_LEGAL_OPERATOR_NAME`, `DEV_LEGAL_OPERATOR_INN` and
`DEV_LEGAL_DOCUMENTS_PUBLISHED` through `prepare-dev-legal.sh`. It stages and
atomically replaces that file with mode 600. The overlay survives ordinary
deployments. Automatic image publication and manual Deploy DEV now share the
same-commit reusable workflow, so automatic publication includes these secrets.
After changing legal secrets without a code push, dispatch Deploy DEV explicitly
from `dev-deploy` to apply them using the existing successful exact-SHA images.
Missing/invalid requisites block requested activation. Local `.env.local` is Git/Docker ignored.
No private `.cache` input is imported by application code or Dockerfiles.

Missing/invalid operator configuration keeps publication off even when requested.
The publication state, versions/date, exact source and resolved operator fields
all enter the document checksum. API/frontend drift blocks acceptance instead of
recording acceptance of a different document. A published draft cannot be inferred
from a deployed code version: runtime publication is separately controlled.

## Persistence and enforcement

Additive migration `000062_account_legal_consents` introduces revision snapshots
and account-owned events. Applying the migration neither accepts terms nor grants
marketing permission. The normal authorized release/migrator path must apply it
before enabling publication.

- `GET /web/v1/account/legal`: cookie-owned, no-store current status.
- `POST /web/v1/account/legal/accept`: explicit `accepted=true`, exact revision,
  locale and UUID `X-Idempotency-Key`; no client account identity.
- `POST /web/v1/account/legal/marketing`: explicit `opt_in`, locale and UUID key.
  Grants require the exact current marketing revision and `recipient_version`.

All writes retain existing cookie authentication, exact Origin and CSRF checks.
The active account row is locked in the same way as identity replacement/unlink.
Evidence stores account, server time/sequence, request ID, action, exact revision
snapshot/checksum and locale. Contacts, IPs and prompt content are not copied into
events. Identical retries do not duplicate evidence; changed payloads cannot reuse
the same request ID.

Marketing is initially unchecked and separate from service terms. The recipient
version is a domain-separated account-and-address digest for the first verified
email in canonical identity order. The address the user saw is checked again under
the account lock. A new primary email cannot inherit a previous address's opt-in.
The latest grant/revoke event and matching current revision determine effective
permission. The digest is a concurrency token, not an authorization credential.

Withdrawal does not require accepting new terms, a current marketing version or
enabled publication. Workspace pages and new actions have no separate legal gate
or first-action dialog. Cookie authentication, exact Origin/CSRF, ownership,
billing, moderation and job/idempotency checks remain mandatory.

Registration shows a small linked continuation notice; ordinary login and password
recovery show none. `/login?mode=register` opens registration directly. Final email
registration includes the exact displayed version/checksum/locale. The API validates
it before completing signup and records it against the proof-owned account before
issuing cookies. Proof-bound idempotency survives transient failures. OAuth signup
stores the notice in the existing one-use, browser-bound server transaction and
records it after verified identity resolution, before session issuance. Ordinary
OAuth login keeps its existing resolve-or-create behavior and does not manufacture
legal evidence. No historical account is automatically marked accepted. A revision
change requires reloading the registration page; it never accepts a newer unseen text.
Advertising is not granted by registration. Its voluntary settings and independent
withdrawal remain in the account profile. Historical evidence is preserved.

There is no marketing sender in this change. A future sender must recheck current
recipient, consent version, latest withdrawal and publication before dispatch;
never export a lasting list of previously opted-in recipients. Login/security
email continues independently. Preview mode neither saves synthetic consent nor
forwards writes to a real backend.

## Before publication

1. Confirm actual database/backup locations, recipient providers and applicable
   processing contracts against the factual system. A competitor's text proves
   none of these facts for Epifera. Keep the private readiness report separate.
2. Review operator requisites and the dated texts. Verify that available payment
   modes, separate renewal consent and actual runtime behavior match the edition;
   publishing documents alone must not enable real payments or migrate old opt-ins.
3. Release tested code/migration through the existing authorized pipeline. Configure
   identical runtime fields for API/platform, then enable publication deliberately.
4. Smoke public texts/links, mismatched revision rejection, cookie/CSRF enforcement,
registration evidence, ungated authenticated actions, optional marketing, independent withdrawal and new
   primary email behavior on a synthetic account. No real payment/provider call.
5. Preserve accepted snapshots when changing text; do not rewrite historical rows.
   Schema rollback is not automatic. Publication can be paused without blocking
   withdrawal or deleting existing evidence.

The generated-file lifetime is governed separately by
[generated-file retention](../../docs/DATA_RETENTION_POLICY.md#generated-file-lifetime-two-calendar-months).
This integration does not change backups, processor contracts or data locations.

## Focused verification

Backend: `go test ./web/platform/legal ./internal/service/legalacceptance
./internal/adapter/inbound/websession ./internal/platform/config ./cmd/api`.
PostgreSQL persistence checks use `LEGAL_TEST_DATABASE_URL` and refuse databases
whose name does not start with `epifera_legal_`. They create/clean a unique schema;
never point this test at DEV/PROD. UI checks cover links, unchecked marketing,
registration revision mismatch, retry identity, ordinary login without fake evidence, profile access and
withdrawal while paused. Run package typecheck, lint, tests, build and packaging.
