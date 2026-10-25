# T17 Integration Landing Implementation Plan

Status: completed locally. Local only; no commit, push or deployment.

**Goal:** Build `/en/mcp` from saved Magnific reference 17, with English content,
empty Russian counterpart and explicit draft/noindex outside sitemap.

**Architecture:** Typed planned Integration record, own reference-specific hero,
client tabs/guide, capability grid, five alternating feature rows, MCP/Agents
comparison, six selectable/copyable example briefs and eleven FAQ. Reuse matching
LandingHeader/Footer, HomeSearch, LandingDialog, MediaImage/Video, FAQ and keyboard
navigation helper. Scoped CSS; no new dependencies or backend surface.

## Approved scope / design decisions

The user approved rebuilding the saved 24 templates in English, using matching
shared elements only. This is the next template in that existing design. MCP is
still planned in the actual project: the landing must preserve that truth.
No endpoint, install command, compatibility version, paid plan entitlement,
automated workflow, third-party directory listing or OAuth is invented.

Use an unavailable connection-details field and an active status/guide action
in the reference's setup-card geometry. Client tabs switch real local preparation
steps and labels, not executable fake commands. A compact Planned badge and
honest copy explain access. Feature CTA returns to the selected client card.
Examples are editorial briefs, copy explicitly; no auto-submit, provider call,
clipboard persistence, token input, external message or API request.

Sources: `references/magnific-html-js/17-mcp.html`, saved desktop/default,
eight client states, FAQ states and mobile capture. Local reference media with
provenance/actual descriptors/hash/pixel equality. Hero/capability/feature imagery
is reference artwork; video and interface stills are explicitly attributed to
Magnific and not shown as a recording of a working Epifera integration.
Replace with owned/licensed final visuals before indexed launch.

## Task 1 — data / route / shell

- [x] Add failing tests for exact own-shell, draft/noindex/empty RU, truthful
  planned record, route structure, tabs/keyboard, guide/focus, clipboard failures,
  prompt selection and FAQ.
- [x] Package existing cached images/WebM with provenance and safe asset formats.
- [x] Add Integration record + English copy and explicit publication declarations.

## Task 2 — page / interactions

- [x] Build responsive hero and eight bounded client tabs with shared helper.
- [x] Add real status and setup-guide dialogs with native inert focus return.
- [x] Match six capability entries, video reference, five alternating features,
  comparison cards and six interactive briefs; support clipboard/select fallback.
- [x] Reuse FAQ, matching header/search/footer; no public theme/language switches.
- [x] Link MCP overview from T02; preserve other planned integration actions.

## Task 3 — verification / documentation

- [x] Fresh advisory audit before long tests/build; affected/public/shared
  regression, typecheck/lint, assets/packaging/publication and final build.
- [x] HTTP EN/RU/noindex/sitemap, incoming link and served MIME/hash checks.
- [x] Native desktop/mobile/tablet/wide, all tabs, prompts, copy, dialogs/focus,
  FAQ, search/menu/video and horizontal overflow; save screenshots and reset
  viewport. Reuse localhost:7159 server and current checkout only.
- [x] Update scoped UI catalog/index, docs INDEX/state and measured evidence.

## Result and verification

T17 template completed: T01-T17 plus T23 = 18/24. This is a local planned-MCP
landing, not a deployed or functioning connector. No commit/push/deploy.

- Red shell/route tests failed first. Final 252 public/shared tests in 27 files
  passed, including the updated T02/T06 consumers. Focused 54-test run passed.
- Typecheck, final full lint, fresh audit (zero vulnerabilities), assets,
  packaging, publication (4 tests) and final production build passed.
- Final CSS uses overflow:clip and a continuous dark preview surface; sticky
  brief preview remains inside its own layout without a white lower gap.
- HTTP EN/RU 200, one English shell, empty RU, noindex/nofollow and no sitemap
  entry. Both T02/T06 incoming links and all 14 media MIME/SHA/byte checks passed.
- Native widths 390/820/1280/1600: no horizontal overflow. Eight client tabs,
  roving Home/End, mobile inner scroll, guide/status/inert/focus, six brief
  selections/copy, 11 exclusive FAQ, menu/search (Russian Nano Banana Pro query),
  both incoming navigations and feature CTA preserving Codex verified.
- Reference WebM played through native controls: 1280x720, 38.301 seconds,
  readyState4, error=null, playback advanced to16.097709; paused and closed.
- Viewport reset, /en/mcp left in deliverable tab3, final browser error log empty.

Implementation findings resolved before completion: the shared connections
section also had a T06 consumer, updated alongside T02 and tests. Unnecessary
manual useCallback on the generic opener was removed per React Compiler guidance;
only the header search handler uses stable dialog-dependent memoization. No lint
rule disabled. Alpha AVIF is preserved rather than re-encoded with altered RGB
under fully transparent pixels.

Evidence under `.cache/integration-rebuild/`: `integration-final.log`,
`focused-final.log`, `typecheck-final.log`, `lint-final.log`, `build-final.log`,
`assets.log`, `packaging.log`, `publication.log`, `audit.json`, `http-smoke.json`,
`browser-qa.json`, `desktop.png`, `mobile.png`, `mobile-video.png`, `full-page.png`.
Broader existing uncommitted work preserved; no alternate site/port/worktree,
new dependencies, backend/auth/billing/provider/private-artifact/Jobs change.
