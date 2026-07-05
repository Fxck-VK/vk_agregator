# Account ID Only Audit

Status: active rollout audit
Updated: 2026-07-04

This document lists the remaining legacy ownership dependencies that still use
`user_id` as a business owner while the account identity rollout is in
compatibility mode.

## Scope

Audited surfaces:

- billing and ledger;
- jobs and worker lifecycle;
- artifacts and artifact access;
- conversations/dialog context;
- referrals;
- payments and YooKassa top-ups;
- VK Bot and Mini App call sites;
- PostgreSQL repository contracts and compatibility queries.

This is not a behavior change. It is the dependency map for the next
`account_id` rollout step.

## Current State

The project is in dual-read/dual-write compatibility mode:

- `IdentityResolver` creates or resolves the canonical `account_id` for VK
  users.
- Business tables have additive account ownership fields, for example
  `jobs.account_id`, `payment_intents.account_id`,
  `artifacts.owner_account_id`, `conversations.account_id`, referral account
  fields and billing `owner_account_id`.
- PostgreSQL writes often backfill account ownership from `users.account_id`.
- PostgreSQL reads often accept either legacy `user_id` or canonical
  `account_id`.
- Product services and inbound handlers still mostly pass `user.ID` into
  billing, jobs, payments, artifacts, conversations and referrals.

That is safe for rollout, but it is not yet `account_id-only`.

## Chapter 2 - Dual-Write Check

New account ownership writes are present for the main compatibility surfaces,
but most product services still pass only legacy `user_id` into repositories and
rely on PostgreSQL to backfill account ownership from `users.account_id`.

That means the current state is valid as a rollout bridge, not as the final
account-native contract.

| Surface | Legacy owner | Account owner | New-write behavior | Current risk |
| --- | --- | --- | --- | --- |
| `credit_accounts` | `user_id` | `owner_account_id` | `postgres/billing.go` backfills `owner_account_id` from `users.account_id` on insert. | Billing service still creates accounts with only `userID`; account ownership is implicit storage behavior. |
| `credit_reservations` | credit account id in `account_id` | `owner_account_id` | Insert backfills from the owning `credit_accounts.owner_account_id`. | Works only if the credit account already has canonical owner populated. |
| `ledger_entries` | credit account id in `account_id` | `owner_account_id` | Insert backfills from the owning `credit_accounts.owner_account_id`. | `GrantWith` and webhook top-up still call billing by `intent.UserID`; tests do not yet prove `user_id != account_id` top-ups end-to-end. |
| `payment_intents` | `user_id` | `account_id` | Insert backfills `account_id` from `users.account_id`; reads accept `user_id OR account_id`. | Payment service creates intents with `UserID` only; authorization, history, refund checks and webhook grant are not account-native yet. |
| `payment_refunds` | `intent_id` indirect owner | none materialized | Refund ownership is derived through the linked intent. | No direct `payment_refunds.account_id`; operator/account-scoped refund views need joins and can drift from the target model. |
| `payment_events` | provider ids | none | Webhook inbox rows are provider-event records and are not directly account-owned. | No dual-write required, but processing must always resolve through provider-verified payment/refund state. |
| `jobs` | `user_id` | `account_id` | Insert/update backfills `account_id`; reads and counters accept `user_id OR account_id`. | `CreateJobInput` has no `AccountID`; billing reserve, capacity and worker lifecycle still use `UserID`. |
| `artifacts` | `owner_user_id` | `owner_account_id` | Insert/update backfills `owner_account_id`; lookups accept `owner_user_id OR owner_account_id`. | Artifact service saves with one owner id and sets `OwnerUserID`; worker/Mini App access checks still compare legacy owner ids. |
| `conversations` | `user_id` | `account_id` | Insert backfills `account_id`; reads accept `user_id OR account_id`. | Dialog context still derives ownership from `job.UserID`. |
| `referrals` | user owner fields | account owner fields | Migration and repository writes backfill account fields. | Referral service still applies/activates/rewards by user owner fields. |

