# English AI tools overview (reference 02)

**Goal:** Build `/en/ai` from the saved English Magnific tools overview and its desktop/mobile interaction captures. The user approved this implementation; no commit, push or deployment is requested.

**Design:** Preserve the source section order: blue sculptural hero and five shortcut tiles; eight-category tool directory; editor-plugin carousel; six model-style tabs; eight AI-client connection tabs; four audience filters with five rotating scenario cards; twelve documentation cards; sixteen visual tool cards; closing CTA and centered footer. Match the 1264px desktop and 375px mobile captures, local display font, spacing, responsive card widths and keyboard interactions.

**Architecture:** Add `features/ai-tools-overview` with a page composition, directory, model/connection panels and editorial sections. English copy lives in `i18n/public/ai-tools-en.ts`, extracted reference artwork in `ai-tools-reference-en.ts` and `public/assets/images/ai-tools/`. Use the shared header/search/modal/brand/model-icon/carousel only where the reference matches. Extend `LandingFooter` with an opt-in centered layout and `LandingCarousel` with opt-in bounded controls; their existing defaults remain unchanged.

**Product limits:** Link available actions to existing Epifera routes and model IDs. Keep the full reference layout, but label unimplemented plugins, MCP, flows and 3D tools as coming soon and open explanatory previews rather than pretending that a connector or editor is live. No foreign endpoints, tracking, prices, certifications, subscriber totals, paid requests or copied authorization code. Team scenarios describe individual creative workflows, without promising real-time account collaboration.

**Publication:** English only; `/ru/ai` stays empty, both routes draft/noindex and absent from sitemap. Add the existing publication declaration contract. The current local server remains localhost:7159, single active checkout; preserve image-generator and legal work.

## Tasks

- [x] Inspect source HTML, full-page desktop/mobile screenshots and recorded tabs/filters; identify reusable components and preserved media dimensions.
- [x] Add route/interaction/default-component regressions and observe the expected failures before implementation.
- [x] Package 22 existing reference images with provenance; extract tool text into typed data and map every active action to a real Epifera destination.
- [x] Implement hero, directory, style/client tabs, plugin/audience/showcase carousels, documentation grid, CTA and centered footer.
- [x] Connect `/ai`, own its public shell and publication metadata; preserve the existing page defaults and legal routes.
- [x] Verify desktop/mobile, all eight tool tabs, six styles, eight clients, audience ordering, carousel bounds, dialogs/focus, menus, local anchors, no overflow/broken images and reduced motion behavior.
- [x] Run focused tests, lint/types, assets/packaging and production build. Inspect final diff/status and update UI docs/current context with actual local status.

## Checks

Use workspace-local TEMP/TMP for Vitest on Windows. Run `npx vitest run 'src/app/(public)/ai/page.test.tsx' src/components/public/landing src/i18n/public/LocalizedPublicShell.test.tsx src/i18n/seo.test.ts`; then package typecheck/lint, asset and packaging checks, and production build. The Windows sandbox does not allow Turbopack's sharp junction; run that same build with the required filesystem permission rather than weakening source checks. Browser checks use the existing IAB viewport capability, resetting its temporary size before completion. No live provider/payment/connector calls or clipboard mutation during browser QA.

## Local verification — 2026-10-25

- Initial regressions failed on the missing route, duplicate public shell, absent centered-footer contract and carousel viewport marker; the implementation resolves these failures.
- 48 tests across eight files passed, including the overview, generator, legal routes, shared footer/carousel, public shell and SEO declarations. Full ESLint, typecheck, asset validation, packaging checks and production build passed. Private command output: `.cache/ai-tools-rebuild/tests-final.log` and `build-final.log`.
- Browser checked at 1264×900 and 375×812: all category/style/client tabs, keyboard wrapping, all four audience orders, carousel scrolling, planned preview/Escape/focus return, search with Russian model name, mobile menu, mobile footer and no document overflow/broken images. Reduced-motion rules and bounded carousel controls are covered by source review/regressions. No live generation or connector calls were made.
- 22 local images total 1,490,134 bytes; private media receipt records original dimensions and SHA-256. Temporary scripts and original reference archives stay outside the runtime bundle.
- Inline integration review: existing header/footer/carousel defaults, legal data and model admission remain intact. The repository-prescribed delegated model is unavailable, so no independent-agent review is claimed.
- Local implementation only. No commit, push or deployment. Prior legal/image-generator changes are preserved.
