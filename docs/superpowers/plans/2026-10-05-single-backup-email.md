# One primary and one backup email

Status: implemented locally. User approved the behavior on 2026-10-05.

Keep one primary and one backup email per account. Derive roles from verified
email bindings ordered by creation time and ID; preserve any legacy additional
bindings, allowing removal but no further additions. No destructive migration.

1. Add regressions for concurrent email additions, roles and hidden add action.
2. Serialize email link, replacement and unlink on the account row (memory mutex
   for tests). Enforce the two-email limit. Replace only the current backup in
   place, with a conditional version check, preserving the primary and audit.
3. Bind replacement codes to account, backup identity and captured version in a
   dedicated challenge namespace. Verify ownership before checking conflicts.
4. Expose cookie-authenticated, Origin/CSRF protected replacement actions and
   safe role metadata. Show primary/backup labels, replace/unlink and a code
   form. Keep the old address until success; reset consumed proof on conflict.
5. Verify concurrency, stale proofs, expiry, foreign ownership, replay, safe
   DTOs, browser route guards and UI states. Run backend test/vet and frontend
   lint/typecheck/test/build/packaging. Obtain a bounded read-only review.
6. Update account contract, architecture, UI catalog and state. Restart owned
   local runtimes if necessary. No commit, push, DEV deployment or live mail.

Risks: cross-tab replacement must not overwrite a newer binding; recovery must
remain proof-only; safe DTOs must omit raw addresses and internal versions;
same-account email conflicts must not mutate bindings. All billing/jobs and
unrelated music changes stay outside this work.

Verification: scoped Go test/vet including API wiring; isolated PostgreSQL
concurrent additions/replacements and audit rollback; recovery/unlink regression
and atomic credential/session/audit rollback; frontend 1690 tests passed (four
existing skips), lint, typecheck, production build, packaging and the browser
sign-in fixture. Read-only review passed after its recovery/unlink finding was
reproduced and fixed. Mock delivery only; no new live mail or real-account writes.
