# T09 article implementation plan

**Goal:** Rebuild saved Magnific reference 09 as an English Epifera guide at `/en/blog/ai-art-prompts`, with an empty Russian route and draft/noindex publication.

**Architecture:** The inspected reference shares the editorial header, split hero, byline, sticky outline/inspiration, share island, related cards and footer with T08. Extract that exact common shell into `components/public/editorial`; preserve T08 content, defaults and behavior through its tests. Render T09 from a typed article record with safe React text/links, five independent sections, three inline illustrations, lists and four example briefs. No copied author identity/publication date or foreign scripts.

**Execution:** Use executing-plans inline. The prescribed delegated model is unavailable. Existing approved 24-template spec/reference and current user instruction authorize implementation; no new approval, checkout, port, dependency or publication. One server on localhost:7159.

## Tasks

- [x] Inspect reference HTML/CSS, desktop/mobile screenshots and existing editorial consumers. Reference has no large intro band, no FAQ and no inline contents block; preserve those differences.
- [x] Add meaningful regressions in `src/app/(public)/blog/ai-art-prompts/page.test.tsx` and shell tests: draft/noindex/outside sitemap, RU empty, one shell, five sections, complete anchor targets, three inline images, four examples, safe links, independent shared-control labels/clipboard rejection and no generation side effects. Observe expected missing-route/shell failures.
- [x] Package hero and three inline rasters from reference 09 as lossless WebP, preserving normalized decoded pixels; record source/output hashes. Reuse already packaged inspiration and related artwork; exclude foreign author, branded film art, scripts and unused media. Add `art-prompts-reference-en.ts` and attribution README.
- [x] Extract shared `EditorialPage`, `EditorialControls`, types and matching CSS. Props provide breadcrumbs, title, date, read time, outline entries, hero/inspiration/related data, control labels and optional intro. Adapt T08 to the same public interfaces without changing its article data or model controls. Preserve pending/success/manual-copy semantics, focus return, native anchors, reduced motion and all default headers.
- [x] Add typed `art-prompts-en.ts` and `features/editorial-article/ArtPromptsArticle.tsx`/`ArticleBody.tsx`. The body accepts safe inline text/strong/internal links and paragraph/list/image blocks; no raw HTML. New record includes five headings, own practical writing guidance, four fictional example briefs and explicit reference-art captions. Prompt text does not imply output resolution; actual dimensions remain workspace settings. Actions only navigate; no Jobs, staging or API/provider calls.
- [x] Add route `page.tsx`/`publication.ts`, exact own-shell rule and discovery from T08 related guides. Keep every new locale draft/noindex and outside sitemap; English only.
- [x] Run focused article/comparison/header/shell tests; all existing public-template integration tests; full lint/typecheck, asset/packaging/publication gates and normal build. Use workspace-local TEMP/TMP, logs in `.cache/article-rebuild/`. Known sharp Windows junction permission may require routine escalation, without changing the build command.
- [x] Visually compare desktop/mobile hero, article/list/image spacing and bottom cards; check menu/search, outline anchors/current item, share sanitized URL/copy, listen info, back-to-top, loaded images, bounds and previous T08 page. Save proof, reset viewport, review scoped diff, update UI index/catalog, docs index/state/plan and final receipt/status. No commit/push/deploy.

## Interfaces and verification

`EditorialPage({page,children})`: `page` is a readonly `EditorialPageData` record with unique `id`, `kind`, title/breadcrumbs, author/date, read-time, controls, sections, hero, inspiration and related cards. `EditorialImage` uses a local absolute `/assets/images/` src and alt; width/height are supplied for body illustrations. Optional `intro` retains T08's band, omitted for T09. `EditorialOutline({entries,label})` takes arbitrary section ids rather than comparison imports. All clipboard operations require an explicit click and use origin+pathname only. `EditorialShare({labels,onClose})` uses the shared focus-trapped `LandingDialog`.

Commands from `web/platform`: `npx vitest run <focused files>`, shared public-template suite, `npm run lint`, `npm run typecheck`, `npm run validate:assets`, `npm run test:packaging`, `npm run test:public-pages`, `npm run build`. Success requires actual green output and browser observation, not test implementation alone.

## Result

Completed locally at `/en/blog/ai-art-prompts`. T01-T09 and T23 now have reference-based first-page implementations (10/24). The English guide uses the shared editorial shell plus its own typed article content, without the comparison intro/FAQ. RU stays empty; both draft/noindex outside sitemap. No publication or backend/provider/auth/billing/Job changes.

Verification: expected red missing-route/duplicate-shell tests were observed. Focused 32 tests passed before the additional visibility regression. Final integration passed 109 tests across 16 public/shared files; the remaining MediaVideo file passed 3 tests separately (112 unique tests/17 files total). Full lint/typecheck, assets, packaging, four publication tests and normal production build passed. Native anchors/share were verified in the browser; JSDOM's synthetic navigation limitation is not treated as a browser result. One extra test invocation from the repository root used the wrong Vitest config; the final suite ran from web/platform as required.

Desktop 1280 and mobile 375 were visually checked against saved reference screenshots: split hero, body/list/image layout, sticky current outline, related cards, mobile menu/search/listen info, native section anchors, explicit sanitized share/copy, back-to-top and horizontal bounds. Shared T08 browser regression retained eight models, nine chapters, one header and no overflow. Final article console check had zero errors. Browser viewport override reset was requested; stale browser-tool sizing required refreshing the control session before the actual mobile observation.

Inline review covered route/publication ownership, server/client boundaries, safe text rendering, independent outline ids/labels, pending/failure clipboard behavior and unchanged generation/auth/billing boundaries. The prescribed delegated model remains unavailable. Source changes were complete before the successful production build; later changes only update documentation/receipts.

Ignored evidence: `.cache/article-rebuild/` contains red/focused/integration/media/lint/typecheck/build logs, reference crops, asset hash/pixel receipt and ready-desktop/body/related/share/mobile/mobile-body/mobile-image JPG proofs. UI contracts/index, docs index and current state updated. No commit, push or deployment.
