# Public Account ID Implementation Plan

> **For agentic workers:** Use subagent-driven-development for independent UI tasks; the primary agent owns integration, backend and review. Follow the current checkout's AGENTS.md and the owner's single-workspace rule.

**Goal:** Give each account a permanent public ID, display/copy it in the profile and find it in the protected operator console.

**Architecture:** PostgreSQL owns a positive unique identity number. A small domain formatter/parser owns `EP-000123`; a narrow repository reads the account mapping. The profile boundary and existing guarded operator endpoint expose safe projections. Memory and preview implementations preserve their existing purposes.

**Tech Stack:** Go, PostgreSQL16/pgx, existing React/TypeScript platform and admin.

## Global Constraints

- Approved design: `docs/superpowers/specs/2026-10-31-account-public-id-design.md`.
- Work only in the active checkout; reuse localhost7159. Do not commit/push/deploy.
- Do not replace UUID ownership or change billing/provider/auth/session contracts.
- No new dependency, public lookup route, PII-heavy account listing or client-chosen ID.
- Account/profile additions are compatible with older DTOs; no invented production ID.
- Keep unrelated UI unchanged; required viewport widths360/390/768/1024/1440/1920/2560/3840.

## Task1: Stable storage and domain contract

Files: `internal/domain/account_public_id.go`, its tests; `migrations/000065_account_public_id.{up,down}.sql`; `internal/adapter/storage/{postgres,memory}/account_public_id.go`, matching tests; memory identity/registration creation hooks.

Interfaces:

```go
type AccountPublicIDRepository interface {
    GetAccountPublicNumber(context.Context, uuid.UUID) (int64, error)
    GetAccountByPublicNumber(context.Context, int64) (*Account, error)
}
func FormatAccountPublicID(number int64) string
func ParseAccountPublicID(value string) (int64, error)
```

- [x] Write failing format/parse tests for1,123,1000000,MaxInt64 and malformed/overflow/zero IDs; run `go test ./internal/domain -run PublicID -count=1`.
- [x] Implement canonical formatting and bounded parsing. No truncation or UUID substring.
- [x] Write failing memory tests for concurrent unique account assignment, unknown-account reads and stability after linking a second identity; watch them fail.
- [x] Add the immutable account mapping under the existing memory mutex, including email registration; never assign on read.
- [x] Write a PostgreSQL integration test using an isolated schema: create existing accounts, apply000065, assert positive unique backfill; exercise new inserts, explicit-number rejection and lookup. Read only synthetic data; never migrate an application database during this task.
- [x] Add the additive identity-column migration with uniqueness/positive constraint and the narrow PostgreSQL reads. Verify migration and repository behavior against the local PostgreSQL test container.

## Task2: Profile and protected operator contract

Files: `internal/service/accountservice/service.go` and tests; `internal/app/api/core.go`; `cmd/api/main.go`; `internal/adapter/inbound/admin/{handler.go,dto.go,operator_users_referrals_audit.go}` and focused public-ID tests.

Interfaces: `accountservice.WithPublicIDRepository(domain.AccountPublicIDRepository)`; `AccountProfile.PublicID string` JSON `public_id,omitempty`; `admin.Deps.Accounts domain.AccountPublicIDRepository`.

Operator JSON adds optional `account` with `public_id`, `account_ref`, `status`, `role`, `account_type`, `locale`, `created_at`, `updated_at`. Query is `/admin/users/operator?public_id=EP-000123`; legacy `user_id` is unchanged.

- [x] Write profile tests proving the repository's stable number is returned and lookup errors are propagated.
- [x] Wire production core to the same canonical account repository; keep existing optional DTO compatibility.
- [x] Write route tests for valid web-only accounts, invalid/unknown public IDs, conflicting filters, missing repository and unauthorized requests; prove unsafe account fields are absent.
- [x] Implement exact public-ID lookup before the legacy user dependency branch, using current operator authentication/capability/rate/audit wrappers.
- [x] Run focused accountservice, memory, admin and API wiring tests/vet.

## Task3: Profile ID and copy action

Files: platform profile identity card and its tests, `ProfileWorkspace.tsx`/tests, `lib/web-api/contracts.ts`/tests, local preview fixture, account dictionary and relevant UI contract entry.

Interface: `ProfileIdentityCard` accepts optional `publicID?: string`; `ProfileWorkspace` passes `profile.public_id`. Only backend-projected IDs appear in real sessions.

- [x] Add failing tests for displaying the public ID while hiding the UUID, explicit copy of the exact value, pending/success and denied/unavailable clipboard handling.
- [x] Extend the existing card using shared Button/CopyIcon and existing design tokens. Keep the ID selectable and bound long strings without clipping.
- [x] Add schema validation/preview fixture and dictionary copy; do not activate English routes.
- [x] Run focused tests, types and lint; check the profile at all eight required viewport widths and save evidence under `.cache/responsive-qa/` or explicitly record blocked cases.

## Task4: Operator UI, integration and verification

Files: `web/admin/src/{api/usersReferralsAudit.ts,screens/UsersReferralsAuditScreens.tsx}` and focused tests; relevant identity/architecture documentation and docs index.

- [x] Add failing tests for selecting the `public_id` query for EP IDs and preserving UUID lookup; test/render the safe account result.
- [x] Extend the existing lookup field/result with Russian account-ID labels and safe metadata; do not introduce a broad listing.
- [x] Run admin tests/types/lint and the focused combined Go/platform checks; verify source diff and unchanged owner/auth/billing boundaries.
- [x] Update durable identity/architecture/UI contracts and check docs links. Mark exact verification scope and remaining limitations.
- [x] Review the integrated change, report implementation and actual test results; leave all changes local and uncommitted.


## Verified locally — 2026-10-31

- Eight affected Go packages passed their unit/mock suites. Native Windows vet
  initially crashed inside the sandbox; separate Linux-target vet and serial
  native vet outside the sandbox both completed successfully. Final unit command
  used `-vet=off` only because vet had been independently completed; no release
  checks or CI gates were disabled.
- Real PostgreSQL16 test in a separate temporary database passed backfill,
  concurrent assignment, exact lookup, link/unlink stability, explicit/default
  assignment rejection, positivity/uniqueness and immutable rollback. The actual
  migration runner separately proved failed down preserves version/mapping and
  subsequent up is idempotent. Old destructive down was reproduced as RED before
  replacement. Application data was not migrated.
- Platform:51 focused tests, types, scoped lint and production build passed.
  Admin:9 tests, full lint, types and production build passed. CI syntax
  (actionlint), docs validation and diff whitespace checks passed. New database
  regressions are included in the existing PostgreSQL CI job.
- Profile ID/copy:all8 required viewport widths passed scoped browser checks,
  including QHD/UHD, keyboard activation, exact copy, rejected clipboard/manual
  selection and maximum-length ID layout. Clipboard was deterministically
  stubbed. Existing header overlap at360/390 predates this change and remains.
- Admin result:16 static-layout cases (normal/maxID x8 widths) passed summary
  containment, no horizontal overflow and keyboard search-field access. This is
  exact SSR markup/CSS, not live authenticated admin interaction. Existing shell
  heading overlaps the result at1024; shared shell was not changed.
- Independent review found destructive schema rollback, now corrected and
  regression-tested; follow-up approved with no remaining confirmed findings.
- Evidence: `.cache/account-public-id-20261031/` and
  `.cache/responsive-qa/2026-10-31-account-public-id/`.
- No commit, push, deployment, application-database migration or ownership change.
  Local preview uses synthetic EP-000123; real existing accounts receive their
  permanent numbers when000065 is applied in an authorized deployment.
