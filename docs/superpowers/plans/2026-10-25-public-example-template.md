# T14 public result example implementation plan

Status: completed locally; no commit, push or deployment.

> Execute inline with the executing-plans workflow. The established single-checkout,
> reference-driven design is approved by the user; no new worktree or publication.

**Goal:** Rebuild reference 14 as `/en/inspiration/examples/aesthetic-flowers`.

**Architecture:** A typed, static PublicExample record drives a page-owned detail
layout and matching shared reference chrome. Public cached artwork remains separate
from private account artifacts. No account, provider, Job, billing or backend change.

**Tech stack:** Existing Next/React/TypeScript/CSS Modules, shared media/modal/carousel
components; Sharp/JSDOM only for local reference inspection and media packaging.

## Constraints and source decisions

- Work only in the active checkout; reuse localhost:7159. English implementation,
  empty Russian counterpart; both draft/noindex/outside sitemap. No commit/push/deploy.
- Source: saved `14-ai-image.html`, original source/captures under
  `.cache/magnific-ui-capture/14-ai-image/`. Actual source is
  `https://www.magnific.com/free-ai-image/aesthetic-wallpaper-with-flowers_394975120.htm`.
- Desktop default capture is scrolled to the footer. Use its markup, mobile capture
  and loaded carousel states to recover the main layout and related cards.
- The cached main image is 606×1060, despite source HTML declaring 3072×5376.
  Display/download actual preview dimensions. Never claim an unavailable original.
- Source labels it AI-generated; its model and original prompt are not provided.
  Record origin/provenance explicitly, modelVersionId=null, promptPublicationAllowed=false,
  moderationState=reference-only and reference-use scope. Render a separate editorial
  suggested prompt; do not claim an Epifera/user-generated artifact or consent grant.
- Preserve the matching rail/topbar/mobile navigation through a small ReferenceChrome
  extraction from T13. Keep T13 search semantics and layout unchanged. Reuse matching
  footer, MediaImage, MediaPreviewDialogTemplate, LandingDialog/Carousel and PopoverPanel.
- No original scripts/trackers/avatars, remote runtime assets, foreign entitlements,
  subscriber counts or unimplemented editing controls.

## Task 1 — record, policy and regression contract

Files: new `i18n/public/public-example-en.ts`, reference data/media manifest, route
`app/(public)/inspiration/examples/aesthetic-flowers/{page,publication,page.test}`.

- [x] Add failing route/behavior tests and exact shared-shell isolation test.
- [x] Package the main and 18 cached related reference images from the existing
  cache. Keep source hashes/dimensions; verify lossless decoded equality. Export
  actual local PNG/WebP previews, with brand-derived download filenames.
- [x] Implement the typed record and explicit publication policy.

## Task 2 — detail UI and matching shared chrome

Files: `components/public/reference/ReferenceChrome*`, dictionary and collection
adapter; new `features/public-example/*` for page, artwork/form, actions and viewer.

- [x] Extract only matching shell parts. Preserve T13 labels, sizes and behavior.
- [x] Build large artwork and 360px desktop action panel; stack on mobile.
- [x] Implement information, format menu/downloads, PNG clipboard/error, device-only
  saved public ID, canonical share/fallback and related-section focus navigation.
- [x] Implement suggested-prompt copy and variation form/presets. Handoff only in
  memory to existing /app/image; never submit a generation or put prompt in URLs/storage.
- [x] Add two bounded native-scroll carousels with actual loaded related images,
  tag filtering and shared fullscreen media navigation. Modal backgrounds inert;
  Escape/backdrop/keyboard/focus return verified, including menu → dialog transitions.
- [x] Link this sample from the existing public collection without changing its
  reference members or claiming the flower artwork belongs to its cinematic series.

## Task 3 — verify and document

- [x] npm audit before long checks; focused tests, shared/public regression suite,
  types/lint/assets/packaging/publication policy and production build.
- [x] HTTP smoke EN/RU, noindex/sitemap and all media hashes/download MIME types.
- [x] Native browser desktop/mobile geometry, menus, preview, carousels, clipboard,
  search, device bookmark and prompt handoff; save screenshots, reset viewport.
- [x] Update scoped UI index/catalog, docs index/state, result checklist and diff check.

Review focus: no invented original parameters, no private artifact exposure, correct
preview sizes, actual working actions, preserved T13 defaults, no deployment claim.

## Result and verification

- Main 606×1060 reference, 9 series cards and 9 similar cards implemented. One final
  source thumbnail was absent from all existing caches and is omitted instead of
  creating an empty card or fetching new assets. 19 WebP files plus a PNG download
  are locally served; decoded pixels and served MIME/SHA-256 were verified.
- The existing asset policy does not accept JPEG. Downloads therefore offer PNG
  and WebP without weakening the validator or bypassing it with a new route.
- Shared ReferenceChrome and deviceBookmarks preserve T13 behavior/key. Optional
  carousel controls=overlay matches T14; existing default above stays unchanged.
- 197 public/shared tests passed across 23 files. Types, full lint and final scoped
  lint passed; zero npm audit advisories. Asset/packaging/publication tests passed.
  Final production build succeeded after final breakpoint/brand-copy adjustments.
- HTTP: EN/RU 200, English one shell/18 related cards, Russian empty, noindex and
  sitemap exclusion, all 20 file hashes/MIME types, incoming collection link and
  the previous 48-card collection preserved. Evidence: http-smoke.json.
- Browser checked actual 390/820/1280/1600 widths without horizontal overflow,
  desktop and stable 390×844 fullscreen viewer, Information/format/More menus,
  focus return, carousels/tags, Russian model search, PNG/prompt copy, share and
  device bookmark restoration. QA bookmark was reverted; the temporary T13 tab
  was restored. Browser viewport override reset; new detail tab kept deliverable.
- Native variation navigation reached /en/app/image with the entered instruction
  and suggested prompt, empty URL query and no automatic generation.
- Evidence: .cache/public-example-rebuild/{browser-qa.json,desktop.png,mobile.png,
  mobile-preview.png,integration.log,build-final.log,lint-final.log,audit.json}.
- T01–T14 plus T23: 15 of 24 templates implemented locally. Private artifacts,
  backend/auth/billing/provider/job flows and public indexing are unchanged.
