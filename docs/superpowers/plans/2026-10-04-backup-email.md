# Backup email implementation plan

**Goal:** Add a confirmed backup email from the profile and use it to recover the same account when the original email is unavailable.

**Architecture:** Reuse verified email identities, the account-wide password and existing browser recovery endpoints. Every linked verified email already supports recovery and password login; a separate password, account or database migration would duplicate existing behavior. “Backup” describes an additional verified address, rather than a new restricted identity type.

**Scope:** Use existing AccountSecurity, CredentialField, Button and localized copy. Email aliases keep their existing sign-in capability. Adding an address requires a valid account session and a code delivered to that address. Recovery requires a fresh code, changes the shared account password and revokes old sessions. An address owned by another account must never be moved or merged. No push, personal password mutation or real email sending is part of this task.

**Execution:** Inline in the existing isolated worktree. User authorization covers implementing the already proposed backup-email behavior; committing/pushing requires a separate request.

## Task 1: Profile and recovery UI

- [x] Add failing UI tests for “Добавить резервную почту” when a verified email exists, “Добавить почту” otherwise, and the two-step code-confirmation flow.
- [x] Verify the failures with `npx vitest run src/features/account/AccountSecurity/AccountSecurity.test.tsx src/features/auth/LoginForm/LoginForm.test.tsx`.
- [x] In AccountSecurity, select the email action from verified identities, explain backup recovery in the form and show backup confirmation after successful verification. Preserve preview, CSRF mutations and safe profile refresh. Validate the returned safe identity and detect duplicate addresses.
- [x] In the Russian and English dictionaries, explain that recovery accepts the original or backup email. Keep the existing LoginForm recovery request/reset flow.
- [x] Run the focused UI tests and lint/typecheck.

## Task 2: Recovery contract verification

- [x] Add browser-boundary tests using real memory-backed auth/link services: link by code, recover through the backup address, retain account identity, rotate the shared password, revoke sessions and reject reused codes.
- [x] Verify an unconfirmed address cannot recover, a conflicting address cannot move between accounts, and a removed address cannot recover even with a previously issued code.
- [x] Reproduce concurrent unlink after recovery identity resolution; the failing test showed recovery reattaching the removed address. Extract proof-only `accountlink.VerifyEmailRecoveryCode` and use it in the account handler, preserving `VerifyEmailCode` for profile linking. Confirm the regression now passes and the original password remains unchanged.
- [x] Run `go test ./internal/adapter/inbound/account ./internal/service/accountauth ./internal/service/accountlink ./internal/service/accountservice ./internal/service/identityresolver`.
- [x] Attempt the race detector; unavailable in this Windows toolchain because CGO is disabled and no C compiler is installed. The concurrent-unlink regression uses deterministic interleaving and passes.
- [x] Update the account contract, architecture, browser runbook, UI catalog, machine decision and document index to describe verified aliases and proof-only recovery accurately.
- [x] Run frontend tests/build/packaging, documentation validation and `git diff --check`. Review the diff for unchanged ownership, code proof, CSRF, session revocation and secret handling.

## Completion

- Backend: complete `go test ./...` and `go vet ./...` passed; five backup-email browser-boundary scenarios passed. Optional live database/Redis integrations are skipped by the general suite when their test URLs are absent.
- Frontend: 242 test files passed, one skipped; 1,668 tests passed, four skipped. Lint, typecheck, production build, packaging, asset and Next lint contract checks passed.
- Scoped independent review: PASS, no findings. Existing challenge consumption still uses load-then-delete; this task does not establish atomic concurrent proof consumption. Sequential reused-code rejection is covered.
- Local API and frontend restarted; health, Russian login and profile routes return HTTP 200. Authenticated live email recovery remains a user smoke scenario, since it changes the personal password and revokes sessions.
- Existing music changes remain local and separate. No commit, push or real email delivery was performed for this task.

## Changed files

Backend: `internal/service/accountlink/service.go`; `internal/adapter/inbound/account/handler.go`; `internal/adapter/inbound/account/backup_email_test.go`.

Frontend: `web/platform/src/features/account/AccountSecurity/AccountSecurity.tsx`; its tests; `web/platform/src/features/auth/LoginForm/LoginForm.test.tsx`; `web/platform/src/i18n/account-auth.ts`; `web/platform/src/lib/web-api/contracts.ts`.

Documentation: `.agents/state.json`; `docs/ARCHITECTURE.md`; `docs/ACCOUNT_IDENTITY_CONTRACT.md`; `docs/runbooks/BROWSER_ACCOUNT.md`; `docs/INDEX.md`; `web/platform/docs/ui-catalog.md`; this plan.
