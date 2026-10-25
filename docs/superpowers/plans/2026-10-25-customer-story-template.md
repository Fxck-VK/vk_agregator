# T20 Customer Story Template Implementation Plan

Status: completed. Local-only; no commit, push or deployment.

Goal: rebuild /en/customers/football-campaign from saved Magnific reference20.
RU empty; both explicit draft/noindex outside sitemap. One active checkout and
localhost:7159. This is a source-attributed case study, not an Epifera customer.

Architecture: typed CaseStudy (client/industry/challenge/solution/results/quote/
source/credits); dedicated CustomerStory layout because T08/T09 EditorialPage
has a different byline/outline/body contract. Reuse matching light LandingHeader,
centered Footer, HomeSearch, LandingDialog, LandingCarousel, MediaVideo and
MediaPreviewDialogTemplate. No dependencies, backend or provider calls.

Design: divided two-column hero, original source hero image and source notice,
large intro, sticky desktop/mobile-first factual sidebar, four narrative sections,
one short attributed Billy Boman quote and original editorial takeaways, four
local YouTube-cover links, three-image bounded carousel and shared lightbox,
three credit columns, closing reference media and business CTA. Existing B2B
page links to this reference study. No fabricated clients, results, performance,
legal guarantees, certifications or Epifera customer relationships. No review/
rating/CaseStudy structured data publishing before real verified content.

Source YouTube films are not cached; use explicit original YouTube links with
local public thumbnails/fallback. Existing CSP frame-src none stays intact;
no iframes/scripts or third-party network until an explicit outbound navigation.
The cached enterprise WebM may play in an explicit shared video dialog; no
foreign client badges or entitlement claims in the closing section.

## Task1 — red/data/media
- [x] Fresh dependency audit before long tests.
- [x] Failing route/EN/RU/noindex/shell/media/link/gallery/focus/no-write tests.
- [x] Package six cached images (including enterprise poster) and one WebM,
  and four public thumbnails; actual-format/SHA/dimensions/source provenance.
- [x] Typed CaseStudy with honest attribution and concise original copy.

## Task2 — presentation/interactions
- [x] Own hero, intro, sidebar,4sections, source quote/editorial takeaway blocks.
- [x]4original-video links,3gallery images/carousel/lightbox, credits, closing.
- [x] Shared search/dialog lifecycle/native inert/opener focus/media fallback.
- [x] Exact publication/own shell routing; incoming business reference link.

## Task3 — checks/docs
- [x] Focused and affected public/shared regressions, types/full lint, assets,
  packaging/publication and final production build.
- [x] HTTP EN/RU200/noindex/sitemap, safe outbound links, incoming CTA and
  media MIME/bytes/SHA. Security policy unchanged.
- [x] Native390/820/1280/1600px, gallery arrows/thumbnails/cycle/focus,
  local video playback, mobile menu/search, business links/credits and overflow.
- [x] Save viewport screenshots, reset viewport, retain deliverable tab.
- [x] Inline review, UIcatalog/index/docsINDEX/state and finalgitstatus.

Count after completion: T01–T20 plus T23 =21/24 local templates.

## Verified result

Implemented /en/customers/football-campaign locally from saved reference20.
Typed CaseStudy keeps Magnific attribution and isEpiferaCustomer=false; source
metrics unverified by Epifera, one short attributed quote, separate editorial
notes. Source films open explicitly on YouTube with local covers; CSP unchanged.
Own light case layout reuses matching shared components without forcing article
chrome. Business page has an explicit incoming reference-study link.

- Fresh npm audit0 vulnerabilities before long tests.
- Red route/shell contract; targeted scroll-panel regression also red/green.
-40focused tests, final347public/shared/account/payment/media tests39files passed.
-Types/full lint, assets/packaging/publication and final build exit0 passed.
-HTTP EN/RU200/noindex/nofollow, exact sitemap exclusion, incoming business link,
  frame-src none and all11media MIME/bytes/SHA passed.
-Native390/820/1280/1600px with no page overflow; desktop sticky facts height
  corrected to614px/545px viewport on720px window, source button now reachable.
-Mobile menu/search(Russian Nano Banana Pro), gallery arrows/thumbnails/cycle,
  carousel end-disabled, native inert/focus return and source details verified.
-Local platform video13.917s/1920x1080ready4, explicit Space play and ended=true
  observed. No autoplay; unmount on close; return focus verified.
-Incoming business link clicked; viewport reset; final story tab retained.
-In-place review: no unresolved scoped issue. No backend/private artifacts,
  authentication, billing/ledger/provider/Jobs changes, commit/push/deploy.

Evidence: ignored .cache/customer-story-rebuild/ logs,audit.json,http-smoke.json,
browser-qa.json,desktop.png,mobile.png,mobile-gallery.png,desktop-video.png and
finalgitstatus. Original public JPEG covers retained in ignored cache, lossless
WebP decoded-pixel equality verified. Existing JSDOM navigation notices did not
fail tests. Final template count T01–T20/T23:21/24 local, not published/indexed.
