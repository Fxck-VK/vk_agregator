# Account Identity Contract

Status: active target architecture
Updated: 2026-07-02

This document defines the target account identity layer for all current and
future user surfaces: VK Bot, VK Mini App, Telegram Bot, Web and Mobile.

## Goal

One human or organization should have one internal `account_id`. Messenger IDs,
VK Mini App launch identities, email, phone, Google, Apple and future login
methods are only ways to access that account.

```text
VK / Telegram / Web / Mobile / Mini App
        -> IdentityResolver
        -> account_id
        -> Billing / Jobs / Artifacts / Referrals / History
```

## Core Contract

`account_id` is the business owner of:

- balance and billing ledger;
- payment intents, refunds and subscriptions;
- jobs and workflow history;
- artifacts and storage access;
- conversations and summaries;
- referrals and rewards;
- account-level risk, status, roles and preferences.

External identities are login or channel bindings only:

- `vk_user_id`;
- `telegram_user_id`;
- email;
- phone;
- Google subject;
- Apple subject;
- username/password credential;
- future OAuth or enterprise identity.

Business logic must not treat a messenger/user-provider ID as the owner of
money, jobs or artifacts.

## Target Data Model

```text
accounts
  id
  status: active | blocked | deleted
  role: user | moderator | admin | operator
  account_type: personal | business
  locale
  timezone
  risk_level
  created_at
  updated_at

account_identities
  id
  account_id
  provider: vk | telegram | google | apple | email | phone | password
  external_id
  normalized_id
  verified_at
  last_used_at
  created_at
  updated_at

account_sessions
  id
  account_id
  identity_id
  refresh_token_hash
  device_id
  ip_hash
  user_agent_hash
  expires_at
  revoked_at
  created_at

account_credentials
  id
  account_id
  credential_type: password | otp | passkey
  secret_hash
  changed_at
  created_at

account_links_audit
  id
  account_id
  actor_account_id
  action: linked | unlinked | login | merge_requested | merge_completed
  provider
  identity_id
  created_at
```

Required indexes:

```sql
UNIQUE (provider, normalized_id)
INDEX account_identities_account_id_idx (account_id)
INDEX account_sessions_account_expires_idx (account_id, expires_at DESC)
INDEX account_links_audit_account_created_idx (account_id, created_at DESC)
```

All large business tables should eventually have `account_id, created_at`
indexes for cursor-based access:

```text
jobs(account_id, created_at DESC, id DESC)
ledger_entries(account_id, created_at DESC, id DESC)
payment_intents(account_id, created_at DESC, id DESC)
artifacts(owner_account_id, created_at DESC, id DESC)
conversations(account_id, source, updated_at DESC, id DESC)
referrals(referrer_account_id, created_at DESC, id DESC)
```

## IdentityResolver

All inbound surfaces must resolve identity before touching core business logic.

```text
Resolve(provider, external_id) -> account_id
ResolveOrCreate(provider, external_id) -> account_id
LinkIdentity(account_id, provider, external_id)
UnlinkIdentity(account_id, identity_id)
```

Rules:

- `ResolveOrCreate` may create an implicit account for first-touch channels
  such as VK Bot or Telegram Bot.
- `LinkIdentity` requires proof that the current user controls the target
  identity.
- `UnlinkIdentity` must not leave an account without at least one usable login
  method unless the account is explicitly marked as channel-only.
- Account merge must be explicit, audited and conservative.

## Account Boundary

`internal/service/accountservice` is the product-facing account boundary above
`IdentityResolver` and `accountauth`.

Responsibilities:

- return safe account profile DTOs;
- list linked identities without `external_id` or `normalized_id`;
- expose only masked labels for email/phone identities;
- route link/unlink through the verified account auth layer;
- keep ownership checks, rate limits and audit writes outside VK Bot, Mini App
  and future Web/Mobile handlers.

