# Website referrals and available login methods

Approved scope: implement invitations now; decide activation and reward rules later. Keep Google adapters, disable Google login/linking, and use the primary verified email as the profile/menu label.

## Constraints

- Canonical account IDs own invitations. No synthetic VK users.
- Reuse account referral codes and relations; preserve existing VK behavior.
- Record an anonymous, random invitation token hash. First invitation wins. No email, IP or provider tokens in referral analytics.
- Accept only for an account created after the invitation, within 30 days. Reject self-referrals and reassignment. Acceptance is atomic and idempotent.
- Email signup and OAuth callbacks preserve the same HttpOnly invitation cookie. Authenticated acceptance retries must not prevent successful login.
- Public capture is bounded and validates the application Origin. Account reads use the session; acceptance requires CSRF. Responses are uncached.
- Website reward posting is disabled until rules are agreed. Display actual counters and an explicit pending-rules message; do not invent bonus amounts.
- Google is disabled by default at the server and hidden in the UI. Disabled Google cannot be the remaining usable login method.
- No commit, push or deployment in this implementation turn.

## Tasks and verification

1. Add migration 000056 and `webreferralservice` with account-based PostgreSQL storage. Red tests: first touch, new-account eligibility, self-referral, concurrent/idempotent acceptance, account-scoped summary, no rewards. Existing referral scanners must tolerate nullable legacy IDs.
2. Add browser endpoints `GET /web/v1/referrals`, `POST /web/v1/referrals/visit`, `POST /web/v1/referrals/accept`. Use a secure HttpOnly cookie, strict input, service timeouts and the existing session/CSRF middleware. Retry acceptance after successful signup/OAuth and at authenticated workspace initialization. Test cookies, origin, CSRF, owner isolation and unavailable-service behavior.
3. Add localized `/invite/{code}` entry and replace the profile placeholder with loading/error/real-data states, copy invitation and actual counters. Use existing UI components and translated copy. Preview remains read-only with no invented statistics. Tests cover capture, copy, failures and malformed API data.
4. Disable Google using explicit configuration, including bearer/browser adapters and last-method checks. Fix the shared primary-email label and disabled identity UI. Run focused backend/frontend tests.
5. Run backend affected suites, frontend lint/typecheck/tests/build/assets/packaging and migration checks. Review the complete diff and document actual limitations and configuration. Update architecture/UI routing notes only where behavior changes.

Deferred: qualifying activation event, reward size and settlement rules. No website credit grants are reachable in this release.
