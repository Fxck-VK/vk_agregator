# T11 documentation and help implementation plan

**Goal:** Continue the user-approved 24-template rebuild with the English Help Center and an account-management article matching saved Magnific reference 11. `/en/ai/docs/account-management` is the reference page; `/en/ai/docs` provides a working article directory. Russian routes stay empty, all draft/noindex/outside sitemap.

**Approved design:** Dark full-height HelpCenterFrame, 72px icon rail, compact breadcrumbs/auth/share row, searchable 320px article navigation, a roomy article with hero, linked table of contents, numbered steps, lists, callouts, questions, previous/next and support panel. Match desktop capture 0254 and mobile 0278, drawer 0198 and scrolled 0199. Reuse only matching HelpCenterFrame/Drawer, HelpIcon, localized Link, BrandLogo and EditorialShare. Own help-specific navigation/body/styles preserve academy/legal defaults. No foreign scripts, app ads, account limits or unsupported authentication promises. Data is typed original Epifera copy with seven real articles in four groups. No raw HTML or runtime CMS.

**Constraints:** Current checkout and existing localhost:7159 only; no new dependencies, worktrees, preview ports, commit, push, deployment, backend/auth/billing/provider changes or paid requests. User already approved reference-driven English reconstruction; no new design approval gate is needed. Use writing-plans/executing-plans and meaningful red/green tests inline. The prescribed delegated model is unavailable, so no substitute agents are launched.

## Files and interfaces

- Create `src/i18n/public/documentation-en.ts`: labels, four groups, seven typed articles with stable slugs and section IDs, original text, links, steps/list/callout/question block variants. `src/features/documentation/documentation-model.ts` exports `documentationPath`, `findDocumentationArticle`, `documentationNeighbours` and scoped title/section/body search.
- Create `DocumentationCenter.tsx`, `DocumentationNavigation.tsx`, `DocumentationArticle.tsx`, `Documentation.module.css` under `src/features/documentation`. Client adapter owns menu search and share; article renderer uses React text; directory links to actual articles.
- Add `/ai/docs` and `/ai/docs/[slug]` page/publication files and tests. Unknown slug rejects via notFound; exact slug declarations retain draft/noindex for both locales. Add own-shell routing only for `/ai/docs` and its children. Extend HelpCenterFrame kind union with `documentation` without changing defaults.
- Add a discoverable Help Center link in existing academy navigation; update UI catalog/index, docs index and state.

## Tasks

- [x] Inspect reference screenshots, saved DOM and existing help/academy/legal consumers. Keep local page source access filesystem-only when browser file protocol is unavailable.
- [x] Write and run meaningful failing tests: locale/publication identity; unknown slug; article anchors; search titles/body, case and empty states; collapse then search; current navigation; bounded neighbours; native local links; explicit share; mobile selection/Escape semantics; own-shell matching.
- [x] Implement typed data/model and safe body/navigation renderer, frame adapter, route/declarations and local discovery link. Copy is checked against current AccountSecurity, artifact-retention and publication contracts.
- [x] Run focused tests plus affected academy/legal/shared/public templates. Fix actual defects, preserving mandatory checks and previous defaults.
- [x] Full types/lint, asset validation/packaging/publication and production build. Reuse prescribed Windows/TEMP workarounds if needed; inspect every output.
- [x] Browser QA using the existing localhost server: desktop/mobile geometry, internal scrolling/anchors, title/body search, current entry, neighbours, share, both drawers/focus/Escape, academy/legal regression, unknown route and no horizontal overflow. Save proof under ignored `.cache/documentation-rebuild` and reset viewport.
- [x] Inline review, docs/state/plan and whitespace/status. Report local URL and verified checks; no publication claims.

## Verification commands

From `web/platform`: `npx vitest run src/features/documentation/documentation-model.test.ts 'src/app/(public)/ai/docs' src/i18n/public/LocalizedPublicShell.test.tsx`, then affected academy/legal/help-drawer/editorial-share tests and all public-template route tests. Run `npm run typecheck`, `npm run lint`, `npm run validate:assets`, `npm run test:packaging`, `npm run test:public-pages`, `npm run build`. Bind TMP/TEMP to ignored local `.cache` for sandbox-safe compiler/worker temporary files.

## Result

Completed locally: `/en/ai/docs` and seven article routes, including account-management. Four real groups, title/section/body search, article anchors and initial hashes, numbered steps, informational callouts, questions, bounded previous/next and support mailto. Reused help shell/drawer and share controls; legal/academy defaults unchanged, AcademyNavigation now links to Help Center. All eight publications remain draft/noindex, Russian pages empty, unknown slug returns 404. **T01–T11/T23 implemented: 12/24.** No commit, push or deployment.

Verification: red observed for absent routes/model and duplicate generic shell. A duplicate banner from the article hero and joined directory-link words were caught and fixed before the passing runs. Focused help/academy/legal/shared suite: 54 tests in 8 files. Final public/shared integration: 142 tests in 19 files. Full typecheck/lint, assets, packaging and four publication-generator tests passed. Fresh npm audit reported 0 vulnerabilities. Normal production build passed after the known Windows sandbox sharp junction restriction required the same command outside sandbox; TypeScript also passed during build. Two JSDOM synthetic document-navigation notices were checked through real browser navigation; no check was waived.

Browser: desktop 1280 and mobile 390. Main heading x456/y180 desktop, x36/y162 mobile; mobile contents heading y410.6 against reference y405. Verified body search, empty state, collapsed-group restoration, local directory/article links, bounded neighbours, current entry, initial/top reset, native article anchor (section y100 vs reading y68, window scroll 0), share/copy clean origin+pathname, mobile global/help drawers with inert background, search focus and Escape return, selection closing the drawer, no horizontal overflow and zero console errors. Viewport override reset. Existing legal/academy tabs retain one header and no overflow. HTTP smoke passed 11 cases: eight English screens, two empty Russian screens, unknown 404; every existing new route noindex.

Inline review covered route/publication identity, known slugs, safe React-only content/links, search scoping and remounts, native anchors, frame defaults, drawer cleanup, share semantics and copy against existing AccountSecurity and retention contracts. No new dependencies, media, provider/billing/auth/backend or persistence. Existing uncommitted changes retained. Evidence under ignored `.cache/documentation-rebuild/`: desktop/mobile/anchor/search PNGs, npm-audit JSON, build log and HTTP-smoke JSON.
