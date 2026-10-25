# Legal page reference rebuild

**Goal:** Remove the old legal test layout, then rebuild the existing legal routes from saved Magnific reference 23 while preserving the reviewed Epifera documents.

**Approved design:** The user approved a fresh implementation from the saved reference after removal of the old presentation. Desktop uses a 72px icon rail, a compact breadcrumb/action bar, a 320px document navigation panel and an independently scrolling reading panel. Mobile keeps the compact top bar and opens global/document navigation in accessible drawers. This reference has no marketing header or footer, so do not reuse the homepage chrome.

**Architecture:** Keep document resolution, runtime operator fields, source hash, publication state and rendering on the server. Pass only localized document navigation and the server-rendered body to a client frame. Both existing locales and all five slugs remain reachable; authoritative document text remains Russian. No legal content, checksum, consent, authentication, payment or provider change.

**Constraints:** Single active checkout and localhost:7159. No dependencies, original third-party JavaScript, new ports, worktrees, commit, push or deployment. Preserve the current unpublished image-generator work. Noindex and sitemap exclusions remain. Use existing brand, icons, localized links and modal primitives.

## Implementation

- [x] Add meaningful route/content-preservation tests and mobile navigation/focus tests; verify their expected failure before implementation.
- [x] Remove the old page JSX and page-only styles from `features/legal/Legal.module.css`; preserve registration, payment-notice and profile styles.
- [x] Add `LegalPageFrame`, `LegalNavigation`, `LegalPageContent` and scoped CSS in `features/legal`; add RU/EN presentation labels in `i18n/public/legal-page.ts`.
- [x] Replace `/legal` and `/legal/[slug]` with thin server compositions; let these routes own their shell in `LocalizedPublicShell` without affecting other public routes.
- [x] Verify active-document groups, working links, document scrolling, keyboard navigation, Escape/focus return and mobile overflow at 1280px and 375px in the existing local site. Existing table cells are compared against the catalog in route tests; the selected live privacy document has no table.
- [x] Run focused legal/shell/registration tests, typecheck, lint, packaging and production build. Keep catalog hash and all reviewed texts byte-for-byte unchanged.
- [x] Update UI catalog/index, legal runbook and current routing context with actual local status; inspect final diff and git status.

## Verification commands

From `web/platform`: `npx vitest run src/features/legal src/app/\(public\)/legal src/i18n/public/LocalizedPublicShell.test.tsx`; then package `typecheck`, `lint`, `test:packaging` and `build`. Existing legal-catalog tests verify the reviewed revision checksum; route tests compare every paragraph/table cell with the resolved catalog and check fail-closed missing configuration and unknown slugs. Browser verification uses synthetic or existing local runtime fields without printing operator details.

## Reference evidence

Saved HTML: `references/magnific-html/23-privacy.html`. Desktop start: `references/magnific-states/23-privacy/0255-desktop-default.webp`; mobile: `0187-mobile-default.webp`. Geometry is implemented from that reference; unused competitor-only document categories are omitted rather than creating fictitious Epifera policies.

## Verified locally

- 64 tests across 11 legal, shell and authentication files passed, including every document paragraph/table cell, missing operator configuration, noindex, unknown slugs and keyboard focus return. The initial legal/shell checks failed against the old layout as expected. Test TEMP/TMP is workspace-local because the Windows sandbox prevents transform-cache renames in its generated system temp directory.
- Typecheck, full ESLint, packaging and the production build passed. The Windows sandbox could not create Turbopack's existing sharp junction; the unchanged build passed outside that restriction. No source/check was weakened.
- Inspected desktop 1280×720 and mobile 375×812. Confirmed exact 72px/320px panel widths, 56px header and 4px pane gap; no horizontal document overflow; real document navigation, active group, focus cycling, Escape return and independent long-document scrolling. RU and EN routes remain reachable; the source text stays Russian.
- Receipts in `.cache/legal-page-rebuild/`: desktop/mobile legal index, mobile menu and cropped document headings. Screenshots avoid publishing operator details into QA receipts.
- Changes were reviewed inline. The project's prescribed gpt-5.5 reviewer is not available in this session; no substitute agent or independent-review claim was used. No commit, push or deployment.