Product surfaces must not read `account_identities` directly and must not build
identity DTOs from raw repository rows.

## Account HTTP API

`cmd/api` exposes the account boundary under `/account/*`.

Current endpoints:

```text
GET /account/me
GET /account/identities
POST /account/identities/link
POST /account/identities/email/request-code
POST /account/identities/email/verify
POST /account/identities/phone/request-otp
POST /account/identities/phone/verify
POST /account/oauth/login
POST /account/oauth/link
DELETE /account/identities/{id}
POST /account/identities/{id}/unlink
```

Authentication currently uses the same trusted VK launch identity bridge as the
Mini App: verified launch params in production, and `X-VK-User-ID` only when
the app secret is empty for local/dev tests.

Response rules:

- return `account_id` and safe identity refs only;
- never return raw `external_id` or `normalized_id`;
- mask email and phone labels;
- use provider labels such as `VK`, `Telegram`, `Google`, `Apple`;
- do not echo raw identity input in errors.

`unlink` is active for identities owned by the current account and routes
through `accountauth`.

`link` is intentionally fail-closed until method-specific verification exists.
The HTTP endpoint must not accept client-supplied JSON such as `verified=true`
as proof of email, phone or OAuth ownership. Email/password, phone OTP, OAuth,
Telegram and VK ID verifier adapters should validate the proof first and then
call `AccountService.LinkVerifiedIdentity`.

Email linking uses a method-specific verifier flow:

```text
POST /account/identities/email/request-code
  -> validate current account session
  -> normalize email
  -> rate-limit by account_id + hashed email
  -> store hashed challenge in Redis with TTL
  -> send code through the configured email sender

POST /account/identities/email/verify
  -> validate current account session
  -> normalize email
  -> rate-limit verification attempts
  -> compare submitted code with stored HMAC hash
  -> delete challenge
  -> AccountService.LinkVerifiedIdentity(current account_id, email)
```

Rules:

- challenge storage must contain only hashed email/code data and TTL;
- request and verify rate-limit keys must not contain raw email values;
- responses must not echo raw email values or verification codes;
- expired challenges return a controlled failure and are deleted;
- the default runtime sender is fail-closed until real delivery is explicitly
  configured with `ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp`;
- the SMTP adapter is used only by the account link service, sends plaintext
  verification codes through the configured mail relay, and must not log codes,
  passwords or raw recipient values.

Phone linking uses the same method-specific verifier boundary:

```text
POST /account/identities/phone/request-otp
  -> validate current account session
  -> normalize phone
  -> rate-limit by account_id + hashed phone
  -> store hashed challenge in Redis with TTL
  -> send OTP through the configured SMS/phone sender

POST /account/identities/phone/verify
  -> validate current account session
  -> normalize phone
  -> rate-limit verification attempts
  -> compare submitted OTP with stored HMAC hash
  -> delete challenge
  -> AccountService.LinkVerifiedIdentity(current account_id, phone)
```

Rules:

- challenge storage must contain only hashed phone/code data and TTL;
- request and verify rate-limit keys must not contain raw phone values;
- responses must show only masked phone labels and must not echo OTP codes;
- expired challenges return a controlled failure and are deleted;
- the default runtime sender is fail-closed until real delivery is explicitly
  configured with `ACCOUNT_PHONE_DELIVERY_PROVIDER=http`;
- the HTTP phone adapter is a generic SMS/OTP provider boundary. It accepts a
  templated request body with `{{phone}}` and `{{code}}`, treats non-2xx
  responses as delivery failures, and must not log request bodies, response
  bodies or raw phone values.

OAuth and platform login use provider-specific adapters under
`internal/adapter/accountoauth`:

```text
POST /account/oauth/login
  -> verify provider assertion
  -> accountauth.ResolveOrCreate(verified login)
  -> issue account session

POST /account/oauth/link
  -> authenticate current account
  -> verify provider assertion
  -> AccountService.LinkVerifiedIdentity(current account_id, verified login)
```

