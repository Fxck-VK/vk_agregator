# Tattoo task-solution template plan

**Goal:** Build T06 at `/en/ai/tattoo-generator` from the saved Magnific reference 06. English only; Russian route empty; draft/noindex and outside sitemap. No commit/push/deployment.

**Design:** Dark split hero with three text fields, four style presets, three placement presets and five selectable artwork examples; light body with four-step accordion/image, three alternating feature rows, seven-card workflow carousel, model/tool directory, eight AI-client tabs, three cost/workspace cards, eight visual workflow cards, fifteen FAQ and centered footer. Match 1264px desktop and 375px mobile captures. Preserve the existing one-checkout localhost:7159 preview.

**Architecture:** Add `features/task-solution` with focused hero/form, editorial/directory and composition components; English copy and packaged reference media live in `i18n/public`. Reuse `LandingHeader/Footer/Dialog/Carousel`, `InputSurface`, `Button`, existing smart search, and the T02 connections component because its layout/behavior matches this archive. CSS is scoped to the new page. Add a discovery link from the image-generator related links.

**Action contract:** Explicit valid form submit composes idea/style/placement into the existing transient `stageImagePrompt` handoff and navigates to `/app/image`. No Job, provider call, charge, URL prompt, browser storage or automatic upload. The existing image generator controls model/settings/output count/cost and paid confirmation. Do not promise a fixed four-image batch, guaranteed placement or production-ready stencil. Unimplemented workflows/MCP are explanatory previews; prices, certificates, customer logos/testimonials and training claims from Magnific are not Epifera facts. Saved artwork is attributed; replace/review rights before indexable publication.

## Tasks

- [x] Inspect source HTML, CSS/classes and recorded desktop/mobile/preset/gallery/FAQ states; identify existing components and safe prompt navigation.
- [x] Add metadata/own-shell/form/presets/preview/steps/FAQ/links regressions and observe intended failures (missing route and duplicate shell).
- [x] Package 24 reference images with dimensions/source-output SHA-256 and typed data, excluding customer logos/scripts. Existing asset policy rejects JPEG: 12 JPEGs use lossless WebP with decoded-pixel equality; PNG/WebP retain bytes and originals stay in references.
- [x] Build the complete hero/body, validated form, gallery, steps, features, carousel, directory, connections previews, cost guidance, workflow mosaic and FAQ.
- [x] Connect route/publication/shell and discovery from existing image-generator page; preserve all previous work.
- [x] Verify desktop/mobile against captures: five examples, presets, text editing/validation, safe submit navigation, accordion keyboard, bounded carousel, eight clients, fifteen FAQ, menu/search, modal focus/Escape and no overflow/broken media. Shared FAQ presentation needed stronger page scoping; corrected and checked via browser computed styles. Hero image sizes account for the crop to avoid undersized source selection.
- [x] Run focused/integration tests, full types/lint/assets/packaging and production build; update UI docs/state, review diff/status and save screenshots/receipts.

## Verification

Use workspace-local TEMP/TMP for Vitest. Include the new solution and current generator/model/family/overview/legal/shell/SEO/Landing tests. Same production build may need filesystem permission for sharp's Windows junction. Browser checks use CUA on the existing server, with no paid generation and a temporary viewport reset before completion. Work inline; the project-prescribed delegated model is unavailable.

Result: 74 tests across 12 files passed; types, full lint, asset validation, packaging, publication-manifest tests and final production build passed. CUA checked desktop 1264px and mobile 375px, no horizontal overflow or console errors, one header/footer, 7 flow cards, 8 workflow cards and 15 FAQ. A synthetic brief transferred idea/style/placement to `/en/app/image` without URL prompt or generation. Proof/logs are ignored under `.cache/tattoo-solution-rebuild/` (`ready-desktop.jpg`, `ready-mobile.jpg`, `ready-details.jpg`, `assets.json`, `integration.log`, `build-final.log`). No commit/push/deployment.
