# T21 Company / Contacts Template Implementation Plan

Status: completed locally. Local only, no commit/push/deploy.

Goal: /en/company/about-us from saved Magnific reference21. RU empty; both
explicit draft/noindex outside sitemap. Same active checkout/localhost:7159.

Architecture: CompanyProfile with real brand/support contact; unknown founding
/team/address stay null/empty. AboutLanding owns matching overlay header,
announcement/hero, statistics, focus directory,8editorial scenario cards,
6account/terms/team notes,7-step timeline,4values, product visual/chips,
14existing model cards with hover/focus video previews,10FAQ, contribution
callout and3contact cards. Reuse LandingHeader/Footer/Dialog/Carousel/FAQ,
HomeSearch, MediaVideo, ModelIcon and existing docs/model/link data where matching.

Truthful adaptation: no foreign staff/LinkedIn/client marks, testimonials,
ratings, subscriber/headcount/ARR/founding/acquisition, certification/SSO or
jobs vacancy claims. Keep reference layout but use functional focus cards,
original editorial scenarios and a process timeline without invented dates.
Support and press/general inquiries all use real support@epifera.com; help
links to current docs. No message automatically sent, PII form or new mailbox.
Current available routes vs planned Business/MCP clearly distinguished.

## Task1 — admission and red contracts
- [x] Fresh dependency audit before long tests.
- [x] Failing EN/RU/noindex/shell/data/truthfulness, contact/mailto,10FAQ,
  model preview/focus/reduced-motion, source dialog/native inert/no-writes tests.
- [x] Package10source images and11small cached model clips with actual formats,
  SHA/bytes/dimensions and reference attribution; reuse3existing static posters.

## Task2 — layout and behavior
- [x] Add typed company-en/reference-en data, computed factual counts and EN route.
- [x] Build all reference section layouts, own scoped colors/sizes/responsive CSS.
- [x] Add14current model links, hover/focus videos with pause/reduced-motion,
  hidden/offscreen/dialog cleanup; shared bounded auto carousel.
- [x] Use actual3contact destinations; hero/contact links andsource/feedback callouts.
- [x] Exact own shell/publication; connect About in current nav/footer data.

## Task3 — checks/docs
- [x] Focused + public/shared regression, types/full lint, assets/packaging/
  publication checks and final build.
- [x] HTTP200EN/RU, noindex/sitemap/incoming links, allmedia MIME/SHA,
  no third-party requests/scripts/iframes or send action, CSP unchanged.
- [x] Native390/820/1280/1600px, mobile nav/search, preview play/pause/focus,
  model carousel,10exclusiveFAQ, contact destinations, source details/focus,
  allsections and overflow. Reset viewport, retain screenshot/tab.
- [x] Inline review; update UIcatalog/index/docsINDEX/state and finalgitstatus.

Expected final count: T01–T21 plus T23 =22/24 local templates.

Verification: 361 tests in 40 files passed; typecheck, full lint, asset validation/tests, packaging, publication tests and final production build passed. Fresh npm audit: 0 vulnerabilities. HTTP: EN/RU200 and noindex, sitemap exclusion, three incoming pages, CSP unchanged, all21 media MIME/bytes/SHA passed. Native390/820/1280/1600px without page overflow; mobile menu/search, ten exclusive FAQ, source dialog/focus, contact anchor, model playback/focus cleanup and carousel arrows verified. Browser errors: none. Viewport reset1280x720. No commit/push/deploy.