Rules:

- Google, Apple and VK ID are OIDC/JWKS adapters and must validate signature,
  issuer, audience and expiry before returning a login assertion;
- Google and Apple identities use stable `sub`, never display email or name;
- VK ID and Telegram identities use verified numeric platform identity;
- Telegram supports OIDC ID tokens and a legacy HMAC auth-data path for
  compatibility;
- OAuth endpoints must not echo `id_token`, raw auth data, device info or
  provider subject in responses/errors;
- unconfigured providers return controlled unavailable errors and remain
  fail-closed.

## Login Methods

Every login method enters the system through the same account identity layer.
Method-specific adapters prove identity ownership first, then pass only a
verified login assertion to `internal/service/accountauth`.

Supported target methods:

- email/password;
- phone OTP;
- Google OAuth subject;
- Apple OAuth subject;
- Telegram user id;
- VK ID / VK user id.

```text
email/password adapter
phone OTP adapter
Google / Apple adapter
Telegram / VK adapter
        -> verify method-specific proof
        -> accountauth.ResolveOrCreate(verified login)
        -> IdentityResolver.ResolveOrCreate(provider, external_id)
        -> account_id
```

Rules:

- `accountauth` must reject unverified assertions and must not verify passwords,
  OTP codes or OAuth tokens itself;
- email/password maps to the `email` identity provider after password proof,
  while password verifiers live in `account_credentials`;
- phone OTP maps to the `phone` identity provider only after OTP proof;
- Google and Apple map by stable provider subject, not display email/name;
- Telegram and VK map by verified numeric platform user id;
- all methods create or find `account_id` through `IdentityResolver`, never
  through UI-specific repositories.

## Implicit Account Flow

A user does not need to register before paying or generating.

```text
VK event
  -> verify VK callback
  -> extract vk_user_id
  -> ResolveOrCreate("vk", vk_user_id)
  -> account_id
  -> create jobs / ledger entries / payment intents
```

The user-visible UX can still be "start using the bot". Internally the system
already owns a durable `account_id`.

Later, if the same user creates login/password or links email:

```text
existing account_id with vk identity
  -> confirm email/password
  -> LinkIdentity(account_id, "email", normalized_email)
```

No balance, payment, artifact or job transfer is needed because those records
already belong to the same account.

## VK Bot Registration After Use

The VK Bot supports post-use identity linking without changing the user's
business owner:

```text
existing VK user
  -> account menu
  -> "Привязать email/телефон"
  -> user sends email or phone as a normal VK message
  -> bot stores the contact only as pending dialog state
  -> user presses "Подтвердить привязку"
  -> IdentityResolver.LinkIdentity(current account_id, provider, external_id)
```

Rules:

- the current VK callback identity is resolved first through
  `IdentityResolver`, so the linked email/phone is attached to the same
  `account_id`;
- balance, payments, jobs, artifacts, referrals and history remain attached to
  that account and are not transferred or copied;
- entering an email/phone does not create a Job and does not mutate billing;
- the confirmation message masks the contact value and does not expose raw PII
  back in full;
- invalid contact input keeps the user in the linking dialog and does not fall
  through to AI generation.

This flow confirms intent inside the already verified VK session. Independent
email/SMS ownership verification is still required before email or phone can be
used as a standalone login method.

## Surface Responsibilities

VK Bot, Telegram Bot, VK Mini App, Web and Mobile are UI surfaces. They must:

- verify the inbound platform token/signature/session;
- pass a provider/external identity to `IdentityResolver`;
- use the returned `account_id` for backend calls;
- never trust client-supplied balance, role, status, price or moderation state.

Core services must:

- accept `account_id` as owner identity;
- keep provider/channel-specific fields as metadata or delivery context only;
- remain independent of which UI surface initiated the action.

## Current Repository State

The current runtime implementation is in the VK compatibility phase:

