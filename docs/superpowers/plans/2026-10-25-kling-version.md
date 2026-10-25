# Kling 3.0 model-version template plan

**Goal:** Implement approved SEO template T05 at `/en/ai/kling/kling-3-0`, using saved Magnific reference 05. English only; Russian route empty. No commit, push or deployment.

**Design:** Match the archived hero/breadcrumb/status chip/CTA, three-cell trust band, eight-item mixed gallery, four-section sticky navigation, version summary, nine-row specification table, three image-led scenarios and comparison callout, three archived research figures, five FAQ, closing banner and related links. Use the existing Epifera header/footer/search, GeneratorLanding layout, and model-family media viewer where the source structure matches.

**Architecture:** Move the shared model presentation CSS and mixed-media component into `components/public/generator-landing`, retaining family exports/defaults. Add `features/model-version` and separate typed English copy/media data. Extend media with a local asset-base selector and page title/test ID. Keep current family behavior covered by regression tests. Link Kling 3.0 names from the family table/timeline to the new detail page; launch actions keep the existing `video_kling_v3` catalog route.

**Limits:** Draft/noindex, absent sitemap; preserve prior local work and localhost:7159. The specs use existing `capabilities.application` data; no admission, provider mapping, billing or job changes. No foreign prices/free quotas/certifications. Examples and research remain attributed Magnific references, with no provider calls or claims of Epifera results. No prompt entry/persistence, tracking, foreign JS or paid calls.

## Tasks

- [x] Inspect reference HTML/CSS, 1264px desktop and 375px mobile screenshots, FAQ states and existing components/data.
- [x] Add route/metadata/spec/safe-action/media and own-shell regressions; observe intended failures before implementation.
- [x] Package reference 05 images/video with original-byte provenance and extract version-only CSS/media/research data.
- [x] Extract matching model media/styles with preserved family defaults; implement English composition, model specs, scenarios, research, FAQ, closing and related links.
- [x] Add publication declaration, shell ownership and family-to-version navigation.
- [x] Check desktop/mobile, all section anchors, gallery/video controls, pause/reduced motion, Escape/focus, menu/search, five FAQ and overflow/broken media; recheck family after extraction.
- [x] Run focused/integration tests, full types/lint, asset/packaging checks and production build. Update docs/current state, review diff/status, save screenshots and local receipt.

## Verification contract

Vitest uses workspace-local TEMP/TMP. Run the version, family, image-generator, overview, legal, Landing, shell and SEO tests. Run package typecheck/lint/validate:assets/test:packaging/build. The same Turbopack build may require filesystem permission for sharp's Windows junction. Use CUA for browser verification, save private screenshots, reset the viewport. Work inline because the project-prescribed delegated model is unavailable.

## Local outcome — 2026-10-25

- Expected red observed for the missing version route and duplicate public shell. Final regression suite: 66 passing tests across ten files, including the version, family defaults, existing generator/overview/legal, shared Landing components and metadata. Full lint/typecheck, asset and packaging validation, production build and `git diff --check` passed.
- Shared `ModelLanding.module.css`, `ModelMedia` and `GeneratorChapter` now live under `components/public/generator-landing`. Family aliases/default titles/asset roots and separate launch actions are preserved. No provider/admission/billing/auth/consent change.
- Desktop 1264×900 and mobile 375×812 checked. All four section anchors, nine specs, five FAQ, gallery/video decoding, mute defaults, Escape/focus return, pause, mobile menu/search, footer and family↔version links work. No page overflow or broken inspected media. Browser QA caught keyboard focus on an off-screen moving card (left < 0); `:focus-visible` now removes gallery animation and the focused card is visible at left 0. Reduced motion remains enforced by the existing media/CSS rules.
- 12 images and one local WebM preserve original bytes, 1,338,029 bytes total. Media provenance/SHA-256, command logs, browser receipt and desktop/mobile screenshots are in the private `.cache/kling-version-rebuild/` folder. Final user-facing copy avoids API/runtime terminology and foreign commercial claims; research is attributed to Magnific.
- Layouts T01/T02/T03/T04/T05/T23 are now implemented locally. English version only; Russian empty. Draft/noindex, no sitemap entry. No commit, push or deployment.
