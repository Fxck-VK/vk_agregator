# English image generator landing rebuild

> **For agentic workers:** Execute inline in the single active checkout. The user approved removal of both old demos and a new English implementation based on reference 03. No commit, push or deployment is requested.

**Goal:** Replace the old image-generator demonstration with a faithful responsive English landing page, using the saved desktop/mobile reference and interaction captures.

**Architecture:** The route renders the new `ImageGeneratorLanding` only for English; Russian stays empty and both remain draft/noindex. New page-specific components implement the prompt, gallery, four create panels, section navigation, model comparison, family carousel and editorial sections. Reuse the existing landing header/footer because both archived references use the same navigation/footer; reuse other controls only where their actual appearance/behaviour agrees.

**Tech stack:** Existing Next/React application, CSS modules, typed English copy, local archived assets, existing modal/media/brand primitives. No added dependencies or foreign scripts.

## Constraints

- Keep the active checkout and localhost:7159; preserve reference archives and shared imagery.
- First remove ImageToolLanding, its private stylesheet and obsolete ru/en demo dictionary.
- Rebuild /en/ai/image-generator; /ru/ai/image-generator has no old demo or synthesized translation.
- Preserve draft/noindex and sitemap exclusion. The working private generator remains a separate route.
- Rendering and navigation do not submit jobs or paid calls. A prompt action leads to the existing generator, without automatic submission or local-storage prompt persistence.
- Source study statistics stay attributed to Magnific; foreign prices, free allowances, certifications and unsupported product features must not become Epifera claims.
- Scope styles to this page. Preserve shared typography tokens and existing accessibility boundaries.

## Tasks

- [x] Remove the old demo and its exclusive copy/styles; retain route/publication metadata as a blank draft until replacement.
- [x] Extract section geometry, local media and interaction data from saved HTML/CSS/screenshots. Compare archived home/header/footer before reuse; document asset provenance.
- [x] Add typed English copy and page-specific CSS/components for all reference sections: hero/prompt, gallery, sticky rail/mobile chips, create modes, model table/auto card/family carousel, six use cases, workflow, trust, attributed study, plans, eleven FAQ entries, closing/related links/footer.
- [x] Add route tests for en-only content/noindex, meaningful interaction tests for create modes, gallery, prompt handoff and absence of paid requests; update public shell routing tests.
- [x] Run focused feature/route/shared-contract tests, lint, types, assets and production build. Inspect desktop/mobile in the existing local server and compare screenshots, horizontal overflow, menus/dialog focus, section links, responsive tables, FAQ and reduced motion.
- [x] Update UI index/catalog and active routing context, preserving the actual implementation/publication status. Review changed source and git status.

## Verification

Record actual results after implementation. The archived source has a 1264px desktop capture and a 375px mobile capture, including the full editorial body and eleven FAQ states.

### Verified locally on 2026-10-24

- Old demo and its exclusive RU/EN dictionary/CSS removed; new English route owns its chrome, Russian route empty. Both browser documents expose noindex/nofollow. Sitemap exclusion tests pass.
- 129 tests passed across 11 focused route, private-generator, publication, dictionary, header and typography files. The final route check passed all 9 tests after the closing CTA review. Initial intended behavior tests failed against the blank retired route, then passed after implementation.
- Full ESLint and subsequent changed-file checks passed, as did typecheck, asset validation, packaging assertions, and the final production build. No dependencies added.
- Desktop 1264×900 and mobile 375×812 inspected in the existing localhost:7159 server. All seven desktop chapter anchors tracked correctly with no document overflow or broken loaded images. Mobile table scroll, sticky chips, menu, three alternate creation compositions, gallery/close/focus restoration and FAQ were checked. Reduced-motion behavior is provided in CSS; OS preference was not changed during this check.
- Refreshed 53 public source images (58 total local images including existing hero/default create panel) after discovering 256px archived previews. Additional create modes match the source reference/6-image/edit-triptych layouts. Asset origin and draft-only use recorded in the local media README.
- Reviewed route ownership, private draft consumption/TTL, no paid requests, modal lifecycle, CTA destinations and source-only claims inline. The prescribed gpt-5.5 reviewer is not available in this session; no substitute agent or independent-review claim was used.
- UI index/catalog, documentation map, active context, documentation links and referenced media paths checked. Core auth, billing, jobs and provider boundaries unchanged. No commit, push or deployment performed.
- Evidence: `.cache/image-generator-rebuild/epifera-desktop.jpg`, `epifera-mobile.jpg`, `epifera-full-desktop.jpg`; source state data and public image download receipt are in the same ignored folder.