### Verification Coverage

Confirmed coverage:

- `migrations/000037_account_business_dual_write.up.sql` adds account owner
  columns, backfills historical rows and creates account-scoped indexes for
  jobs, payment intents, artifacts, conversations, referrals and billing rows.
- PostgreSQL repositories for billing, payments, jobs and artifacts can accept
  explicit account IDs or derive them from `users.account_id`.
- `internal/service/accountauth/rollout_test.go` checks that a legacy VK user
  with a pre-existing `account_id` keeps balance, jobs and payment visibility
  through the canonical account.

Gaps to close before `account_id-only`:

- PostgreSQL integration tests do not run the full account business migration
  path and do not assert `credit_accounts.owner_account_id`,
  `credit_reservations.owner_account_id`, `ledger_entries.owner_account_id`,
  `jobs.account_id`, `payment_intents.account_id` and
  `artifacts.owner_account_id` for new rows where `user_id != account_id`.
- Memory repositories often default account owner fields to the legacy user id,
  which hides missing explicit account ownership in unit tests.
- Payment refunds need a product decision: either keep indirect ownership through
  `payment_intents`, or add `payment_refunds.account_id` before account-scoped
  operator/refund workflows become first-class.

### Required Next Step

The next implementation chapter should make business services account-native:

1. Add explicit `AccountID` to billing, payment intent, job and artifact service
   inputs.
2. Pass canonical account ownership from VK Bot and Mini App after identity
   resolution.
3. Keep legacy `user_id` only as channel metadata and named compatibility
   fallback.
4. Add split-owner tests where `user_id != account_id` for ledger top-up,
   payment intent creation, job creation and artifact save/access.

## Chapter 3 - Dual-Read Account-First

Main repository read paths now prefer canonical account ownership and use legacy
user ownership only as a compatibility fallback.

Implemented account-first reads:

- `postgres.BillingRepository.GetAccountByUser` queries
  `credit_accounts.owner_account_id` first, then falls back to
  `credit_accounts.user_id`.
- `postgres.PaymentRepository.ListIntentsByUser` queries
  `payment_intents.account_id` first, then falls back to
  `payment_intents.user_id`.
- `postgres.JobRepository.ListByUser`,
  `CountActiveByUserOperation` and `CountSucceededByUser` query
  `jobs.account_id` first, then fall back to `jobs.user_id`.
- `postgres.ArtifactRepository.GetBySHA256` and
  `FindReusableInputReference` query `artifacts.owner_account_id` first, then
  fall back to `artifacts.owner_user_id`.
- Memory repositories mirror the same account-first behavior for jobs, payment
  intent lists, artifact hash lookups and reusable input-reference artifacts, so
  unit tests stop masking account ownership behind a broad owner `OR`.

This chapter intentionally keeps existing repository method names and arguments
for compatibility. A method such as `ListByUser(ctx, id, ...)` still receives one
UUID and cannot know whether the caller passed a legacy `user_id` or canonical
`account_id`. The safety rule is:

1. try the account-owned read;
2. if no rows are found, use the legacy user-owned read;
3. do not merge both result sets in hot paths until service contracts pass
   explicit `account_id` and legacy `user_id` separately.

Remaining account-first gaps:

- service inputs still need explicit `AccountID` fields for billing, payment,
  job and artifact flows;
- VK Bot and Mini App still need to pass resolved `user.AccountID` into product
  services, leaving `user.ID` as channel/legacy metadata;
- operator filters that explicitly accept `user_id` remain compatibility
  filters; account-native operator calls should use `account_id`;
- reusable artifact hot paths need split-owner tests and account-scoped indexes
  verified on production-sized data.

## Chapter 4 - Account ID Only Switch

Business service boundaries now treat `account_id` as the canonical owner for
money, jobs, artifacts, payment history and conversation history. Legacy
`user_id` remains in domain rows as channel metadata, foreign-key compatibility
and rollback-safe migration support.

Implemented account-owner writes:

