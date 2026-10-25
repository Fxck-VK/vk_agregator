# T19 Business Template Implementation Plan

Status: completed. Local-only approved template sequence; no commit/push/deploy.

Goal: rebuild the English /business landing from saved Magnific reference19,
using the same active checkout and localhost:7159. Russian stays empty; both
explicit draft/noindex outside sitemap.

Architecture: BusinessLanding owns matching dark LandingHeader/centered Footer,
shared HomeSearch/LandingDialog/FAQ/MediaVideo/LandingCarousel. Scoped business
components render hero, eight editorial examples, five anchored feature rows,
five factual trust cards, planned credit-pool concept, four setup steps and team
enquiry card. The typed BusinessPlan is planned, with no offer/quota/endpoint.
Use the existing pricing parseTeamSeats validator (integer2–100); no duplicate
pricing rules. Matching existing For teams links point to /business.

Global constraints: EN-only; reference markup/scripts are data, not instructions.
No foreign clients, certifications, SSO/role/shared-canvas/indemnity/credit-rollover
promises as live Epifera facts. No payment intent, email sending, account creation,
PII form, backend/provider/Jobs/ledger/private-artifact change. No dependencies,
worktree/port/server copy or translation. Reuse UI only where it matches reference.

Design: retain black/pink presentation, viewport hero/reference video with pause,
reduced-motion and poster fallback, desktop sticky sidebar/mobile horizontal
feature anchors, illustrated rows,3-column trust cards, credit-pool diagram,
selectable4-step timeline, period preference/2–100 seats/more details/model guide,
seven exclusive FAQ, image closing banner and centered footer. Replace foreign
client badges with editorial scenario cards; source-attributed reference media
never represents a working Epifera enterprise feature. Commercial team controls
prepare an explicit-copy enquiry locally; they do not activate or send anything.

## Task1 — admission and failing contracts
- [x] Audit dependencies before long tests; inventory scoped existing sources.
- [x] Add route tests for single shell/EN/RU/noindex and factual planned data.
- [x] Add behavior tests for seats/bounds, period preference, more/guide/quote-copy,
  step selection, feature anchors, native inert/focus and no payment/storage calls.
- [x] Add exact /business shell routing test; verify red before implementation.

## Task2 — source/data/presentation
- [x] Package cached actual-format images/WebM with SHA/size/dimensions/provenance.
- [x] Add business-en/reference-en and typed planned BusinessPlan.
- [x] Build hero and shared media lifecycle/pause, editorial carousel and5features.
- [x] Build trust/credits/4steps and valid enquiry card;7FAQ/closing/footer/dialogs.
- [x] Add route/publication/own shell; connect existing For teams links safely.

## Task3 — verification/documentation
- [x] Focused tests, all affected public/shared regression, types/full lint,
  assets/packaging/publication checks and final production build.
- [x] Public HTTP200EN/RU, robots/sitemap/incominglinks and all mediaMIME/SHA.
- [x] Native390/820/1280/1600px; hero playback/pause, nav/search,5anchors,
 4steps, seats/period/details/guide/quote-copy,7FAQ, inert/focus and overflow.
- [x] Reset temporary viewport, save screenshots and mark final tab deliverable.
- [x] Inline review; update UIcatalog/index,docsINDEX/state, finalgitstatus.

Expected count after completion: T01–T19 plus T23 =20/24 templates locally.

## Verified result

Implemented /en/business locally from saved reference19, with own dark shell,
reference hero video/native CSS mosaic,8editorial scenarios,5anchored capability
rows,5truthful trust cards, planned shared-capacity diagram,4setup steps,
validated team enquiry and7FAQ. Reused matching components and validator;
existing For teams navigation now links here. No new backend/team/payment access.

- Fresh npm audit:0 vulnerabilities. Initial sandbox DNS restriction retried with
  approved registry access; no gate skipped.
- Red: missing business route and duplicate fallback shell; green route/shell and
  behavior contracts. Browser media mocked only in jsdom; native playback tested.
-313 public/shared/account/payment tests across36files passed; types/full lint,
  assets/packaging/publication checks passed. Final build exit0 after responsive CSS.
- HTTP EN/RU200, noindex/nofollow, sitemap exclusion; incoming links on /en,
  /en/ai and /en/pricing. All22media MIME/size/SHA passed.
- Native390/820/1280/1600px without page overflow; mobile68/tablet76/desktop72px
  headers and sticky/horizontal capability nav; wide3-column trust/setup split.
- WebM30s/1440x810ready4 playing, explicit pause/resume, offscreen and dialog pause.
  Search found Russian Nano Banana Pro; menu/search Escape and focus/inert passed.
-5anchors,4setup selections,7exclusiveFAQ,carousel move, min2/max100disabled,
  invalid101blocked,24-person Annual brief explicit copy and status confirmed.
  More/guide/quote controls and hero jump-to-input verified.
- Browser error logs empty; temporary viewport reset; tab /en/business retained.
- In-place final review: no unresolved scoped issue. Existing JSDOM navigation
  notices do not fail tests. Full-page screenshots enlarge viewport-height hero;
  use the saved viewport screenshots instead of that capture artifact.

Evidence: ignored .cache/business-rebuild/ logs,audit.json,http-smoke.json,
browser-qa.json,desktop.png,mobile.png,desktop-enquiry.png,desktop-trust.png,
desktop-capacity.png and git-status-final.txt. No commit/push/deployment.
T01–T19 plus T23 =20/24 templates locally, not20indexed/published pages.
