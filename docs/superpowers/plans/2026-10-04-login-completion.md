# Login completion and diagnostics

**Goal:** Confirm the browser session after login, replace the guest document with current authenticated content, and identify failed sign-in stages without logging credentials.

**Evidence:** LoginForm accepts a successful login response and only calls router.replace. Guest workspace layouts can already be in the Next client cache; the guest layout does not run WorkspaceSessionHealth. EmailRegistration uses the same client-navigation pattern. websession.passwordLogin currently maps every password-service error to 401, including temporary failures and rate limits. The proxy already generates X-Request-ID and sanitized request logs.

**Design:** Preserve existing cookie authentication, CSRF, Origin and session locking. After cookie issuance, verify `/web/v1/me` with a bounded no-store request; validate its safe profile and, when provided, match the newly issued account header. A successful check starts a document navigation to a validated localized return path so old guest RSC/layouts cannot be reused. Keep progress until navigation completes; report a navigation timeout if the old document remains. Share completion across password login and registration.

**Diagnostics:** Named stages and a bounded set of codes distinguish credentials rejected, rate limit, service unavailable, network/timeout, session not confirmed, invalid response and navigation failure. Log only stage, code, numeric HTTP status, elapsed time and a validated UUID correlation ID. User feedback includes a short explanation and support ID, never raw response/error text. Server password-login failures log a sanitized stage and return the correct HTTP family. Do not send personal email, change personal passwords or deploy this task.

- [x] Reproduce missing session confirmation with a failing LoginForm test; reproduce backend temporary/rate-limit failures misreported as 401.
- [x] Add tested completion/diagnostics/navigation helpers; wire password login and signup, retaining safe locale return paths and preview guards.
- [x] Classify backend login failures and correlate them with existing proxy request IDs.
- [x] Verify pending, rejected cookies/session, invalid response, rate limit, network/timeout, duplicate submits, navigation failure and safe diagnostics. Run a loopback browser fixture with synthetic accounts.
- [x] Run auth suites, backend test/vet, frontend lint/typecheck/test/build/packaging, docs/whitespace checks; restart local services.
- [x] Review security and update the runbook, UI catalog, architecture and active decision. Record results and limits below.

No commit/push without a new request. Existing backup-email and music changes are preserved.

Verification: initial regression tests failed for absent confirmation and backend 401 classification. After implementation, the full platform suite passed 243 files / 1685 tests (one file / four tests intentionally skipped), asset tests 6/6, Next lint contract 2/2. A later explicit stalled-request test exposed cross-realm DOMException classification; fixed it by inspecting only the exception name and reran all 12 completion tests successfully. The final build, typecheck, lint, packaging and documentation validation passed. Go websession/accountauth/accountregistration/accountservice tests and vet passed; the subsequent safe-log test and password-login tests passed. Independent scoped review: PASS, no findings.

The real browser fixture passed all three cases against the built platform: a previously visited guest workspace becomes a new authenticated document; missing browser cookies produce a session-confirmation reason and correlated reference; upstream 503 produces the availability reason and correlated reference. Fixture preparation first omitted public/static packaging assets and used a button locator for a guest link; corrected both, then filtered out the Next route announcer from error assertions. These were test fixture failures, not successful application checks. Local API and frontend were restarted with the final build. No live personal sign-in, email, password change, DEV deployment or push was performed. Browser diagnostics are in the console; server/proxy failure correlation is available in their existing logs, with no new external telemetry service. Pre-2xx registration retry diagnostics remain its existing behavior.
