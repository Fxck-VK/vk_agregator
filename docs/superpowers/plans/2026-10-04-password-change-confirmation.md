# Password change confirmation implementation plan

**Goal:** Require the current password when replacing an existing account password, and show the correct action in the profile.

**Architecture:** Keep the authenticated, CSRF-protected `/web/v1/account/password/set` boundary. Initial setup inserts only when no credential exists; replacement verifies the current password and conditionally replaces the exact verified hash. Email-code recovery remains available from login.

**Constraints:** Shared Account Layer owns identity. No new dependencies, migrations, browser credential storage, commits or deployment. Existing music changes remain untouched. Password status is safe metadata; hashes never enter profile responses.

## Accepted design

- Existing password: current password, new password and repeat-new-password fields. Verify the current credential before replacing it.
- First password: new password and confirmation, with a verified email on the same account.
- Credential insertion and replacement must be atomic. Concurrent first setup cannot overwrite a password; concurrent change/reset cannot reuse stale proof.
- `password_set` in the safe profile determines the action label. Unknown status hides the password action instead of guessing.
- Wrong current password gives a local form error and preserves the account session. Clear all password fields after a submitted attempt or cancellation.
- Existing email-code reset keeps its session revocation behavior. Authenticated change keeps existing session behavior.

## Tasks

- [x] Add failing browser-handler tests for unconfirmed replacement, wrong proof and confirmed replacement; assert the previous password remains valid on rejection.
- [x] Add `CompareAndSwapCredential(ctx, credential, expectedHash)` to memory/Postgres stores. Empty expected hash means insert only; an existing hash means update only if it matches. Return conflict when the condition fails.
- [x] Make first setup insert-only; add `ChangePasswordForVerifiedEmail(ctx, actorID, accountID, email, currentPassword, newPassword)` with ownership, rate limit, hash verification, conditional update and audit. Route optional `current_password` to that method.
- [x] Expose optional `password_set` through the safe account profile and validate it in the web contract. Test before/after setup and profile data privacy.
- [x] Add failing UI tests for labels, current-password confirmation, mismatched repeats, failure cleanup and first setup. Reuse CredentialField/Button/StateNotice and add Russian/English copy.
- [x] Verify focused tests, full Go tests/vet, frontend tests/lint/typecheck/build/packaging and actual PostgreSQL conditional-write behavior when the local database is available. Review the security boundary and update current contract/UI docs.

Commands: `go test ./internal/service/accountauth ./internal/service/accountservice ./internal/adapter/inbound/account ./internal/adapter/storage/memory ./internal/adapter/storage/postgres`; `npx vitest run src/features/account/AccountSecurity src/lib/web-api/contracts.test.ts`; `go test ./...`; `go vet ./...`; frontend `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`, `npm run test:packaging`.

## Verification result

- Full Go suite and go vet passed.
- Focused PostgreSQL conditional-write integration passed against local PostgreSQL, using an isolated schema. The general Go suite skips opt-in database integrations when TEST_DATABASE_URL is absent.
- Frontend: 242 test files passed, one skipped; 1,662 tests passed, four skipped. Asset and Next lint contract checks passed.
- Frontend lint, typecheck, production build and packaging passed. Documentation validation and diff whitespace checks passed.
- Independent scoped security review found no blocking issues.
- Local API/frontend were restarted. HTTP `/ru/login`, `/ru/app/profile` and `/health` return 200. Browser confirms the guest profile route loads; an authenticated browser submission remains for the user to test. No personal password was changed.
- Implementation is validated locally. DEV push is requested; deployment will not be monitored. Existing music work remains separate.

## Changed files

Backend: `internal/domain/repositories.go`; `internal/service/accountauth/password.go`; `internal/service/accountservice/service.go`; `internal/adapter/inbound/account/handler.go`; `internal/adapter/storage/memory/account_security.go`; `internal/adapter/storage/postgres/account_security.go`.

Frontend: `web/platform/src/features/account/AccountSecurity/AccountSecurity.tsx`; `web/platform/src/i18n/account-auth.ts`; `web/platform/src/lib/web-api/contracts.ts`.

Tests: `internal/adapter/inbound/account/password_change_test.go`; `internal/service/accountauth/password_change_test.go`; `internal/service/accountservice/password_state_test.go`; `internal/adapter/storage/postgres/account_password_change_integration_test.go`; `web/platform/src/features/account/AccountSecurity/AccountSecurity.test.tsx`; `web/platform/src/lib/web-api/contracts.test.ts`.

Documentation: `.agents/state.json`; `docs/ARCHITECTURE.md`; `docs/ACCOUNT_IDENTITY_CONTRACT.md`; `docs/runbooks/BROWSER_ACCOUNT.md`; `docs/INDEX.md`; `web/platform/docs/ui-catalog.md`; this plan.