- `users.vk_user_id` is the unique user identity;
- VK Bot and VK Mini App share the same backend user for the same VK ID;
- `internal/service/identityresolver` resolves `provider=vk` identities and
  creates/links the legacy `users` bridge when needed;
- `domain.User` exposes `account_id` as the compatibility bridge returned by
  `IdentityResolver`;
- PostgreSQL user storage reads and writes `users.account_id`;
- `cmd/api` wires a PostgreSQL-backed `IdentityResolver` through SharedCore;
- VK Bot and VK Mini App resolve the current user through `IdentityResolver`
  before touching jobs, billing, payments, conversations or artifacts;
- conversations, jobs, payments and referrals already carry `source` metadata
  such as `vk_bot`, `miniapp` or `vk_miniapp`;
- `accounts`, `account_identities`, `account_sessions`,
  `account_credentials` and `account_links_audit` exist as schema foundation;
- existing `users.vk_user_id` rows are backfilled to `account_identities` with
  `provider=vk`;
- business tables use additive account ownership columns in compatibility mode:
  `jobs.account_id`, `payment_intents.account_id`,
  `artifacts.owner_account_id`, `conversations.account_id`,
  referral account owner fields, and `owner_account_id` on credit
  accounts/reservations/ledger entries;
- repository reads accept legacy `user_id` and account ownership during rollout,
  while new writes backfill account ownership from `users.account_id` when the
  caller still passes only a legacy user id;
- VK Bot can link email or phone identities to the current account after the
  user has already used the bot, with explicit VK-button confirmation and no
  job/billing side effects;
- `internal/service/accountauth` provides the shared login-method boundary for
  verified email/password, phone OTP, Google, Apple, Telegram and VK ID
  assertions, while `internal/adapter/accountoauth` verifies OAuth/platform
  assertions before they enter that boundary;
- `subscriptions` is reserved in the target model, but no current
  subscriptions table/domain model exists yet.

The target migration is additive:

1. Add `accounts` and `account_identities`. Done in migration `000035`.
2. Backfill one account per existing `users.vk_user_id`. Done in migration
   `000036`.
3. Add `provider=vk` identities for existing users. Done in migration
   `000036`.
4. Introduce `IdentityResolver`. Done.
5. Route VK Bot and Mini App through the resolver while preserving current UX.
   Done for current VK identity resolution; returned legacy users carry
   `account_id`.
6. Add account ownership columns to business tables and dual-write during
   rollout. Done in migration `000037` for current business tables.
7. Switch business services from `user_id` ownership to `account_id`.
8. Keep compatibility views/helpers until old `user_id` ownership is removed.

## Security Invariants

- Do not auto-merge accounts from unverified email, display name, avatar or
  device fingerprint.
- Linking a new identity requires proof of control over both the current
  account and the target identity.
- Session tokens and refresh tokens are stored only as hashes.
- Passwords and OTP secrets are stored only as strong hashes or verifier
  material.
- Email/password credentials can be created only for an email identity that is
  already verified and linked to the same account.
- Password reset requires the same verified email code path and revokes active
  account sessions after the credential is rotated.
- Link, unlink and merge actions are always audited.
- Account and identity APIs must not expose raw provider tokens, launch params,
  full phone/email values or private artifact URLs.
- Login, link and unlink flows must be rate-limited by a stable hashed key, not
  by raw email, phone, token or platform id.
- Public account identity DTOs must use safe refs and must not include
  `external_id` or `normalized_id`.
- Account merge must stay disabled unless a dedicated confirmed and audited
  merge flow exists.

## Conflict And Merge Rules

Identity conflicts must be treated as account-merge candidates, not as a reason
to silently move ownership.

Rules:

- linking an identity that already belongs to the same `account_id` is
  idempotent and may return the existing safe identity;
- linking email, phone, Google, Apple, Telegram, VK ID or VK identity that
  already belongs to another `account_id` must return a controlled conflict
  (`ErrAccountMergeRequiresConfirmation` at the account auth boundary);
