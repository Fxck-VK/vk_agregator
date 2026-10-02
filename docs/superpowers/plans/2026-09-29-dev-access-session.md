# Remember DEV access for 30 days

Status: implemented and locally verified; not deployed. User approved remembered
DEV access on 2026-09-29.

**Goal:** Keep the DEV site private while avoiding recurring browser password
dialogs during page, media and API requests.

**Architecture:** Nginx checks the existing htpasswd credential only on
`/__dev/login`. A server-only Next route then issues a signed, host-only,
HttpOnly, Secure, SameSite=Lax cookie with a fixed 30-day lifetime. Nginx
`auth_request` checks that cookie before forwarding any application request.
Only document navigation redirects to login; background requests receive 401
without a Basic challenge. Account authentication is an independent boundary.

**Constraints:** No new dependencies, no password in browser storage, no shared
account identity, no public internal gate endpoints, no secret logs. Keep
existing bcrypt/APR1 validation in Nginx. Derive separate signing/issuer keys
from the existing secret htpasswd entry with distinct SHA-256 domain labels.
This preserves sessions across deployments and revokes them on credential
rotation without adding a new deployment secret. The rendered issuer proof is
private and passed only after Nginx authenticates the login request.

## Execution

- [x] Write and run failing session/route tests: signature, expiry, rotation,
  cookie flags, forged issuer, redirects, background 401, disabled contour.
- [x] Implement `web/platform/src/lib/dev-access/session.ts` and internal
  `web/platform/src/app/web/dev-access/[action]/route.ts`.
- [x] Wire `deployments/nginx/dev-web.conf`, `docker-compose.dev-web.yml`
  and the private Nginx credential/issuer bootstrap. Keep Basic Auth only
  at login, strip caller proof/auth headers elsewhere, fail closed.
- [x] Verify the real Nginx gate with synthetic credentials, including
  protected assets/API, login, cookie-only re-entry and upstream account 401.
- [x] Update relevant infrastructure policy and operational/security docs.
- [x] Run focused tests, typecheck, lint, packaging/build and configuration
  validation. Report deployment limits; do not commit/push without a request.

Public root smoke without browser navigation headers must remain 401. Login
is rate limited. Internal route output is private/no-store/noindex. Return URLs
are bounded local paths; control characters, network-path references and
technical gate URLs are rejected. Cookie expiry is fixed, not sliding.

## Verification

- 89 focused session/handler/proxy/locale/API forwarding tests passed.
- Real Nginx 1.30.5 tests run against loopback with synthetic APR1 credentials.
- Typecheck, ESLint, production build and packaging assertions passed. Build
  required network access to the existing Google Fonts dependency.
- `validate-infra.ps1 -SkipPromtool` passed, including rendered Compose. Updated
  its obsolete pre-localization matcher expectation to the current source;
  existing locale/proxy tests also passed. No route/CSP behavior was loosened.
- Existing `test-dev-web-auth.sh` cannot verify Unix mode 600 under Windows Git
  Bash/NTFS; it stops at that assertion. Its unchanged Linux check remains in CI.
- Docker daemon was unavailable. Real Nginx used the official Windows binary;
  CI now explicitly repeats the gateway integration tests with Linux Nginx.
- Live DEV/browser acceptance and rollout remain pending. No push or deployment
  was requested for this change.