- `billingservice` exposes account-owner variants for balance checks,
  account ensure, reservations, refunds and top-ups. New reservation and ledger
  writes pass `owner_account_id` explicitly instead of relying only on storage
  backfill.
- `paymentservice.CreateIntent`, active-intent lookup, attach/cancel checks,
  history and webhook processing resolve ownership through `account_id`.
  YooKassa top-up grants now call billing with both legacy user id and canonical
  account id.
- `joborchestrator.CreateJob`, capacity checks and route checks accept
  `AccountID`. New jobs persist `jobs.user_id` as the channel user and
  `jobs.account_id` as the owner used for billing, limits and worker context.
- `artifactservice` added account-aware save methods. Worker provider outputs,
  Mini App uploads and VK image/video references now save artifacts with
  `owner_user_id` plus canonical `owner_account_id`.
- Mini App job, chat, payment, balance, artifact download and conversation paths
  resolve the authenticated VK user once and pass `user.EffectiveAccountID()` to
  business services.
- VK Bot generation, top-up, balance, account view and reference-artifact paths
  pass `user.EffectiveAccountID()` to business services while keeping VK peer
  delivery tied to the channel user.
- Dialog context creates and reads conversations by canonical account owner, with
  legacy user id kept on the row for compatibility.
- Worker provider requests, reference ownership checks, output artifact writes
  and assistant context use the job account owner. VK delivery still uses
  `job.UserID` intentionally because delivery is channel-scoped, not financial
  ownership.
- Admin user balance display reads the user's effective account balance instead
  of the legacy user row directly.

Compatibility that intentionally remains:

- Repository method names such as `ListByUser`, `GetAccountByUser` and
  `GetByIDForUser` are kept to avoid a large interface rename in the same
  rollout. Callers should pass account owner ids unless they are explicitly in a
  channel/legacy path.
- `users.id`, `jobs.user_id`, `payment_intents.user_id`,
  `artifacts.owner_user_id`, `conversations.user_id` and billing
  `credit_accounts.user_id` remain populated for foreign keys, channel metadata,
  admin lookup and rollback-safe compatibility.
- Referrals already dual-write account fields, but the public service API still
  exposes legacy naming. A later cleanup can rename/refine referral contracts
  after account merge/conflict rules are fully finalized.
- Operator filters that accept `user_id` continue to exist for support workflows.
  Account-native filters should use `account_id` when available.

The project is now account-native at product service boundaries. A future
cleanup can remove legacy naming and compatibility fallbacks after production
data proves every active row has a valid `account_id`/owner account field.

## Legacy Dependency Map

### Domain Contracts

Repository interfaces still name and expose user-owned methods:

- `internal/domain/repositories.go:218` - `JobRepository.ListByUser`.
- `internal/domain/repositories.go:229` -
  `JobRepository.CountActiveByUserOperation`.
- `internal/domain/repositories.go:231` -
  `JobRepository.CountSucceededByUser`.
- `internal/domain/repositories.go:255` - `CommandRepository.ListByUser`.
- `internal/domain/repositories.go:261` -
  `ConversationRepository.GetActiveByUserPeer`.
- `internal/domain/repositories.go:267` -
  `ConversationRepository.GetByIDForUser`.
- `internal/domain/repositories.go:269` -
  `ConversationRepository.ListByUserSource`.
- `internal/domain/repositories.go:360` -
  `BillingRepository.GetAccountByUser`.
- `internal/domain/repositories.go:464` -
  `PaymentRepository.ListIntentsByUser`.
- `internal/domain/repositories.go:502` -
  `ReferralRepository.GetCodeByUserID`.
- `internal/domain/repositories.go:511` -
  `ReferralRepository.GetReferralByReferredUserID`.

Domain structs still carry legacy ownership fields:

- `internal/domain/job.go:238` - `Job.UserID`.
- `internal/domain/artifact.go:296` - `Artifact.OwnerUserID`.
- `internal/domain/conversation.go:31` - `Conversation.UserID`.
- `internal/domain/payment.go:146` - `PaymentIntent.UserID`.
- `internal/domain/billing.go:82` - `CreditAccount.UserID`.
- `internal/domain/referral.go:97` - referral user owner fields.

