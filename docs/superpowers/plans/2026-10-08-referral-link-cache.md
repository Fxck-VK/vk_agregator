# Referral link cache implementation plan

**Approved design:** The user approved caching the stable referral code for the current account, refreshing statistics independently, and clearing the cache on logout/account changes. Keep the cache in the current browser tab's memory only. A full reload fetches the code again; browser storage and server-rendered shared caches are excluded.

**Goal:** Reopening the referral tab immediately shows its personal link without reading the code again on the server.

**Architecture:** Keep the existing authenticated summary route for the first load. Add authenticated `GET /web/v1/referrals/stats`, which reads counters without looking up or generating a code. Store only the validated code with the confirmed browser account and session revision; the session abort signal clears it. React reads the cache with `useSyncExternalStore` and a null server snapshot. Statistics remain uncached and refresh on each opening. Late responses after account/session changes cannot update either UI or cache.

**Constraints:** Existing cookie-session authorization, account ownership, no-store responses, disabled rewards, localization and preview isolation remain. No dependencies, migrations, commits or deployment in this change.

## Task 1: Authenticated statistics-only read

Files: `internal/service/webreferralservice/service{,_test}.go`, `internal/adapter/inbound/websession/referrals{,_test}.go`, `internal/adapter/inbound/websession/handler.go`.

- [x] Add a regression test for `/web/v1/referrals/stats`: anonymous/forged identity is rejected; an authenticated request uses its principal, returns real counters and no code, and has no-store headers.
- [x] Run `go test ./internal/adapter/inbound/websession -run Referral -count=1` and observe the missing route failure.
- [x] Add `Statistics(context.Context, uuid.UUID) (Statistics, error)` to the referral service and handler boundary. Project only counters from the repository summary; force rewards disabled. Reject missing repository and nil account. Add a service test proving no code read or allocation occurs.
- [x] Run affected Go tests and vet.

## Task 2: Account-scoped cached link and independent statistics states

Files: `web/platform/src/features/referrals/referral-code-cache.ts`, `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram{,.test}.tsx`, `web/platform/src/features/account/ProfileWorkspace/ProfileWorkspace.tsx`, `web/platform/src/i18n/{ru,en}.ts`, `web/platform/docs/{ui-index,ui-catalog}.md`, `docs/runbooks/WEB_REFERRALS.md`.

- [x] Test reopening with a pending statistics response: the link appears immediately and `/stats` is requested. Test statistics failure/retry preserves the link without displaying invented counters. Test logout/same-account relogin, account switching, stale responses and preview do not reuse/cache another session's code.
- [x] Run focused Vitest tests and observe the caching failures before implementation.
- [x] Add a single-entry memory cache with `readReferralCode(accountId): string | null`, `subscribeReferralCode(callback): () => void`, and `rememberReferralCode(accountId, code, session): boolean`. Validate the session revision/account before writing, remove abort listeners when replacing entries, clear and notify on session abort. No browser storage.
- [x] Pass confirmed `profile.account_id` into the referral component. Use a null SSR cache snapshot, first-load summary or later `/stats`, independent link/statistics loading/error states and existing retry/copy controls. Validate responses and ignore aborted or obsolete generations.
- [x] Document cache scope, reset conditions and the statistics route. Run focused Vitest, frontend lint/typecheck/build, documentation validation and `git diff --check`. Inspect final diff/status and leave changes ready for the user's next push request.

## Verified result

- Related frontend suite: 19 files, 125 tests passed, including cached reopening, statistics failure/retry, logout, account changes, stale responses, SSR and preview isolation.
- Go referral service, web-session handler, API bootstrap and API command tests passed; affected Go vet passed.
- Frontend lint/typecheck, production build and packaging passed. Initial sandboxed build could not create the sharp junction; the same build passed with filesystem permissions, without a source/configuration change.
- Asset/Next lint-contract checks, documentation validation and diff whitespace check passed.
- Changes remain uncommitted and undeployed.

## Rechecked on 2026-10-11

The cache implementation was already present when the user approved this optimization again. Added an integration regression using the real `ProfileWorkspace` tabs: opening the referral tab, returning to overview and reopening immediately restores the invitation; only `/stats` is read on return and retry. Copying works during the delayed statistics read and remains available after a 503. The focused profile/cache suite passed 21 tests across 3 files. Production behavior did not need another implementation change.
