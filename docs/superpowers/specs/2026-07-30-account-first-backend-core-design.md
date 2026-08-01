# Account-First Backend Core Independence

Status: accepted

Date: 2026-07-30

## Goal

Complete the migration from a VK-originated backend to a channel-neutral core
without breaking the existing VK Bot or VK Mini App. A browser platform,
future mobile clients, and future channels must use the same Accounts, jobs,
pricing, billing ledger, payments, artifacts, moderation, and provider
routing without fabricating a VK identity or a second backend.

## Scope

This is a backend-core migration. It includes canonical ownership, session
authentication, channel context, delivery, and safe schema rollout. It does
not remove the VK Bot, remove Mini App support, replace providers, alter
prices, migrate user balances, or introduce a separate web business backend.

VK remains a supported channel adapter. Its callback signature, launch
parameter verification, peer/message references, anti-spam keys, and outbound
delivery client stay at the adapter boundary.

## Current State

The Account Layer and most business ownership have already begun the migration:

- accounts, identities, session storage, and credentials exist;
- jobs, payment intents, artifacts, conversations, billing accounts,
  reservations, and ledger entries have account ownership columns;
- Job orchestration, billing, payments, and artifact access already prefer
  AccountID for new account-aware callers;
- VK Bot and Mini App pass both a legacy user reference and EffectiveAccountID
  during the compatibility rollout.

The migration is incomplete because the core still contains VK-specific
transport and legacy owner requirements:

- protected account HTTP routes still authenticate through VK launch
  parameters;
- persisted sessions rotate a hashed refresh secret but have no persisted,
  verifiable short-lived access secret for protected generic requests;
- jobs, payment intents, artifacts, credit accounts, commands, deliveries,
  inbound events, and parts of conversation/referral storage still require a
  legacy user or VK field;
- worker delivery is currently a VK-specific action, so a web-created Job
  would fall through to a VK send path;
- generic payment service includes VK payment-message attachment behavior;
- referrals remain principally user-owned even though account owner columns
  are available.

## Target Architecture

~~~text
VK Bot adapter ──────────────┐
VK Mini App adapter ─────────┼──> RequestPrincipal(AccountID)
Web adapter ─────────────────┘             |
                                            v
      Account-first core: jobs, projects, billing, payments,
      artifacts, referrals, moderation, provider routing, audit
                                            |
                                            v
         optional ChannelContext and DeliveryTarget ports
                     |                         |
              VK delivery adapter       web result publication
~~~

### Canonical Principal

Every authenticated request produces a RequestPrincipal with:

- AccountID as the sole authorization and business ownership identity;
- SessionID and authentication method for audit/rate-limit decisions;
- optional channel metadata kept outside authorization decisions.

No core service receives a VK launch parameter, VK user id, or HTTP request in
order to identify an owner. A VK adapter validates its input and resolves the
same RequestPrincipal as a browser session or another future channel.

### Account-First Ownership

New Jobs, Projects, Artifacts, PaymentIntents, ledger entries, referral
records, conversations, and delivery/publication records are owned by
AccountID. Legacy user columns may remain as nullable provenance during the
compatibility window but cannot be required for a new account-native write.

Credit account primary keys remain internal credit-account identifiers.
owner_account_id, not ledger_entries.account_id, remains the canonical owner
of money. The migration must not conflate those two meanings.

### Channel Context and Delivery

Transport is separate from ownership. ChannelContext represents optional
source, recipient, and thread information using a channel name and opaque
references. VK peer ids live only in the VK adapter mapping.

DeliveryTarget is optional:

- a VK job has a VK target and is delivered through the existing VK adapter;
- a web job has no external push target. Its result becomes usable when its
  owner-checked Artifact is ready; the browser retrieves it through the shared
  Artifact API;
- terminal Job charging/release follows the existing backend ledger policy and
  never depends on browser confirmation or a fake VK delivery.

Generic payment creation does not attach a VK message. VK payment-status
notification is a separate adapter concern.

### Neutral Sessions

Account sessions use an opaque short-lived access secret and a rotating
long-lived refresh secret, both stored only as hashes. A shared session
authenticator validates the access secret and returns RequestPrincipal.

Browser adapters transport these secrets only in Secure, HttpOnly, SameSite
cookies and enforce Origin plus CSRF checks for writes. Existing non-browser
surfaces keep their existing adapter authentication until they migrate to the
same principal boundary.

## Delivery Sequence

The work is delivered in safe, independently verifiable stages.

1. Add RequestPrincipal and verifiable neutral access sessions without changing
   the VK Bot or Mini App behavior.
2. Add account-native write/read contracts and owner-scoped repository
   methods. Preserve dual reads and legacy provenance for existing rows.
3. Move referral/reward ownership to AccountID and prove financial
   conservation under retry.
4. Replace core VK peer/source fields with generic ChannelContext and route
   channel mapping through adapters.
5. Replace the VK-specific delivery worker contract with a generic delivery or
   web-publication port. Retain the existing VK delivery adapter behind it.
6. Backfill, validate, and observe all ownership columns. Only then make
   canonical ownership non-null and remove legacy fallback reads in a later,
   backup-first release.

## Data Migration Policy

- Use additive forward migrations. Never drop a legacy owner column in the
  first account-first release.
- Backfill in bounded batches with reconciliation reports, not one opaque
  table-wide update.
- Add new foreign keys and constraints as NOT VALID where supported, reconcile
  existing data, then validate them.
- Before any final nullability or ownership enforcement, verify:
  account/legacy owner agreement, no cross-owner Job/Artifact/Reservation
  links, one credit account per owner/currency, and ledger sum equals cached
  balance.
- Final cutover migrations are forward-fix only and require a verified backup.
  A down migration must never discard canonical account ownership data.
- The current migration runner wraps each file in a transaction. Do not hide
  concurrent index creation in a normal migration; schedule it through a
  reviewed migration-runner capability if it is required for a large table.

## Security and Compatibility Invariants

- Existing VK Bot and Mini App retain their current behavior and access to the
  same account-owned history throughout the migration.
- No new web/mobile account requires a fake VK id or a synthetic VK peer.
- No API receives a client-supplied AccountID as authority.
- No session credential, raw provider payload, private artifact URL, payment
  secret, or PII enters logs, analytics, URLs, or browser storage.
- Owner checks occur server-side for every Job, Artifact, Project, ledger
  record, referral, and payment intent.
- Idempotency applies to every paid Job and payment intent before any balance
  mutation.
- A payment redirect never credits a balance; existing verified webhook and
  idempotent ledger behavior remain the only credit path.

## Definition of Done

The core is channel-neutral when:

1. a non-VK account can authenticate with a verifiable account session;
2. it can create and own a Job, payment intent, balance, Artifact, Project,
   and referral relationship without a user_id or VK peer requirement;
3. a web Job reaches usable Artifact state without attempting a VK API call;
4. the existing VK Bot and Mini App regression suites pass against the same
   account-owned business history;
5. domain/service ownership APIs use AccountID and generic channel context,
   while VK IDs remain confined to adapters and compatibility provenance;
6. data reconciliation and full migration-chain tests pass before final
   removal of fallback reads.