Target: add account-native service and repository contracts first, then make
legacy methods explicit compatibility helpers.

### Billing And Ledger

`internal/service/billingservice/service.go` still accepts `userID` as the
owner for balance lookup, reservation, grant and refund flows. The PostgreSQL
repository keeps compatibility by resolving:

- `internal/adapter/storage/postgres/billing.go:93` - insert backfills
  `owner_account_id` from `users.account_id`.
- `internal/adapter/storage/postgres/billing.go:139` - lookup uses
  `(user_id = $1 OR owner_account_id = $1)`.
- `internal/adapter/storage/postgres/billing.go:204` and
  `internal/adapter/storage/postgres/billing.go:352` - reservations and
  ledger entries backfill `owner_account_id` from the credit account.

Target: introduce account-owner billing methods and require
`owner_account_id` on new writes. Keep legacy user lookup as a named bridge only.

### Payments And Webhooks

`internal/service/paymentservice/service.go` still authorizes and lists payment
intents by `UserID`:

- `internal/service/paymentservice/service.go:454` - cancel ownership check.
- `internal/service/paymentservice/service.go:500` - source-scoped ownership
  check.
- `internal/service/paymentservice/service.go:582` - intent ownership check.
- `internal/service/paymentservice/service.go:594` - user-owned payment
  history entrypoint.

Webhook processing still uses `intent.UserID` for refund eligibility and
top-up:

- `internal/service/paymentservice/webhook_processor.go:468` - refund balance
  lookup.
- `internal/service/paymentservice/webhook_processor.go:619` - refund balance
  lookup.
- `internal/service/paymentservice/webhook_processor.go:821` - successful
  top-up grant.

PostgreSQL intent writes and reads are dual-mode:

- `internal/adapter/storage/postgres/payment.go:200` - insert backfills
  `account_id` from `users.account_id`.
- `internal/adapter/storage/postgres/payment.go:315` - history reads
  `user_id OR account_id`.
- `internal/adapter/storage/postgres/payment.go:336` - filters accept
  user/account compatibility.

Target: make `account_id` the authorization key for intents, refund checks and
top-up grants. Reject or repair unbackfilled payment intents before removing
legacy fallback.

### Jobs And Worker

Job orchestration still treats `CreateJobInput.UserID` as the primary owner:

- `internal/service/joborchestrator/orchestrator.go:322` - job creation uses
  input `UserID`.
- `internal/service/joborchestrator/orchestrator.go:360` - billing reserve uses
  input `UserID`.
- `internal/service/joborchestrator/orchestrator.go:457` - active video job
  capacity check uses `UserID`.

The worker still propagates legacy job ownership:

- `internal/worker/worker.go:1228` - assistant facts use `job.UserID`.
- `internal/worker/worker.go:1314` - delivery context uses `job.UserID`.
- `internal/worker/worker.go:1443` - reference artifact validation compares
  `artifact.OwnerUserID` to `job.UserID`.
- `internal/worker/worker.go:2392` - generated artifact save uses
  `job.UserID`.

PostgreSQL jobs are dual-mode:

- `internal/adapter/storage/postgres/job.go:55` - insert backfills
  `account_id`.
- `internal/adapter/storage/postgres/job.go:140` - get by owner accepts
  `user_id OR account_id`.
- `internal/adapter/storage/postgres/job.go:207` - filters accept
  `user_id OR account_id`.
- `internal/adapter/storage/postgres/job.go:274` and
  `internal/adapter/storage/postgres/job.go:289` - active/succeeded counts
  accept `user_id OR account_id`.

Target: add `AccountID` to job input and make capacity limits, billing
reservation, worker validation, delivery ownership and history account-scoped.

### Artifacts

Artifact service and access checks still use legacy owner IDs:

- `internal/service/artifactservice/service.go:155` - storage key includes
  `OwnerUserID`.
