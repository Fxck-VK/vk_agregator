# Current Handoff

Status: active
Topic: Account identity system rollout
Updated: 2026-07-05

## Branch And Current State

- Current integration branch: `serega`.
- Same current project was pushed to `origin/serega` and `origin/dev-deploy`.
- Current local HEAD when this handoff was written: `b3c71f2`.
- Recent important commit: `b3c71f2 account: finish identity rollout integration`.
- Recent merge included latest `origin/dev-deploy` provider model changes, including Nano Banana Pro routing through APIMart, before account rollout fixes were committed.
- Do not commit or push additional changes unless the user explicitly asks.

## Read This First

Use the canonical read order:

1. `AGENTS.md`
2. `.agents/state.json`
3. `docs/ARCHITECTURE.md`
4. `docs/ACCOUNT_IDENTITY_CONTRACT.md`
5. `docs/ACCOUNT_ID_ONLY_AUDIT.md`
6. Relevant local `AGENTS.md` for touched package/app
7. Code and tests

Do not read archived handoffs by default. Use `docs/INDEX.md` only to route to
task-specific docs.

## Goal Of The Account Work

The project is moving from VK-user-owned business data to account-owned business
data.

Target model:

```text
VK Bot / VK Mini App / future Telegram / Web / Mobile
    -> platform/session verification
    -> IdentityResolver
    -> account_id
    -> Billing / Jobs / Artifacts / Conversations / Referrals / Payments
```

`account_id` is the canonical owner of money, jobs, artifacts, conversations,
payments, referrals and future subscriptions. VK user id, Telegram id, email,
phone, Google, Apple and password credentials are identity bindings only.

## What Was Implemented

### Account schema and migration foundation

- Added/used account identity schema:
  - `accounts`
  - `account_identities`
  - `account_sessions`
  - `account_credentials`
  - `account_links_audit`
- Existing VK users are backfilled to accounts and `provider=vk` identities.
- Business tables have additive account owner columns for compatibility:
  - `jobs.account_id`
  - `payment_intents.account_id`
  - `artifacts.owner_account_id`
  - `conversations.account_id`
  - referral account owner fields
  - billing `owner_account_id` surfaces

### Identity and account boundaries

- `IdentityResolver` is the resolver bridge:
  - `Resolve`
  - `ResolveOrCreate`
  - `LinkIdentity`
  - `UnlinkIdentity`
- `AccountService` is the product-facing account boundary above identity/auth:
  - returns safe account DTOs;
  - lists identities without raw PII;
  - masks email/phone;
  - routes verified link/unlink;
  - keeps VK Bot, Mini App and future Web/Mobile from reading raw identity rows.

### Account API

Account HTTP routes are mounted under `/account/*`.

Implemented endpoint families:

```text
GET    /account/me
GET    /account/identities
POST   /account/identities/email/request-code
POST   /account/identities/email/verify
POST   /account/identities/phone/request-otp
POST   /account/identities/phone/verify
DELETE /account/identities/{id}
POST   /account/identities/{id}/unlink

GET    /account/sessions
POST   /account/sessions
POST   /account/sessions/refresh
POST   /account/sessions/logout
POST   /account/sessions/{id}/revoke

POST   /account/password/set
POST   /account/password/login
POST   /account/password/request-reset
POST   /account/password/reset

POST   /account/oauth/login
POST   /account/oauth/link
```

Responses must stay safe: no raw `external_id`, `normalized_id`, tokens,
launch params, refresh token hashes, provider subjects, phone/email in full, or
private URLs.

### Email and phone link flows

- Email link flow:
  - request code;
  - store hashed challenge with TTL;
  - verify code;
  - link email identity to current account.
- Phone link flow:
  - request OTP;
  - store hashed challenge with TTL;
  - verify OTP;
  - link phone identity to current account.
- Delivery boundaries exist through `internal/adapter/accountdelivery`.
- Default delivery remains fail-closed unless configured.
- Rate-limit keys are hashed and must not include raw email/phone values.

### Sessions and password login

- Account sessions exist for future Web/Mobile:
  - access/session handling;
  - refresh token hash;
  - revoke/logout;
  - device info hash.
- Password login is built on top of verified linked email:
  - password hashes only;
  - password cannot be set for an unverified/unlinked email;
  - reset password uses email-code verification;
  - reset rotates password and revokes active sessions.

### OAuth/platform adapters

- OAuth/platform adapter boundary exists under `internal/adapter/accountoauth`.
- Supported target providers:
  - Google
  - Apple
  - VK ID
  - Telegram
- Adapters only verify external assertions and pass verified assertions into the
  shared account auth layer. Account linking/login logic remains in the account
  layer, not in provider adapters.

### VK Bot and Mini App account ownership

- VK Bot and Mini App resolve users through `IdentityResolver`.
- Product service boundaries now prefer/pass canonical `account_id` for:
  - jobs;
  - billing balance/reservations/top-ups;
  - payment intents/history/webhook grants;
  - artifacts and reference artifacts;
  - conversations/dialog context;
  - account UI.
- Legacy `user_id` remains as channel metadata and rollback-safe compatibility,
  especially for VK delivery and historical rows.

### Account-first business flow

Current business behavior is account-native at service boundaries, with legacy
compatibility still present:

- billing service has account-owner methods for balance, reservation, refund and
  top-up;
