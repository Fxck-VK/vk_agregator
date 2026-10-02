# Workspace read recovery

User approved the design in chat on 2026-09-29. Implementation is local; publishing requires a separate push/deploy request.

**Goal:** Keep loaded conversations usable through transient failures and recover expired access sessions without duplicating writes.

**Architecture:** A typed read error contract feeds account-scoped resources with deduplication, bounded fast retries and visible/online recovery. Browser session refresh is serialized across tabs and shared with initial session restoration. Persist only the last conversation-list metadata for a server-confirmed account, never credentials, messages or media. Backend distinguishes authentication rejection from infrastructure failure. Sidebar reuses Skeleton/StateNotice and preserves its scroll region.

**Constraints:** No new dependencies; no automatic replay of writes; no public private-data cache; no secrets/PII/query strings in diagnostics; preserve existing DEV cookie-gate changes. No commit/push in this task.

## Execution and verification

- [x] Typed failures and backend status mapping: add failing tests for expired/revoked sessions versus repository errors; return 401 versus 503; attach safe request correlation and failure stage.
- [x] Browser recovery: test concurrent expired reads, cross-tab serialization, failed refresh, cancellation and non-replayed POST. Share the coordinator with SessionRefresh; classify only confirmed account expiry as a login requirement.
- [x] Resource recovery: fake-timer tests for deduplication, retry delays, terminal failures, hidden/offline pause, disposal and preserved snapshots. Integrate balance and conversation loaders; retain typed catalog errors for background recovery.
- [x] Conversation persistence: test reload restoration, TTL, malformed/blocked storage, account isolation, logout clearing and late requests. Keep local metadata subordinate to server authorization.
- [x] Sidebar: test initial skeleton/error, quiet cached updates, successful empty list and preserved rows. Check layout in a browser when available.
- [x] Revalidate existing DEV access gate; update preloading/UI/security documentation; run frontend tests, typecheck/lint/build and backend tests/vet. Review the final diff and report any unavailable live checks.

Acceptance scenarios: an open workspace outlives the 15-minute access token; two tabs refresh together; offline/503 recovers without focus change; reload restores only the active account's metadata; logout removes private data; a failed refresh does not log the user out; a write is never replayed by read recovery.

## Verification record (2026-09-29)

- Frontend full suite: 1549 passed; four optional Nginx tests skipped in the default run. After the final catalog classification change, all 141 model tests passed, including four new regressions.
- Real Nginx: all four gateway tests passed when NGINX_BINARY was supplied.
- Real Chromium: two cross-tab recovery tests passed (Web Locks and forced IndexedDB fallback); only synthetic loopback sessions were used.
- `go test ./...` and targeted `go vet` passed. Session rotation now atomically replaces the old session; tests cover failed storage writes and concurrent attempts.
- ESLint, TypeScript and the production build passed; assets/packaging checks passed. Infrastructure validation passed with Promtool explicitly skipped.
- Local fixture preview at 1440x900: sidebar heading/list geometry verified, no page errors. No authenticated live DEV smoke test or actual 15-minute live-session soak was performed.
- PostgreSQL rollback/concurrency integration test is implemented and wired to a dedicated CI service. It was skipped locally because TEST_DATABASE_URL is unset; the CI job has not run yet.
- Review findings fixed: login/refresh serialization, bounded login duration, missed cross-tab logout-start event, same-account reauthentication, and typed catalog recovery.
- No commit, push or deployment was performed. The 30-day DEV gate still needs publication and environment configuration before it changes the live site.
