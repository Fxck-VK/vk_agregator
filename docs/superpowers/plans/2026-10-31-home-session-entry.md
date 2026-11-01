# Session-aware homepage entry plan

> Execute in the existing checkout. User requested the observed guest landing / signed-in workspace behavior. No commit, push or deployment.

**Goal:** Keep the homepage for guests and direct a verified signed-in browser to `/ru/app`.

**Architecture:** A server-only homepage entry check uses the existing cookie-forwarding no-store API and canonical profile parser. Session cookies are only a lookup hint. Guests with no session cookies make no auth request. A refresh-only session uses the existing serialized, CSRF-protected browser recovery without adding visible UI; only success refreshes the server tree for a new verification. Local preview remains a guest on the public homepage.

**Constraints:** Russian-only; no landing markup/styles, other public pages, backend ownership or payment changes. Never persist credentials or use a cookie value as authorization. Root responses remain private/no-store and use a temporary localized redirect. An unavailable or rejected session leaves the landing intact.

- [x] Add failing tests for guest, verified, rejected/malformed/unavailable and refresh-required sessions, with preview isolation and no extra balance/sidebar calls.
- [x] Add a bounded server identity check and invisible client refresh helper, reusing existing session recovery and localized server navigation.
- [x] Integrate only the home page and its private/no-store proxy response; test that other public pages retain their routing.
- [x] Run targeted tests, types/lint, production build, packaging and docs checks. Verify guest root/home locally and browser recovery using isolated synthetic responses; no real login or provider calls.
- [x] Update current routing/UI documentation and record actual results. Layout is unchanged; no responsive CSS change.

## Verification

- 110 tests in 16 session/routing/HTTP files passed, including 38 focused cases. Scoped ESLint and full TypeScript passed; production build passed.
- 11 browser/HTTP cases: actual guest root, localized home, slash variant and pricing; original recovery component with isolated successful/rejected/unavailable/network/no-CSRF/already-restored/unmounted responses. Guest preview made zero auth reads; home no-store headers confirmed. Authenticated server redirect is covered by actual route/helper tests, not a real user login.
- Evidence: `.cache/home-entry-20261031/results.json`, QA script, `.cache/home-entry-{tests,regression,lint,types,build,browser,docs,packaging}.log`.
- Independent focused review found no material issues. No layout/style change, eight-width visual rerun is not required for this routing-only change. No commit, push, deployment or paid/provider activity.