- `internal/service/artifactservice/service.go:282` - saved artifacts set
  `OwnerUserID`.
- `internal/adapter/inbound/miniapp/references.go:47` and
  `internal/adapter/inbound/miniapp/references.go:73` - reference checks use
  `OwnerUserID`.
- `internal/adapter/inbound/miniapp/handler.go:1959` - artifact download
  checks `OwnerUserID`.

PostgreSQL artifacts are dual-mode:

- `internal/adapter/storage/postgres/artifact.go:45` - insert backfills
  `owner_account_id`.
- `internal/adapter/storage/postgres/artifact.go:61` - update backfills
  `owner_account_id`.
- `internal/adapter/storage/postgres/artifact.go:91` and
  `internal/adapter/storage/postgres/artifact.go:103` - lookups accept
  `owner_user_id OR owner_account_id`.

Target: require `owner_account_id` for new artifacts, use account ownership for
access checks and add account-equivalent dedupe indexes before removing
`owner_user_id` predicates.

### Conversations

Dialog context still derives conversation ownership from `job.UserID`:

- `internal/service/dialogcontext/service.go:192` and
  `internal/service/dialogcontext/service.go:205` - conversation refs use
  `job.UserID`.
- `internal/service/dialogcontext/service.go:218`,
  `internal/service/dialogcontext/service.go:232` and
  `internal/service/dialogcontext/service.go:268` - conversation rows use
  legacy user ownership.
- `internal/service/dialogcontext/service.go:243` - conversation fetch uses
  `GetByIDForUser`.

PostgreSQL conversation reads are dual-mode:

- `internal/adapter/storage/postgres/conversation.go:32`,
  `internal/adapter/storage/postgres/conversation.go:54`,
  `internal/adapter/storage/postgres/conversation.go:69` and
  `internal/adapter/storage/postgres/conversation.go:86` - reads accept
  `user_id OR account_id`.
- `internal/adapter/storage/postgres/conversation.go:109` - insert backfills
  `account_id`.

Target: add account-native conversation refs and account-scoped uniqueness for
active conversations after duplicate cleanup.

### Referrals

Referral service still names and compares referral ownership by user ID:

- `internal/service/referralservice/service.go:145` - self-referral check uses
  `code.UserID` vs `ReferredUserID`.
- `internal/service/referralservice/service.go:153` and
  `internal/service/referralservice/service.go:154` - relation creation writes
  user owner fields.
- `internal/service/referralservice/service.go:164` and
  `internal/service/referralservice/service.go:202` - referral lookup uses
  `GetReferralByReferredUserID`.
- `internal/service/referralservice/service.go:272` and
  `internal/service/referralservice/service.go:282` - referral rewards grant by
  user owner fields.

PostgreSQL referrals are dual-mode:

- `internal/adapter/storage/postgres/referral.go:30` - code lookup uses
  `user_id OR account_id`.
- `internal/adapter/storage/postgres/referral.go:53`,
  `internal/adapter/storage/postgres/referral.go:78` and
  `internal/adapter/storage/postgres/referral.go:80` - account fields are
  backfilled from users.
- `internal/adapter/storage/postgres/referral.go:98`,
  `internal/adapter/storage/postgres/referral.go:107` and
  `internal/adapter/storage/postgres/referral.go:119` - stats and lookups
  accept user/account compatibility.

Target: move referral code, apply, activate and stats to account owner fields.
Keep external channel identity separate from referral ownership.

### VK Bot And Mini App Call Sites

Both VK Bot and Mini App resolve the current VK identity through the identity
layer, but still pass `user.ID` into business services.

Examples:

- `internal/adapter/inbound/vk/handler.go:948`,
  `internal/adapter/inbound/vk/handler.go:1148`,
  `internal/adapter/inbound/vk/handler.go:1390` and
  `internal/adapter/inbound/vk/handler.go:1423` - job/payment/service inputs
  still use `user.ID`.
