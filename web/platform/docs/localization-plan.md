# Platform localization implementation plan

Status: completed initial dictionary migration. The cookie-based locale source
described here is superseded by [URL locale routing](locale-routing.md) and its
[implementation record](locale-routing-plan.md); dictionary ownership is unchanged.

Approved scope: shared dictionaries, complete UI text extraction, persistent locale, parameters/plurals/formats, localized catalog content and errors, responsive/RTL readiness, automated checks.

Use the writing-plans, test-driven-development and verification-before-completion workflows inline. Preserve the existing working tree and Russian UI. Do not change provider capabilities, billing or user content. No deployment or paid generation is part of this work.

## Architecture

Locale is request-scoped on the server and context-scoped in the browser. A validated preference cookie initializes the root document and provider. A server action updates that cookie; the refreshed server tree preserves client state. No process-global mutable language and no translated authentication/storage identifiers.

`src/i18n/locales.ts` defines supported locales and document direction. `dictionary.ts` defines the shared dictionary shape; `ru.ts` and `en.ts` implement it. Existing public dictionaries remain part of the same locale choice. `LocaleProvider.tsx` exposes `useLocale()` and `useDictionary()`. `server.ts` reads the request locale and dictionary. `format.ts` handles named parameters, plural forms, dates, numbers and money. Additional messages and editorial/catalog content use separate typed namespaces under `src/i18n`.

## Delivery sequence

- [x] Core: locale validation/cookie, typed dictionaries, formatter tests, request-scoped loader and provider. Tests cover fallback, plural forms, parameters and independent locale instances.
- [x] Dictionary migration: direct Russian imports replaced by scoped dictionary access; translated arrays use locale-aware factories. Server pages use the request dictionary; pure functions accept locale/dictionary explicitly.
- [x] Text inventory: static UI strings, accessible names, errors, notifications and editorial descriptions extracted. Stable IDs, URLs, user prompts and private content remain unchanged. English dictionary added with parity checks.
- [x] Preference: LanguageSwitcher uses ModeSwitchPanel in the account menu and public/guest/login surfaces. Drafts and active models survive switching. Metadata and lang/dir are localized on first render.
- [x] Content and formats: catalog presentation and error codes localized at display boundaries; dates/counts/currency use the selected locale. Requests retain raw values. Unknown external content has an explicit source-language fallback.
- [x] Layout: long strings and narrow/desktop widths checked, including RTL indicator alignment, scrolling, keyboard selection and sidebar movement.
- [x] Checks: dictionary keys/parameters, source scan, preference and SSR tests, existing suites, lint/typecheck/build and browser checks completed.

## Acceptance checks

`npm run test:i18n`, scoped Vitest suites, `npm run typecheck`, `npm run lint`, `npm run build`, and `git diff --check` must report actual outcomes. Browser checks cover home, chat, model selector, files, previews and account menu; locale survives reload and never translates user messages. A second concurrent browser session must keep its own locale. Synthetic long text and RTL checks must not widen the page or hide controls.

## Verification — 2026-09-16

- 1,326 Vitest tests across 201 files passed; all 6 asset tests passed.
- Typecheck, ESLint, production build and packaging assertions passed.
- 3 Playwright localization scenarios passed against the read-only local preview with Edge: independent server locales/file preview, draft/model persistence, narrow long-label/RTL interactions.
- Additional English browser checks at 390 × 844 covered home, models, inspiration, profile and files: no page errors or document overflow.
- Existing static tests were updated for dictionary-backed labels, logical corner radii, current theme tokens and the existing rejection of unsupported chat attachments.
- Usage and adding languages: [localization.md](localization.md). Additional languages still require their dictionaries and editorial translations.
