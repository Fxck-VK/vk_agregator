# Kling model-family template implementation plan

**Goal:** Implement T04 at `/en/ai/kling` from saved Magnific reference 04, preserving the existing approved English-first design direction. No commit, push or deployment.

**Architecture:** The page owns its matching marketing header/footer. Shared GeneratorLanding styles and section navigation are extracted from the image-generator implementation because both archived pages use the same GeneratorLanding layout. Family-specific versions, timeline, comparison and research live in `features/model-family`; English copy and packaged local media have separate typed data modules. Existing model admission, billing, auth and job submission do not change.

**Constraints:** One checkout, localhost:7159, no translation; RU route empty. Both locales draft/noindex, absent sitemap. Active links use existing Epifera routes/model IDs. Table values describe the existing application capability snapshot, with no copied Magnific prices or unsupported live claims. Unlisted versions/integrations open an explanatory preview. Archived research and examples are attributed to Magnific. No provider/payment/connector calls, foreign JS, clipboard mutation or prompt persistence.

## Tasks

- [x] Inspect archived HTML, CSS, 1264px desktop and 375px mobile captures, gallery detail and twelve FAQ states; inspect existing shared components and model application capabilities.
- [x] Write route, own-shell, FAQ/gallery, draft metadata and safe model-link regressions; observe expected failures before implementation.
- [x] Package the saved artwork, posters and WebM clips locally, preserving dimensions and SHA provenance; extract media/research data without third-party scripts.
- [x] Extract identical shared layout CSS/navigation without changing the existing generator contracts. Build hero, trust band, mixed gallery, versions, recommendations, timeline, comparison clips, workflow, attributed research, cost cards, FAQ, closing and related links.
- [x] Add `/ai/kling`, publication declaration and discoverability from existing overview/home navigation. Preserve all prior local work.
- [x] Verify desktop/mobile geometry, local anchors/active section, media pause/play and sound, preview/modal focus/Escape, FAQ, table scroll, menu/search and responsive overflow.
- [x] Run meaningful route/shared-component tests, full typecheck/lint, asset/packaging validation and production build. Review final diff/status and update UI index/catalog/current context.

## Verification

Use workspace-local TEMP/TMP for Vitest. Test new family route plus generator, overview, legal, shared Landing components, public shell and SEO metadata. Run package typecheck, lint, validate:assets, test:packaging, build. On Windows run the same production build with filesystem permission for the sharp junction, without weakening checks. Use CUA for browser QA on the existing server, save private screenshots, reset temporary viewport. Work inline; the project-prescribed delegated model is unavailable.

## Local outcome — 2026-10-25

- Expected red observed for missing route and duplicated public shell. Integration tests also exposed the inherited homepage-only anchor and a lazy-loaded modal clip; both were corrected. The final media cleanup runs before restoring DOM mocks, so the passing test log has no unsupported jsdom media warnings.
- Final verification: 57 tests across nine files; full ESLint, TypeScript, assets validation, packaging and production build passed. Output is retained privately in `.cache/kling-family-rebuild/`. No security/provider/identity/billing gate was changed or bypassed.
- Browser: 1264×900 and 375×812; all twelve FAQ expand, gallery video loads locally, Escape restores focus, comparison files decode and sound toggles explicitly, motion pauses, active navigation/anchors work, the table scrolls with keyboard without page overflow, mobile menu and Russian Kling search work. The previous image generator still renders with its shared font/layout after extraction. No broken images or media errors on inspected loaded clips.
- 24 images and 10 clips preserve original bytes (17,575,655 bytes total), with SHA-256 receipt and provenance. Main final screenshots: `ready-desktop.jpg` and `ready-mobile.jpg` in the private cache.
- Inline review confirmed model IDs/application snapshots and unavailable reference entries are distinct, no foreign launch URLs or static foreign prices, and no generation submits occur from the landing. Five local SEO layouts are now implemented: T01/T02/T03/T04/T23. Translation, indexable publication and release remain separate.
- No commit, push or deployment. Previous local generator/legal/overview changes are preserved.
