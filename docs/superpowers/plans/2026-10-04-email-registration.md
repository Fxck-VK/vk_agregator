# Email registration implementation plan

> Execute inline with executing-plans; user approved email → code → password → account and session. No commit/push requested.

**Goal:** Public email signup backed by Resend SMTP, shared Account Layer and existing cookie sessions.

**Architecture:** A short-lived Redis challenge binds hashed email and HMAC code to a random HttpOnly browser cookie. Code verification enables one-use registration proof. Accountauth validates/hashes the password and an atomic repository inserts account, verified email, credential and audit. The account UUID is derived from the server-issued challenge for safe retries after a transient session failure; retries never update a credential.

**Constraints:** Exact Origin for all public writes, no credentials in client storage/URLs, six-digit code/10-minute TTL, email and network throttling, same accepted request response for new/existing addresses. Backend availability enables signup only with configured SMTP. Existing identities never get a password through signup. Preview remains an explicitly marked UI simulation. Resend uses smtp.resend.com:587 STARTTLS, username resend, server-only API key, From noreply@notify.neiirohub.ru. No live mail recipient or deployment contour assumed.

## Tasks

- [x] Public browser routes and form: write regressions for Origin rejection and email/code/password/session flow; run tests red.
  - websession/email_registration_test.go; LoginForm/EmailRegistration.test.tsx.
- [x] Shared registration service and atomic repository: write/run tests for proof binding, expiry, replay, limits, delivery failure, weak password retaining proof, existing identity, concurrent duplicate signup and retry preserving password.
  - domain/account_registration.go; accountauth/registration.go; accountregistration/service.go; storage/postgres/account_registration.go; storage/memory/account_registration.go; storage/redis/account_registration.go.
  - Interfaces: RegisterVerifiedEmailPassword(ctx, serverAccountID, email, password) → IdentityResolution; Request(ctx,email,transportPeer) → binding; Verify(ctx,binding,email,code); Complete(ctx,binding,email,password,SessionMetadata,finalizeSession) → SessionTokens.
- [x] Wire browser actions, capability and UI; run focused Go/Vitest tests green.
  - cmd/api/main.go; internal/app/api/core.go; websession/handler.go + account_browser.go; auth/methods.ts; auth/EmailRegistration.tsx; i18n/account-auth.ts.
  - POST /web/v1/auth/email/request-code, verify-code, register. No client account UUID; only cookie binding. Existing session cookies and safe locale return path.
- [x] Document Resend configuration, update UI catalog and durable architecture; run formatting, focused auth/storage/config tests and vet, platform lint/typecheck/build/packaging and docs validator.
  - Resend settings use existing ACCOUNT_EMAIL_SMTP_* configuration. Real configuration and one live delivery depend on user-selected contour and recipient; secrets never go into tracked files.

## Verification commands

Go: `go test ./internal/service/accountauth ./internal/service/accountregistration ./internal/adapter/storage/memory ./internal/adapter/storage/postgres ./internal/adapter/storage/redis ./internal/adapter/inbound/websession ./internal/adapter/accountdelivery ./internal/app/api ./cmd/api`; corresponding `go vet`.

Web: `npx vitest run src/features/auth src/features/account src/lib/auth src/app/login src/lib/web-api/proxy.test.ts src/i18n --maxWorkers=2`; `npm run typecheck`; `npm run lint`; `npm run test:packaging`; `npm run build`.

Storage integration uses isolated TEST_DATABASE_URL/TEST_REDIS_URL when configured; skipped live dependencies are reported. Verify visual RU/EN desktop/mobile and preview no-write behavior. Run `scripts/ci/validate-docs.ps1` and `git diff --check`.

Review corrections: final session-cookie preparation runs within proof recovery; a transient password-step failure keeps the first password read-only for retry. Added regressions for both, incorrect codes, expired confirmation, old availability contracts and preview isolation.

Verification: focused Go packages and go vet passed; platform auth/account/logout/proxy/i18n suite passed (26 files, 179 tests before two additional gateway-error retry cases); typecheck, lint, production build, packaging and documentation validation passed. RU/EN desktop/mobile signup steps checked through the local preview. PostgreSQL/Redis live integration tests remain skipped without endpoints.

Pending live setup: user-selected contour, recipient for one code email, and rotated API key saved in server runtime secrets. No real emails, account creation on a live contour or deployment performed.