- payment service resolves ownership through account id;
- job orchestrator accepts/persists account id;
- artifact service can save owner user id plus owner account id;
- worker checks reference artifact ownership through account owner helpers;
- dialog context creates/reads conversations by canonical account owner.

### Conflict and merge rules

- Linking an identity already attached to the same account is idempotent.
- Linking an identity owned by another account returns a controlled conflict.
- No automatic account merge exists.
- Money, jobs, artifacts, conversations, payments and referrals must not be
  copied or moved during a conflicting link attempt.
- Future merge flow must prove control over both accounts and write audit before
  any merge.

## Key Files

Docs:

- `docs/ACCOUNT_IDENTITY_CONTRACT.md`
- `docs/ACCOUNT_ID_ONLY_AUDIT.md`
- `docs/ARCHITECTURE.md`
- `.agents/state.json`

Schema/storage:

- `migrations/000035_account_identity.up.sql`
- `migrations/000036_account_identity_backfill.up.sql`
- `migrations/000037_account_business_dual_write.up.sql`
- `internal/adapter/storage/postgres/account_identity.go`
- `internal/adapter/storage/postgres/account_security.go`
- `internal/adapter/storage/postgres/account_session.go`
- `internal/adapter/storage/postgres/billing.go`
- `internal/adapter/storage/postgres/payment.go`
- `internal/adapter/storage/postgres/job.go`
- `internal/adapter/storage/postgres/artifact.go`
- `internal/adapter/storage/postgres/conversation.go`
- `internal/adapter/storage/postgres/referral.go`

Services/adapters:

- `internal/service/identityresolver/service.go`
- `internal/service/accountservice/service.go`
- `internal/service/accountauth/service.go`
- `internal/service/accountauth/password.go`
- `internal/service/accountlink/service.go`
- `internal/adapter/accountdelivery/sender.go`
- `internal/adapter/accountoauth/*`
- `internal/service/billingservice/service.go`
- `internal/service/paymentservice/service.go`
- `internal/service/joborchestrator/orchestrator.go`
- `internal/service/artifactservice/service.go`
- `internal/service/dialogcontext/service.go`
- `internal/worker/worker.go`

Inbound/UI:

- `internal/adapter/inbound/account/handler.go`
- `internal/adapter/inbound/vk/handler.go`
- `internal/adapter/inbound/vk/menu.go`
- `internal/adapter/inbound/miniapp/handler.go`
- `internal/adapter/inbound/miniapp/references.go`
- `internal/adapter/inbound/miniapp/upload.go`
- `web/miniapp/src/api/client.ts`
- `web/miniapp/src/settings/AccountSection.tsx`

## Verification Already Run

Before the last push:

- `go test ./...`
- `go vet ./...`
- `git diff --check`

Focused tests were also run for affected account/miniapp surfaces while fixing
merge fallout.

## Important Invariants

- VK Bot, Mini App and `cmd/api` must not call AI providers directly.
- Provider/media work remains in worker/services/adapters.
- Billing is ledger-based; no direct balance mutation.
- Payment redirect URL is not payment proof.
- Top-ups only through provider-verified webhook/reconciliation plus ledger.
- `account_id` owns money, jobs, artifacts and history.
- External identities are login/channel bindings only.
- No raw PII, launch params, tokens, provider payloads, prompt bodies or private
  URLs in logs, docs or API responses.
- Link/unlink/login/password/session actions need audit and rate limiting.
- Do not auto-merge accounts.

## Current Compatibility State

The project is not fully legacy-free yet.

Legacy compatibility that intentionally remains:

- `users.id` is still present and used as a channel/legacy bridge.
- Many repository method names still say `*ByUser` even when callers now pass
  canonical account owner ids.
- `jobs.user_id`, `payment_intents.user_id`, `artifacts.owner_user_id`,
  `conversations.user_id` and billing `credit_accounts.user_id` remain populated
  for foreign keys, delivery metadata and rollback.
- Some referral APIs still expose legacy naming even though account fields are
  dual-written.
- Operator filters that accept `user_id` still exist for support workflows.

This is expected during rollout. Do not delete legacy fields or rewrite
historical ownership without a dedicated migration plan and backfill validation.

## Residual Risks / Next Work

1. Run full DB migration/integration tests against Postgres for split-owner rows
   where `user_id != account_id`.
2. Add production-sized query/index checks for account-owned hot paths.
3. Finish cleanup of legacy method names after production data proves every
   active business row has a valid account owner.
4. Build real email/SMS delivery smoke if enabling standalone email/phone login.
5. Configure and smoke real OAuth provider credentials before exposing OAuth UI.
6. Implement account merge only as a separate explicit audited flow.
7. Add end-to-end smoke:
   `VK bot user -> pays/topups -> creates jobs -> links email -> logs in via web -> sees same balance/jobs/payments`.
8. Keep DEV and PROD env parity checks active before pushing account auth changes
   to production.

## Suggested Next Steps For The Other Agent

1. Read the files listed in `Read This First`.
2. Inspect `git status --short --branch` before touching anything.
3. If continuing account work, start with
   `docs/ACCOUNT_ID_ONLY_AUDIT.md` residual risks and add split-owner tests.
4. Keep changes additive and rollback-safe.
5. Do not run live VK/YooKassa/paid provider tests without explicit approval.
6. Do not push unless the user explicitly asks.
