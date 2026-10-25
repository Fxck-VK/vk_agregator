# T24 Author Archive Implementation Plan

Status: completed locally. Local only; no commit, push or deploy.

Goal: /en/blog/author/epifera from saved Magnific reference24. EN-only with
RU empty; both draft/noindex outside sitemap. Same checkout and localhost7159.

Architecture: AuthorProfile is the existing Epifera editorial brand, no invented
person/bio/social accounts. Server-only author catalog projects existing two
articles, course/lessons, documentation and workflow into minimal material cards.
No content duplication or false publication dates; known article dates remain
labelled draft, others date=null. Twelve cards per page, real page count and
server-rendered ?page= pagination with localized Links/hash. Search covers the
entire catalog, including other pages, via existing LandingDialog/InputSurface/
ScrollArea. Header uses existing editorial variant; own archive layout, centered
footer and BrandLogo. Existing article bylines link to the new author route.

Reference covers are cached local AVIF/WebP; actual JPEG converts losslessly
with decoded-pixel and source/output hashes. Deduplicate identical outputs;
the reference's image-free fourth card uses a CSS gradient. Covers are credited
visual illustrations, not proof of Epifera results. No foreign brand avatar,
stats, biography, publication counts or fake 31-page archive.

## Task1 — admission / red contracts
- [x] Fresh dependency audit before long tests: zero vulnerabilities.
- [x] Failing locale/noindex/shell, twelve-card grid, real pagination, safe query,
  whole-catalog local search, focus/inert, no-writes and article byline tests.
- [x] Package cached covers with actual formats and provenance.

## Task2 — implementation
- [x] Add profile/labels/types, server-only derived catalog and pure pagination/
  search helpers; no provider/account/Jobs/billing/private artifact changes.
- [x] Build reference archive and responsive grid, pagination links, local search
  and source dialog; use shared components only where matching the reference.
- [x] Add page/publication and exact own-shell declaration; connect author in
  real article bylines and navigation/footer. Preserve other draft templates.

## Task3 — verification / review
- [x] Focused and affected public/shared tests; typecheck, full lint, assets,
  packaging/publication checks and production build.
- [x] HTTP both locales, page1/2/last/clamped query, noindex/sitemap, all target
  material links and media MIME/bytes/SHA. No iframe, external scripts or writes.
- [x] Native390/820/1280/1600px; grid, full pagination/back navigation, direct
  page2 reload, search across pages/empty state, menus/source/focus and no overflow.
- [x] Inline review and UI catalog/index/docs index/state; final git status.
  Retain local page and screenshot, reset viewport. All24 local templates then done.

Verification: 398 affected tests in42files passed. After native Clear focus reproduction, failing regression and fix, all69 focused tests across archive/two articles/shell passed. Types/full lint, audit0, assets/packaging/publication and final production build passed. HTTP verifies1/2/3+invalid page queries,25 distinct target routes, bylines/nav, EN/RU/noindex/sitemap/CSP and seven asset MIME/bytes/SHA. Native390/820/1280/1600px passed: pagination/back/reload, whole-catalog search/empty/clear/keyboard/focus, mobile menu, source and actual byline navigation. Scoped mobile header/nav heights and profile/grid offsets match the saved reference. Final viewport1280x720. No publication or backend/account/provider/Jobs/billing/private artifact changes. All24 local template types implemented.
