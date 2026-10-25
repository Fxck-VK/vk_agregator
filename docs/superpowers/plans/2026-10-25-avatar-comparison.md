# T08 model/tool comparison

**Scope:** Continue the approved saved-reference rebuild with 08-avatar-comparison. Build `/en/compare/ai-avatar-models`; RU empty; draft/noindex/outside sitemap. One checkout/localhost:7159, no commit/push/deploy.

**Design:** White editorial header (no announcement/login pair), split title/byline/share/hero, large introductory paragraph, desktop sticky nine-item outline and inspiration card, wide readable article with inline outline, eight model sections and nine main chapters, six always-visible FAQ, closing CTA, share/copy controls, three related guides and centered dark footer. Mobile stacks hero; hides sidebar while preserving inline outline. Keep source typography/spacing; fix its broken/duplicate anchors rather than reproducing them. Dates identify this local draft, not false publication.

**Content:** Compare eight existing image-model IDs for avatar concepts: nano_banana_pro, nano_banana_2, gpt_image_2, gpt_image_2_5_flare, seedream_5_0_lite, seedream_4_5, flux_2_pro, imagen_4_0. Extract only `capabilities.application.image` into a typed public snapshot. Preserve unsupported/unknown references/resolutions; include no prices/provider endpoints. This is a documentary comparison of current preview controls, not a live quality ranking, universal model admission or benchmark. Explain output-image vs 3D-rig difference, model-dependent inputs, workflow and review criteria. No copied foreign tool reviews, tests, customer numbers, privacy/training/IP guarantees or platform/legal-policy advice. Responsible-use content links to existing Epifera terms/data policy. Reference artwork is labelled; exclude Magnific author logo and branded cereal/stadium graphics.

**Reuse:** Extend LandingHeader with opt-in `variant="editorial"` and optional home action, preserving overlay defaults; same navigation/search/focus logic, no theme/language controls. Reuse HomeSearch, LandingDialog, Tooltip, BrandLogo/BrandName and centered LandingFooter. New `features/comparison` handles article-specific outline/hero/read-time/share/back-to-top. Do not force dark generator chapter/FAQ presentation onto the source’s white prose layout. Add discovery from the image-generator related section.

**Actions:** All links target existing pages/model selectors or real local anchors. Listen opens a Coming soon explanation, without TTS/Job. Share opens a reversible dialog; explicit Copy uses only current origin+pathname (no query/hash), shows success only after clipboard resolution, and offers selectable text on failure. No automatic external share requests/messages. Browser clipboard QA uses this public local URL only. Back-to-top respects reduced motion. No backend/auth/billing/provider/Job changes.

## Tasks

- [x] Inspect saved HTML, CSS, desktop/mobile/menus and current shared consumers. Source is an editorial eight-tool comparison, not a selector/table UI.
- [x] Add metadata/RU-empty/sitemap/shell/header-default/structure/snapshot/outline/listen/share/copy-failure/anchors regressions; observe intended missing-route/duplicate-shell failures.
- [x] Package five nonbranded reference rasters as lossless WebP with normalized decoded-pixel equality and source/output SHA receipts; capture typed safe model fields with source hash.
- [x] Implement typed English copy/media/catalog, editorial header option, article layout/outline/model sections/six FAQ/CTA/related/share controls and discovery; preserve existing templates.
- [x] Focused and shared-template tests, full lint/types/assets/packaging/publication gates; fix actual first failures without weakening checks.
- [x] Desktop/mobile browser QA: menu/search, anchors/outline/current item, share/copy, listen info, back-to-top, horizontal bounds, loaded media, previous header consumers. Copy rejection/manual selection covered by unit tests. Save screenshots and reset viewport.
- [x] Final production build (normal command; known Windows sharp junction required permission), inline review, docs/index/state/plan, whitespace/status/receipt. No publication.

Use workspace-local TEMP/TMP for Vitest. Shared-template suite includes all completed public pages, header/footer/carousel/media and shell tests. Read-only preview catalog is not runtime price authority. Keep logs/proof ignored in `.cache/comparison-rebuild/`. Work inline; the project-prescribed delegated model is unavailable.

## Result

Implemented locally at `/en/compare/ai-avatar-models`. T01–T08 and T23 layouts are implemented: **9/24**. This is a reference-based draft, not indexed publication or a tested model quality leaderboard. RU remains empty. No commit, push or deployment.

Verification: focused 22 tests/3 files and integration 101 tests/16 files passed; full lint/typecheck, asset validation, packaging/publication checks and normal production build passed. JSDOM reports its synthetic cross-document-navigation limitation; native browser links/anchors were checked separately. Desktop (1280) and mobile (375) were visually checked, including sticky/current outline, menu/search, sanitized share URL and successful explicit clipboard copy, listen information and back-to-top. No browser console errors in the final check. Temporary viewport override reset. A later tablet override did not change the browser viewport, so no tablet-specific verification is claimed.

Evidence is ignored under `.cache/comparison-rebuild/`: red/focused/integration/typecheck/lint/build logs, asset/data receipts, `ready-mobile.jpg`, `ready-share.jpg` and `verification.json`. UI contract is in `web/platform/docs/ui-catalog.md#model-comparison` and its index. No backend, provider admission, auth, billing or Job boundaries changed.
