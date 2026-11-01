# Profile Billing History Implementation Plan

> For agentic workers: use subagent-driven-development for bounded implementation and independent review. No commits or publication are authorized.

**Goal:** replace the profile billing placeholder with a private, paginated history of purchases, star movements and refunds.

**Architecture:** one account-native read projection combines immutable ledger movements, payment intents and authenticated refund facts. The service converts historical denominations, normalizes statuses and emits safe DTOs with owner-bound keyset cursors. The existing profile section loads an account-keyed client component; preview remains read-only and explicitly labelled.

**Tech Stack:** Go, PostgreSQL/pgx, existing cookie-authenticated web API/BFF, React/Next.js and shared CSS/UI primitives.

## Global Constraints

- Work only in the active checkout, reuse localhost:7159; preserve all current account-ID/checkout/referral changes.
- Read-only feature: no provider calls, ledger/payment mutations, paid tests, commit, push or deploy.
- Expose only safe selected fields; never raw metadata/reasons, provider IDs, contacts, prompts or private URLs.
- Exact canonical account ownership, no legacy fallback. Pending reservations/zero releases are not final charges.
- Purchases use stored prices and actual matching grants, never today's tariff. Each payment grant appears once.
- Keyset pagination ordered by immutable created_at/source/id; bounded pages and owner-bound cursors; no offset or per-row queries.
- Original profile layout and other cards stay intact. Verify affected profile at 360,390,768,1024,1440,1920,2560x1440,3840x2160 CSS px.

## Tasks

- [x] Add a typed safe history read contract and PostgreSQL projection with tests for owner isolation, tied timestamps, denomination, payment/grant dedup, reserves, refunds and deleted/missing jobs.
- [x] Add cursor/service DTO normalization, cookie-authenticated no-store route and explicit API bootstrap wiring; test forged owners/cursors, bounds and safe serialization.
- [x] Replace the profile placeholder using existing StateNotice/Button/CreditAmount, validated transport, initial/page retry and account/session cancellation; test races, empty/error, pagination and read-only preview.
- [x] Update UI/architecture documentation; verify database integration, focused Go/frontend/vet/lint/type/build/security checks, all eight browser widths and independent review.

## Display semantics

Payment order rows show their stored RUB amount, current status and actually committed granted stars; pending/failed orders have no invented star credit. The positive grant is excluded from the ledger branch only when it belongs to that same canonical owner's payment. Ledger rows show actual committed nonzero star deltas, including generation capture, job refunds, bonuses, expiry and adjustments. Manual refund requests and authenticated external refund facts are shown separately as money-return records, deduplicated by owned provider-refund binding; associated ledger corrections remain explicit star movements. Unknown adjustment reasons use a generic label. Purchase/refund dates are their immutable record creation/verification timestamps, not mutable updated_at.

## Verified locally

Eight PostgreSQL-suite scenarios passed in a separate temporary database migrated through 000066, including actual repository/service pagination, historical denominations, refund proof precedence, ownership and read-only transactions. Five focused Go package suites and six-package vet passed outside Windows isolation. 166 frontend tests across 17 files, scoped ESLint, TypeScript, production platform build, packaging, actionlint, docs/infrastructure validation and fresh dependency checks passed. Final browser matrix: 8 actual profile-preview cases plus 48 labelled synthetic populated/loading/error/empty/page-error/maximum-length cases at all required widths; no page errors or actual history requests in preview. Independent backend and whole-feature review has no remaining findings after fixing stale manual-refund precedence. Start+ naming and maximum-safe minor-unit precision have regressions.

Evidence: `.cache/billing-history-20261031/` and `.cache/responsive-qa/2026-10-31-billing-history/`. Browser populated states are synthetic; actual account-owned SQL/service behavior is covered separately by database tests. No production-scale load benchmark, paid/provider call, application-data mutation, commit, push or deployment was performed.