- `internal/adapter/inbound/vk/handler.go:2642` and
  `internal/adapter/inbound/vk/handler.go:2659` - referral activation uses
  user IDs.
- `internal/adapter/inbound/vk/menu.go:478` - account stats count jobs by
  legacy user ID.
- `internal/adapter/inbound/miniapp/handler.go:1282`,
  `internal/adapter/inbound/miniapp/handler.go:1383`,
  `internal/adapter/inbound/miniapp/handler.go:1443` and
  `internal/adapter/inbound/miniapp/handler.go:1739` - product actions pass
  user IDs.
- `internal/adapter/inbound/miniapp/handler.go:1552` and
  `internal/adapter/inbound/miniapp/handler.go:1993` - job access checks use
  `Job.UserID`.
- `internal/adapter/inbound/miniapp/handler.go:1573` - balance lookup uses
  `GetAccountByUser`.
- `internal/adapter/inbound/miniapp/handler.go:1800` - payment history lists by
  user/source.

Target: surfaces should resolve identity once and pass canonical `account_id`
into business services. Channel-specific IDs such as `vk_user_id` can remain for
delivery, webhook idempotency and VK-only rate limiting.

### Commands And Events

`CommandRepository.ListByUser` is still user-owned at
`internal/domain/repositories.go:255`. Command rows are mostly inbound event
records, so they need a product decision:

- if commands are part of cross-channel account history, add and backfill
  `account_id`;
- if commands are only channel-local audit/inbox records, document them as
  channel identity, not business ownership.

### Schema And Migration Dependencies

Important compatibility migrations:

- `migrations/000037_account_business_dual_write.up.sql` adds additive account
  owner columns and compatibility indexes.
- `migrations/000008_conversation_sources.up.sql` keeps active conversation
  uniqueness user-scoped.
- `migrations/000018_media_lifecycle.up.sql` keeps media input-reference dedupe
  owner-user scoped.
- `migrations/000014_referral_events.up.sql` keeps referral events user-id only.

Target before account-only:

1. Validate all account owner fields are backfilled.
2. Add account-scoped uniqueness and dedupe indexes where needed.
3. Make account owner columns required for account-owned rows.
4. Keep legacy columns as compatibility metadata until all services stop using
   them for authorization and ownership.

## Priority For The Next Rollout

### P0 - Required Before Account ID Only

- Add account-native service APIs for billing, payments, jobs, artifacts and
  conversations.
- Update VK Bot and Mini App to pass canonical `account_id` to business
  services.
- Switch payment intent authorization, webhook top-up and refund checks to
  `account_id`.
- Switch job creation, worker validation and credit reservation to `account_id`.
- Switch artifact access checks to `owner_account_id`.
- Add account-native repository methods instead of overloading `*ByUser`.

### P1 - Needed Before Multi-UI Account Release

- Move referral code/apply/activate/stats to account owner fields.
- Move dialog context and active conversation uniqueness to account scope.
- Add account ownership decision for command/inbound event history.
- Add account filters to admin/operator consoles and keep safe user/channel refs
  as display metadata only.

### P2 - Final Cleanup

- Remove `OR user_id` predicates from account-owned repository reads.
- Stop backfilling new rows from `users.account_id`; require explicit
  account ownership at service boundaries.
- Tighten schema constraints and indexes after production backfill validation.
- Retire or rename legacy user-owned methods.

## Verification Matrix

Before removing compatibility fallback, tests should prove:

- existing VK users keep the same balance through `account_id`;
- existing payment intents are visible by `account_id`;
- existing jobs and artifacts are visible by `account_id`;
- a newly created job writes `account_id`;
- a newly created payment intent writes `account_id`;
- a generated artifact writes `owner_account_id`;
- Mini App cannot read a job or artifact from another account;
- VK Bot and Mini App see the same balance for the same account;
- payment webhook top-up grants credits to the canonical account owner only;
- refund eligibility uses canonical account balance;
- referral apply/activate rewards are idempotent by account ownership;
- legacy user fallback remains only where explicitly marked as rollout
  compatibility.
