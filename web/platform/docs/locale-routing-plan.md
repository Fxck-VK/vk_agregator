# Locale URL routing implementation plan

Status: implemented, verified and reviewed (2026-09-16).

Goal: all existing UI pages use `/ru` or `/en`; URLs determine SSR and client
language. Language switches preserve the current page, model, draft, attachments,
query and fragment. Existing technical endpoints and authentication stay intact.

Architecture: locale prefixes are external page URLs. `src/proxy.ts` rewrites
them to stable internal App Router pages, forwarding an overwritten locale header.
The same route tree preserves React state when switching languages. A shared
Link/router adapter owns URL construction; domain route comparisons use the
unprefixed pathname. The root client locale boundary follows browser navigation,
while SSR dictionaries use the proxy-attested request locale. No draft persistence
in localStorage or additional backend state is introduced.

Constraints: no new dependencies, no paid calls, no commits/deployments. Existing
dirty workspace changes must be preserved. Private routes remain session-checked,
no-store and noindex. APIs, callbacks, health and assets are not localized.

## 1. Routing contract
- [x] Add failing routing tests for prefixes, service exclusions, path/query/hash
  preservation, URL precedence, unknown locale handling and legacy redirects.
- [x] Implement pure `src/i18n/routing.ts`: `localeFromPath`, `stripLocale`,
  `localizeHref`, `isServicePath`; update proxy CSP and safe return integration.
- [x] Verify `npx vitest run src/i18n/routing.test.ts src/proxy.test.ts`.

## 2. Client and server integration
- [x] Server dictionaries read the locale header, never choose page language by
  cookie. Keep the validated preference action for explicit user preference.
- [x] Add shared locale Link/router adapters, migrate current consumers and route
  comparisons; make root locale provider follow client navigation without keys.
- [x] Language switch sets preference, navigates to the equivalent URL and
  refreshes server translations while retaining current client state.
- [x] Verify locale UI, navigation, auth return, session and payment tests.

## 3. Public SEO and boundaries
- [x] Generate absolute self-canonical and reciprocal language alternates for
  the existing public home only, using trusted deployment origin configuration.
- [x] Add sitemap with published public home URLs only. Keep private/login routes
  noindex and unlisted. Unknown language and missing page URLs return 404.
- [x] Verify metadata, spoofed headers, API/assets exclusions and old links.

## 4. Browser verification and documentation
- [x] Exercise direct RU/EN loads, conflicting cookie, reload, back/forward,
  selected model, actual File attachments, typed draft and login/payment returns.
- [x] Run typecheck, lint, complete unit suite, build and packaging checks.
- [x] Update active route/localization docs and explicitly supersede the former
  unprefixed Russian design. Record final verification, no unrelated redesign.

## Verification — 2026-09-16

- 1,373 Vitest tests across 204 files passed; all 6 asset tests passed.
- Typecheck, ESLint, production build and packaging assertions passed.
- 7 Playwright scenarios passed against local preview with Edge, covering both
  `e2e/localization.spec.ts` and `e2e/locale-routing.spec.ts`.
- A real PNG File and blob preview, selected Seedream model and typed draft
  survived locale switch and browser back/forward without another upload.
  Upload, quote and login API responses were mocked; no real paid requests ran.
- All current pages served the URL language despite a conflicting preference;
  public shell and canonical/hreflang changed together. Private/login routes
  remained noindex; service paths and missing pages retained their boundaries.
- Legacy payment return and login destination checks passed without invoking
  real authentication or payment providers. Existing session/auth tests passed.
- `docker compose --env-file NUL -f docker-compose.prod.yml -f
  docker-compose.dev-web.yml config --quiet --no-interpolate --no-env-resolution`
  validated the overlay without reading or printing service secrets.
- `git diff --check` passed. No deployment, commit or provider changes.
- Independent scoped code review found no P1/P2 issues in routing, locale
  authority, auth returns, service boundaries or public SEO.

Deployment prerequisite: supply the frontend's trusted `WEB_ORIGIN` at runtime.
The existing DEV web overlay now forwards it to the platform container.
