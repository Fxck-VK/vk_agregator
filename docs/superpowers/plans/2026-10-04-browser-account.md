# Browser account implementation plan

Approved scope: email/password login, email password recovery, configured Google/Apple/VK/Telegram browser login, identity linking and session management in the profile. No email registration or phone login. No deployment in this task.

Reuse the shared Account Layer, cookie session adapter and existing UI controls. Credentials, OAuth code/PKCE/nonce and session tokens must not enter browser storage, logs or frontend configuration. Short-lived OAuth transaction material stays in server Redis. Browser writes require origin and authenticated writes also require CSRF. External login uses a short-lived, one-use transaction bound to the initiating browser; linking additionally requires the same account.

- [x] Add tests and browser account routes: configured capabilities, recovery, email/phone binding, unlink, password setup, active sessions and revocation. Reuse account handlers without exposing token-returning routes.
- [x] Add tests and server OAuth authorization-code flow for the four providers, transaction storage, nonce verification and cookie session completion. Wire server-only configuration and provider availability.
- [x] Extend LoginForm with recovery steps and provider buttons; add a profile security panel. Reuse Button, InputSurface, AsyncState, language and theme tokens; add RU/EN text.
- [x] Keep preview isolated from real writes; expose the complete layout with explicit preview availability. Update proxy to permit only fixed OAuth callback redirects.
- [x] Update account contract/runbook and UI catalog, run focused backend/frontend tests, typecheck, lint and build; inspect local screens.

Validation: 172 focused platform tests passed (25 files), TypeScript, ESLint, packaging and production build passed. Focused account/OAuth/web-session/config/API/storage/service Go tests and vet passed; documentation validation passed. Local RU/EN login, recovery steps, profile security and mobile login inspected. Shared identity unlink now preserves non-phone sign-in under concurrency; the in-memory concurrency test passed. The isolated PostgreSQL concurrency integration test was skipped because TEST_DATABASE_URL is absent. Real OAuth/SMTP/SMS flows remain an operator release check.

Files: internal/adapter/inbound/{account,websession}, internal/adapter/accountoauth, internal/adapter/storage/redis, internal/platform/config, cmd/api/main.go; web/platform/src/features/{auth,account}, lib/{auth,web-api}, i18n, app/login/page.tsx, app/web/v1/[...path]/route.ts.

External end-to-end login and real delivery need operator-owned provider/SMTP/SMS settings. Unit tests use fake provider responses; no real email, SMS, or external account action is authorized by the UI implementation request.
