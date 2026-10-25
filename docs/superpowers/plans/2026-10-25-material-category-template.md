# T15 material category implementation plan

Status: completed locally. No commit, push or deployment.

**Goal:** Build `/en/photos/nature` from saved Magnific category reference 15.

**Architecture:** Typed static StockCategory + 50 public reference records, a pure
filter/sort/pagination model and responsive positioned result grid. Reuse matching
ReferenceChrome/Tools, CollectionCard/Preview, PopoverPanel, ModeSwitchPanel,
ScrollArea, MediaImage and footer. No backend/private artifacts/Jobs changes.

## Constraints / reference decisions

- Active checkout only, localhost:7159; English UI, Russian route empty; both
  draft/noindex/outside sitemap. No commit/push/deploy or new dependencies.
- Sources: `references/magnific-html-js/15-nature-category.html`, desktop/mobile
  default and Square/Masonry captures in `.cache/magnific-ui-capture/15-nature-category/`.
  Two navigation-sidebar captures actually navigated to the homepage: ignore them.
- Match compact topic chips/intro, horizontal filter pills, layout switch, cards,
  sort and footer. Do not copy foreign premium badges, claims, promotion, trackers,
  author avatars or unsupported file/model/entitlement options.
- Preserve actual source dimensions and AI labels; license and people metadata
  are not supplied. Filter definitions include those honest unknown values, not
  invented commercial licenses or human counts. Public previews are WebP only.
- MasonryGrid uses CSS columns, unlike the source's ordered shortest-column layout.
  Use a small measured layout component with pure tested geometry; keep existing
  MasonryGrid defaults untouched. Square uses 1/2/3/4 columns; masonry 1/2/3/4.
- CollectionCard may accept optional className/actions for matching reference cards;
  existing collection defaults remain. Category adds actual WebP download and a
  device-only saved public ID. Suggested prompts are editorial, not source prompts.
- Results are a local reference dataset, not a claimed complete licensed stock
  library. Package 50 existing cached assets, with provenance/hashes/pixel checks.

## Task 1 — data / model / policy

- [x] Add failing tests for shell isolation, model filtering/ordering/pagination,
  geometry, route/noindex, actual result count and user interactions.
- [x] Package cached media into `public/assets/images/material-category/`, manifest
  in docs/research. Classify orientation from dimensions, color from dominant RGB,
  source AI label from markup; never infer a license or people count.
- [x] Add typed category/records and explicit EN/RU publication declarations.

## Task 2 — UI and interactions

- [x] Build shared chrome search, topic chips, intro, sort, filter strip and square/
  masonry mode switch. Search/filter/sort reset to page 1, layout preserves filters.
- [x] Desktop pills use shared anchored popovers; mobile uses shared modal with
  the same filter controls, inert, Escape/backdrop/focus return.
- [x] Implement actual client filtering, counts, reset/empty state and 24-item pages.
  No filter URLs or fake remote total; canonical category route remains stable.
- [x] Render responsive measured layout and actual static cards, shared preview/
  prompt handoff, WebP download, device-only bookmarks, related-tool links.
- [x] Link from T13 related content, without changing existing reference members.

## Task 3 — verification / documentation

- [x] Fresh npm audit before long checks. Focused tests and public/shared suite,
  types/lint/assets/packaging/publication checks, production build.
- [x] HTTP EN/RU/noindex/sitemap, all 50 served asset hashes/MIME and incoming link.
- [x] Native desktop/mobile filters/search/layout/pagination/viewer/bookmarks,
  no overflow/overlap, shared-component regressions, screenshots; reset viewport.
- [x] Update UI index/catalog, docs index/state, plan results and git diff check.

All updates stay local. No paid generation, provider calls, private data publication
or new ownership/consent/licensing authority is introduced.

## Verified result

- /en/photos/nature implemented; 50 cached reference records, 24/24/2 actual pages.
  Pure filtering/search/sort/pagination/geometry tests pass; source dimensions/AI
  labels and measured dominant color retained. No commercial/people/date claims.
- Existing CollectionCard adds optional className/actions only; T13 defaults and
  common preview remain. Shared chrome/filters/modal/scroll/mode/footer reused.
- 219 public/shared tests passed (25 files); final category/model run: 21 tests.
  Types, final full lint and final production build pass. React Compiler dependency
  warning fixed with explicit stable setter dependency, following current official
  React docs; no rule or gate disabled. npm audit found zero advisories.
- Asset, packaging and publication-policy checks passed. HTTP EN/RU return 200,
  one English shell/24 first-page records, Russian empty, both noindex and excluded
  from sitemap. All 50 served WebP MIME/hash checks and incoming T13 link passed.
- Native desktop Square matches 260.25px cells starting x128 at 1280px; Masonry
  has no overlapping rectangles. Actual widths 390/820/1280/1600 checked without
  horizontal overflow. Source+orientation filter, empty/reset, page3, device saved
  ID restoration, download filename, card menu/viewer/arrows/focus passed.
- Mobile filters applied 23 portraits, combined forest search returned 5 matches,
  Square cells were 343×343, stable viewer opacity1 and its close control stayed
  within viewport. QA bookmark reverted, banner resets after reload, temporary
  viewport override reset; category tab marked deliverable.
- Evidence under .cache/material-category-rebuild/: browser-qa.json, desktop.png,
  desktop-square.png, mobile.png, mobile-preview.png, http-smoke.json,
  integration.log, focused-final.log, lint-final.log, build-final.log and audit.json.
- T01–T15 plus T23: 16 of 24 templates implemented locally. Backend/auth/billing/
  provider/jobs, private artifacts and production indexing are unchanged.
