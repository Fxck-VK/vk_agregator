# Reference-led public homepage

**Goal:** the user selected the supplied Magnific HTML and https://www.magnific.com/ru
as the visual design for the first Epifera homepage iteration. Reproduce the
composition before introducing a more individual design.

**Approved direction:** full-width photographic hero with transparent navigation,
large left-aligned heading and animated capability list; warm off-white body,
pill tabs and dark product showcase; horizontal model cards, workflow bento,
tool filters, use-case cards, work showcase, plan/support section, shared FAQ,
photographic closing CTA and column footer. Keep Epifera brand and local actions.

**Architecture:** the existing server homepage keeps request connection and SEO.
A marketing feature owns interactive navigation/tabs/filtering and page-specific
styles. The public shell bypasses its generic chrome only for the homepage.
Reuse BrandLogo/BrandName, localized Link, PageContainer, PrimaryButton and FAQ.
Localize copy through the existing dictionaries. Preserve login, app routes,
theme/session/account/billing/provider boundaries and current unrelated changes.

Do not import or execute the saved page's scripts, trackers, analytics, cookies,
remote forms or competitor authentication. Reference photos/video are local
design assets with provenance documentation. Do not represent competitor client
counts, testimonials, compliance or unsupported products as facts about Epifera.
Responsive layout, keyboard navigation and reduced-motion support are required.

## Implementation

- [x] Record reference assets and structure; normalize actual image formats.
- [x] Add failing behavioral tests for tabs, tool filters, keyboard menus,
  localized internal navigation and the new homepage server/CSP contract.
- [x] Build marketing feature, dictionaries and homepage shell integration.
- [x] Check desktop/mobile visuals against the reference at localhost:7159.
- [x] Run focused tests, lint, typecheck, asset checks and production build.
- [x] Update the UI catalog/index and record actual verification.

Publishing is outside this request. Keep the same active checkout and preview
port; do not create another site or reset unrelated local work.

## Verification, 2026-10-22

- 68 Vitest cases passed across marketing, homepage, public shell and i18n.
- 10 Node asset/lint-contract tests passed; asset validation passed.
- TypeScript, ESLint, production build and packaging assertions passed.
- Documentation validation and git diff --check passed.
- Browser checks at 320, 390 and 768 px plus the default desktop viewport;
  no horizontal overflow. Verified product tabs, audio filtering and native FAQ.
- Fresh reload after the production build renders the local page correctly;
  one main landmark, hero heading inside main, no broken loaded images.
- Proof: .cache/magnific-homepage/desktop.png, mobile.png and full-page.png.
  Temporary viewport override reset; same server/check-out retained.

## Interaction comparison follow-up, 2026-10-22

Compared the saved HTML and live reference: desktop/mobile menus, command search,
video lightbox, independent offering tabs, expandable showcase categories,
tool filters/carousel controls, model motion, inspiration dots, FAQ and footer.
Keep the approved viewport-sized hero and Epifera destinations/copy. Unsupported
competitor products, pricing, client counts and testimonials remain omitted.

- [x] Add regression coverage for orphaned nested menus, local search/focus,
  video introduction, independent quick tabs, and direct slide selection.
- [x] Reuse modal/media/input primitives; add accessible carousel controls,
  reduced-motion/visibility/focus pause and responsive footer groups.
- [x] Validate/remux the incomplete reference video into a complete local clip.
- [x] Complete mobile/browser verification and final focused checks/build.

Follow-up verification:

- 79 Vitest cases / 15 files passed, including search, keyboard/focus restore,
  nested mobile menu close, carousel edge controls, hover/focus/reduced-motion
  pause, quick panels, slide dots, footer disclosure and localization.
- Final browser pass also caught/fixed global Escape for a hover-open disclosure
  with focus outside the header, and clipping of long hero capability labels.
  Capability lines now wrap within their rail; browser reports no clipped lines.
- 10 asset/lint-contract Node tests, asset validation, ESLint, TypeScript,
  production build and standalone packaging passed.
- Full build used `CIRCLE_NODE_TOTAL=2` for this invocation only (one Next worker).
  The default 15 workers exceeded the local machine's available memory. No
  tracked build config, test gate or deployed configuration was changed.
- Browser: local search query/target and Ctrl+K, video dialog with ready decoded
  frames, inline video switching, offscreen pause, tool carousel and reset on
  filter, keyboard tabs, slide selection, exclusive FAQ and banner anchor passed.
- Desktop and mobile menu checks passed; nested mobile navigation closes with
  its parent and restores page scroll. Mobile footer groups toggle correctly.
- No horizontal overflow; the viewport hero and both primary actions fit at
  320x568 as well as the normal desktop/mobile sizes.
- Video preview was fully decoded with FFmpeg after remuxing its complete frames;
  browser reports 21.339 seconds, readyState 4 and no media error.
- Proof: .cache/magnific-homepage/audit-search.png and audit-video.png.

Remaining visual/content differences are explicit: Epifera's real tools replace
the reference's stock/Desktop/MCP/API offerings; no competitor subscriptions or
testimonials are presented as Epifera products. Model cards still use static
reference covers and an accessible stepped rail rather than the reference's
continuous video marquee. Inspiration uses still-image slides instead of the
reference's branded films. These remain part of the later visual/content pass,
not verified implementations of those unavailable products or films.

Only marketing UI/media/dictionaries and this UI contract changed in the follow-up.
Authentication, paid jobs, provider calls, billing and unrelated work were preserved.
No commit, push or deployment was requested or performed.
