# English public homepage rebuild

**Goal:** Remove the former homepage on both /ru and /en first, then rebuild the
English Epifera home from the retained 01-home Magnific reference. Russian home
stays visually empty. Both home versions remain draft/noindex during this work.

**Approved scope:** The user approved deletion before rebuilding, English-only
implementation, a faithful desktop/mobile reference, reusable public components,
and local verification on the existing localhost:7159. No commit/push/deployment
or changes to workspace, login, accounts, billing or provider admission.

**Architecture:** Replace the retired MarketingHome feature with a clean
public-home feature and reusable landing chrome/sections. Keep BrandLogo,
BrandName, localized Link, shared FAQ and modal primitives. Put English-only
copy in an English public dictionary, not inline translated Russian strings.
Metadata and sitemap continue to consume the co-located home publication policy.
Existing public content pages retain their shell; the home owns its own chrome.

**Reference:** references/magnific-html/01-home.html and the desktop/mobile/menu/
suite/filter/FAQ/slide captures under references/magnific-states/01-home. Do not
execute or embed third-party scripts, trackers, authentication or payment flows.
Retained reference media may be reused locally; avoid invented ranking, customer,
price or product-availability claims for Epifera. Unsupported reference links
must lead to honest existing Epifera destinations or remain clearly unavailable.

## Execution

- [x] Capture current scope/dependencies and add route/SEO regression tests.
- [x] Delete the legacy Marketing* feature, its orphaned tests/copy, and replace
  home with an empty route for both locales. Set publication to draft/noindex;
  verify both pages have no former markup and sitemap has no home entries.
- [x] Build the English data contract and reusable LandingHeader, LandingFooter,
  LandingSection, dialog, carousel and media primitives from scratch. Shared
  primitives may be reused, but do not import the old homepage or its stylesheet.
- [x] Compose English home sections: hero; suite tabs and media; model strip;
  workflow grid; tool filters; use-case cards; original carousel; plan cards;
  FAQ; closing and footer. Preserve the reference's warm neutral/red palette,
  generous sections, full viewport hero, rounded media and compact navigation.
- [x] Add meaningful interaction tests for keyboard menus/tabs, filters,
  navigation, reduced motion/pause and modal focus. Do not add CSS-string tests.
- [x] Verify English route content, blank Russian route, noindex/empty sitemap,
  other public pages and workspace route. Run focused tests, lint/typecheck,
  production build, packaging and docs checks.
- [x] Inspect local desktop and mobile in the existing browser, compare against
  the saved captures, exercise controls, save screenshots and restore viewport.
- [x] Update UI catalog/index and active route documentation. Report actual
  local outcome and remaining limitations; leave the single site running.

## Verification commands

From web/platform:

```powershell
npm exec -- vitest run 'src/app/(public)/page.test.tsx' src/features/public-home src/components/public/landing src/i18n/seo.test.ts src/content/publication/publication.test.ts src/i18n/public/LocalizedPublicShell.test.tsx 'src/app/(public)/ai/image-generator/page.test.tsx'
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Root: `pwsh -NoProfile -File scripts/ci/validate-docs.ps1` and `git diff --check`.
Use real local HTTP and CUA browser checks for geometry/behaviour; no paid calls.

## Local verification, 2026-10-24

- Focused route, landing, i18n, brand and publication tests: 110 passed in 17 files.
  The hover/click regression was observed failing before its fix and passing after.
- Full lint, typecheck, production build, asset validation/tests, packaging and
  documentation validation passed. The production build was repeated after the
  final source changes and exited successfully.
- HTTP: /en, /ru, /en/login, /ru/app and /en/ai/image-generator returned 200.
  English home has one English H1; Russian home has no H1 or retired layout.
  Both homes emit noindex/nofollow; /sitemap.xml has no loc entries.
- Browser: 1264x900 desktop and 390x844 mobile. Hero ends at viewport height
  on both; page width never exceeds viewport width. Verified navigation/Escape,
  suite/category tabs, tool filters, carousel arrows/dots, Russian model search,
  video dialog, FAQ, mobile footer and native back-to-top. Viewport restored.
  Screenshots: .cache/english-home-final.png and .cache/english-home-mobile.png.
- Legacy code/dictionaries and two orphaned images removed. Shared imagery,
  ImageToolLanding and login references retained. No new dependency or backend,
  authentication, billing, generation or storage changes. No paid calls.
- Reference media/font remain draft comparison assets; final release requires
  approved content and appropriate asset rights. Unavailable reference products
  are marked Coming soon; no third-party scripts or fabricated claims imported.
- Everything remains local in the one active checkout; no commit, push or deploy.

Subsequent authorization (2026-10-24): user requested pushing all current changes
to dev-deploy together. Release evidence is kept in ignored
.cache/release-2026-10-24-all/status.json; do not infer deployment from this plan.
