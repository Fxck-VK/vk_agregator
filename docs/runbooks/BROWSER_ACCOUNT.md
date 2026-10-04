# Browser account access

Status: implemented browser integration, 2026-10-04. Provider login and code delivery require operator configuration and real-provider smoke before enabling them for users.

The platform uses `/web/v1` cookie sessions and the shared Account Layer, rather than token-returning `/account` login routes. `GET /web/v1/auth/methods` exposes availability flags and configured provider names. Login and profile receive this contract on the server; missing configuration disables the corresponding actions.

## Email and phone

- Email/password login accepts a verified email with a password. Public registration is email → six-digit code → password → account and cookie session; `registration` in the availability contract is enabled only with configured SMTP and the registration service. An older contract without this flag keeps registration disabled.
- Registration uses `POST /web/v1/auth/email/request-code`, `/web/v1/auth/email/verify-code`, then `/web/v1/auth/email/register`. Every public mutation requires exact Origin. The server creates a Secure HttpOnly `__Host-nh-signup` cookie; its random binding must accompany code confirmation and account creation. The code and proof expire after 10 minutes. No account ID, verification assertion or proof token is accepted from browser JSON.
- Redis stores hashed email, HMAC code and one-use proof; comparison/consumption are atomic. Limits are three requests and five verification attempts per email per 15 minutes; a broad 100-request ceiling applies to the transport peer, which may be a shared BFF (not an individual user's IP). Additional account-auth limits apply at creation. Redis outages fail closed. `VK_APP_SECRET` supplies the existing server secret with a separate registration HMAC scope.
- Account, verified email, Argon2id credential and audit entries are created atomically. A proof maps to a stable server-issued account UUID so transient storage/session/cookie-preparation failures can restore that proof with its original expiry. Retrying preserves the first password; the form keeps it read-only for a transient retry and clears it when leaving or completing the flow. Existing email identities cannot be overwritten or merged through registration.
- Recovery: `POST /web/v1/auth/password/request-reset` with email, then `/web/v1/auth/password/reset` with email, code and `new_password`. Both require exact Origin. Request-code returns the same accepted response for known and unknown emails. A successful reset revokes existing account sessions.
- Configure `ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp`, `ACCOUNT_EMAIL_SMTP_HOST`, `ACCOUNT_EMAIL_SMTP_PORT`, `ACCOUNT_EMAIL_SMTP_USERNAME`, `ACCOUNT_EMAIL_SMTP_PASSWORD`, `ACCOUNT_EMAIL_SMTP_FROM`, and existing TLS/timeout settings. Without SMTP, recovery and email linking are unavailable.
- The profile links email and phone with codes through the existing account linker. Phone is a binding, not standalone SMS login. Enable the existing HTTP delivery adapter with `ACCOUNT_PHONE_DELIVERY_PROVIDER=http` and `ACCOUNT_PHONE_HTTP_*` settings.
- Authenticated mutations require cookie principal, matching Origin and CSRF. Client account IDs cannot establish authority.

### Resend sending configuration

The verified sending domain is `notify.neiirohub.ru`. Use the existing SMTP adapter; no Resend key or SDK belongs in the frontend. Configure the selected backend runtime with these non-secret values:

```dotenv
ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp
ACCOUNT_EMAIL_SMTP_HOST=smtp.resend.com
ACCOUNT_EMAIL_SMTP_PORT=587
ACCOUNT_EMAIL_SMTP_USERNAME=resend
ACCOUNT_EMAIL_SMTP_FROM=noreply@notify.neiirohub.ru
ACCOUNT_EMAIL_SMTP_TLS_MODE=starttls
ACCOUNT_EMAIL_SMTP_TIMEOUT=10s
```

Set `ACCOUNT_EMAIL_SMTP_PASSWORD` to the Resend API key in server secrets/runtime env only. Keep rotation of disclosed keys in the operator workflow. The selected API container already receives its runtime env file; restart/redeploy it after changing configuration. For DEV, the dedicated repository secret `DEV_ACCOUNT_EMAIL_SMTP_PASSWORD` is applied by `scripts/deploy/prepare-dev-email.sh` without replacing the existing split secrets. Dispatch `Deploy DEV` from `dev-deploy` to install or rotate the ignored server-only `.env.dev-email` overlay (mode 600, email settings only). It is read only by the DEV API, after the main env file, and survives subsequent automatic deployments; it is excluded from Git and Docker build context. Local runtime configuration stays in ignored `.runtime/email-registration/local.env`; set `NEIROHUB_LOCAL_WORKSPACE_PREVIEW=0` and the local backend origin in the ignored platform development env for real delivery. No mail secrets belong in that frontend env. A mailbox for this sender is not required. Receiving and click/open tracking are unnecessary for verification codes.

For delivery smoke, use an explicitly provided recipient and the selected contour. Request a code once, check the recipient inbox and Resend delivery status, confirm it in the same browser, set a password, and verify login/logout with the new account. Mock delivery and local preview do not establish real delivery. [Resend SMTP documentation](https://resend.com/docs/send-with-smtp).

## External login

Register the exact HTTPS callback `${WEB_ORIGIN}/web/v1/auth/oauth/{provider}/callback` (`google`, `apple`, `vk`, `telegram`). WEB_ORIGIN is the public frontend origin. The first client ID in each existing audience list is the browser client; mobile IDs follow it.

| Provider | Browser configuration | Proof |
| --- | --- | --- |
| Google | `ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS`, `ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET` | code + PKCE, verified OIDC token and nonce |
| Apple | `ACCOUNT_OAUTH_APPLE_CLIENT_IDS`, `ACCOUNT_WEB_OAUTH_APPLE_CLIENT_SECRET` | code, verified OIDC token and nonce |
| VK ID | `ACCOUNT_OAUTH_VK_ID_CLIENT_IDS` | code + PKCE + state, server-side user_info |
| Telegram | `ACCOUNT_OAUTH_TELEGRAM_CLIENT_IDS`, `ACCOUNT_WEB_OAUTH_TELEGRAM_CLIENT_SECRET` | code + PKCE, verified OIDC token and nonce |

Secrets belong in deployment secrets/runtime env, never NEXT_PUBLIC variables. Apple's secret is an operator-generated, signed client-secret JWT; renew before expiry. Apple uses code-only response_mode=query without name/email scopes. Telegram uses OIDC client credentials, not the legacy Login Widget bot token. Existing issuer/JWKS configuration still applies to OIDC verification.

Login starts at `POST /web/v1/auth/oauth/{provider}/start`; profile linking starts at `/web/v1/account/oauth/{provider}/start`. They return the authorization URL. Server-owned Redis transactions expire after 10 minutes and bind provider, browser cookie, state, PKCE, nonce, locale and optional owning account. Consumption is atomic and one-use. Linking must have a valid session for the same account at completion; conflicts do not merge accounts.

Provider tokens stay on the backend. Login issues existing Secure HttpOnly cookies. The frontend proxy permits only fixed 303 callback destinations, preserves separate Set-Cookie headers and sets Referrer-Policy=no-referrer. JWKS cache entries are bound to their source URL.

## Profile and preview

Profile → Access and security shows bindings, code verification, password setup and active sessions. Revocation is owner-scoped. Ending the current session uses the shared cookie logout under the refresh lock, clears client account state and returns to login. Device/IP/token hashes are not displayed. Both shared identity repositories preserve a verified non-phone sign-in method when unlinking; PostgreSQL serializes unlink operations with an account row lock before counting identities inside a transaction.

`npm run dev:preview` shows all future provider buttons disabled. Recovery and registration are marked local UI simulations: they never send mail, create accounts or change passwords. Registration accepts any six-digit preview code and explicitly labels both code/password steps as simulation. Profile mutations are disabled. The preview route continues to block writes to a real backend; preview is development-only.

## Verification and release

Tests cover Origin/CSRF, anti-enumeration, reset revocation, provider exchange, PKCE/nonce, browser binding, expiry/replay, signup throttling, concurrent duplicate creation, atomic rollback, session/cookie retry, session ownership, callback redirects and preview isolation. Run focused Go tests, platform auth/account/proxy tests, lint, typecheck and build.

Before enabling real methods, configure the provider and delivery channel, register callbacks, complete login/cancel/link/conflict with operator test accounts, verify real delivery/registration/recovery, and verify session revocation. These external operations are not established by unit tests. Isolated PostgreSQL tests require TEST_DATABASE_URL; the registration Redis integration test requires TEST_REDIS_URL. On 2026-10-04 these ran against local PostgreSQL 16 and Redis 7, including concurrent registration, rollback and credential preservation. Real Resend SMTP authentication and the local browser request-code step also passed with an operator-authorized recipient. Inbox receipt and account creation require the operator's code/password steps; SMTP acceptance alone does not establish inbox delivery. A sending-only Resend key cannot read delivery status through the email-list API.

Protocols: [Google](https://developers.google.com/identity/openid-connect/openid-connect), [Apple](https://developer.apple.com/documentation/signinwithapplerestapi/request-an-authorization-to-the-sign-in-with-apple-server), [Telegram](https://core.telegram.org/bots/telegram-login), [VK official SDK](https://github.com/VKCOM/vkid-web-sdk/blob/master/src/auth/auth.ts).