- the system must not copy, move or merge balance, ledger entries, jobs,
  artifacts, payment intents, referrals or conversations during a conflicting
  link attempt;
- resolving an already known VK/platform identity must return the existing
  account instead of creating a second account;
- account merge needs a separate flow that proves control over both accounts,
  shows the consequences, records audit and receives explicit confirmation;
- financial merge rules are conservative: ledger history remains append-only,
  payment/refund ownership is not rewritten implicitly, and operator review is
  required before any future money-related merge path;
- until that dedicated flow exists, `accountauth.MergeAccounts` stays blocked
  and returns a non-success error even when the caller passes `confirmed=true`;
- HTTP account endpoints map merge-required conflicts to `409 Conflict` and do
  not reveal which account owns the conflicting identity.

## Security And Audit Foundation

Runtime code now includes explicit guard rails for the future account auth
surface:

- `AccountSession` stores `refresh_token_hash`, never a raw refresh token;
- `AccountCredential` stores `secret_hash`, never a raw password, OTP secret or
  passkey secret;
- password login uses a salted PBKDF2-SHA256 verifier and never creates
  accounts without a pre-existing verified email identity;
- password reset reuses the verified email-code flow, rotates the credential,
  writes audit and revokes active sessions when session storage is wired;
- `AccountIdentity.SafeRef()` returns a PII-free identity reference for APIs and
  logs;
- `accountauth` accepts only verified login assertions and rejects unverified
  email/password, phone OTP, OAuth and platform login claims;
- `accountauth.LinkVerifiedIdentity` requires the actor account to match the
  account being modified until a separate operator flow exists;
- `accountauth.LinkVerifiedIdentity` converts conflicts for already-owned
  identities into `ErrAccountMergeRequiresConfirmation` so user-facing flows do
  not accidentally imply an automatic merge;
- `accountauth.UnlinkIdentity` applies the same ownership check;
- `accountauth.MergeAccounts` intentionally returns
  `ErrAccountMergeRequiresConfirmation` unless a future confirmed merge flow is
  implemented;
- login/link/unlink/password rate-limit keys are hashed before they reach the
  limiter, and `cmd/api` wires account auth to a Redis-backed limiter so the
  quota is shared across API instances;
- PostgreSQL `AccountIdentityRepository` writes `account_links_audit` records
  for account/identity link and unlink mutations, and the in-memory repository
  mirrors that behavior for tests and DEV runs.

## Rollout Checks

The account rollout must stay safe for existing VK users and for users who
register after they already paid or generated content. The current regression
suite covers these invariants:

- an existing VK user keeps the same canonical `account_id`;
- existing balance remains visible by `account_id`;
- existing payment intents remain visible by `account_id`;
- existing jobs remain visible by `account_id`;
- a new VK user receives an implicit account on first resolve;
- email linking attaches to the existing account and is idempotent;
- attempting to link an already bound identity to another account returns a
  merge-required conflict and does not change balance/history;
- unlink refuses to remove the last usable identity from an account;
- link/unlink audit rows stay PII-free;
- login, link, unlink and password set/reset rate limits are enforced with
  hashed keys;
- password reset revokes previously active web/mobile sessions;
- billing ledger entries carry `owner_account_id` and are checked through the
  canonical account owner during rollout.
- E2E smoke covers the key post-use registration path: VK user receives an
  implicit account, receives a payment top-up, creates jobs, links an email,
  logs in through email/password web flow, and still sees the same balance,
  jobs and payment history through the same `account_id`.

## Non-Goals For Current Identity Rollout

The current identity rollout does not implement account merge UI or final
business table rewrites. It introduces the resolver/auth boundaries, end-user
sessions, password login and OAuth/platform verifier adapters while keeping
legacy `user_id` owner checks intact until `account_id` dual-read/dual-write is
fully retired.
