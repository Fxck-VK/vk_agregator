# Website documents and account choices

Status: active operational contract. Publication is controlled explicitly by the
runtime flag; it does not certify external infrastructure or processor contracts.

The final public edition is `2026-10-23.4`, effective 2026-10-23. It removes the
inactive proposed tariff annex and internal audit language. The selected Russian
storage architecture is stated as a public-launch condition, without inventing a
completed migration. Subscriptions/recurring billing remain inactive.

## Source and public pages

`web/platform/legal/catalog.json` contains the four reviewed Epifera drafts and a
separate optional email marketing consent. It contains operator placeholders, not
personal requisites. The catalogue is shared by the Go API and server-only Next.js
renderer. Text changes require updating `revision_hash`, document versions/date and
reviewing the resulting documents. The source hash covers document titles,
sections, paragraphs and tables. Both runtimes validate it before use.

Localized entry: `/ru/legal` or `/en/legal`; individual documents are under
`/legal/{user-agreement,privacy-policy,paid-access,payment-details,marketing-consent}`.
Russian is the authoritative text; English navigation does not imply a translated
contract. Pages render escaped text, are dynamic, public and noindex. Links exist
in login/registration, public/marketing footers, checkout and account profile.

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
enabled publication. Profile/security, logout, legal pages, reading/deleting files
remain accessible. While published, the backend checks current acceptance before
new website conversations/messages, uploads, generation prepare/activate/retry and
checkout. The frontend gate cannot substitute for this server check.

There is no marketing sender in this change. A future sender must recheck current
recipient, consent version, latest withdrawal and publication before dispatch;
never export a lasting list of previously opted-in recipients. Login/security
email continues independently. Preview mode neither saves synthetic consent nor
forwards writes to a real backend.

## Before publication

1. Confirm actual database/backup locations, recipient providers and applicable
   processing contracts against the factual system. A competitor's text proves
   none of these facts for Epifera. Keep the private readiness report separate.
2. Review operator requisites and the dated texts. Paid subscriptions/recurring
   billing are not implemented by this integration and remain inactive.
3. Release tested code/migration through the existing authorized pipeline. Configure
   identical runtime fields for API/platform, then enable publication deliberately.
4. Smoke public texts/links, mismatched revision rejection, cookie/CSRF enforcement,
   authenticated acceptance, optional marketing, independent withdrawal and new
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
revision mismatch, lost-response retry, account switching, profile access and
withdrawal while paused. Run package typecheck, lint, tests, build and packaging.
