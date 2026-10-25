# T18 Pricing Template Implementation Plan

Status: completed. Local only; no commit, push or deployment.

Goal: rebuild `/en/pricing` from saved Magnific reference 18; empty Russian
counterpart, both explicit draft/noindex outside sitemap.

Architecture: typed Offer preview, shared extracted subscription-preview amounts
from the existing account UI, pure monthly/yearly budget calculation, individual
and team layouts, scoped comparison table, guide/plan/quote dialogs and FAQ.
Reuse the matching light editorial LandingHeader with scoped static presentation,
centered Footer, HomeSearch, LandingDialog, PopoverPanel, ScrollArea, CreditAmount,
FAQ, BrandLogo/Name and shared keyboard helper. No new dependencies/backend/API.

## Pricing facts and limits

The current SubscriptionPlansDialog is a UI mockup (unwired purchase buttons),
not the backend's authenticated payment-products catalog. Extract only its base
amounts and stars into a shared explicitly named preview catalog, preserving the
old dialog's rendering. Public plan labels and a concise note identify previews.
Do not copy foreign prices, discounts, unlimited quotas, company testimonials,
license rights, per-seat offers or yearly billing claims.

Start+ / Pro and an Ultima-or-Elite capacity card follow the reference's three
individual cards. Lite appears as a separate weekly trial concept in the units
guide. Yearly view is an estimate = twelve monthly preview amounts; no invented
annual discount or up-front annual grant. Team seats 2–100 prepare a local quote
brief; team prices/collaboration/access remain unverified. No personal data form,
mail sending, payment intent, auto-renewal or checkout is triggered here.

Offer fields: offerType=subscription-preview, planRefs, billingUnits=stars,
termsRef, priceSourceRef=shared-ui-preview. No Offer structured-data publication.
Final payment amounts/product/credits remain exclusively server-owned. Existing
legal links and workspace top-up flow are preserved.

## Task 1 — route / data / tests

- [x] Add failing own-shell, route/noindex/empty RU and pure budget/seat tests.
- [x] Test period/audience/tier switches, comparison selection, dialogs/focus,
  quote validation/copy, model information and no automatic billing requests.
- [x] Extract preview base values and preserve existing subscription dialog.
- [x] Add explicit publication declarations and EN data/model.

## Task 2 — reference layout and behavior

- [x] Individual monthly/yearly view, three cards, capacity selection, plan details.
- [x] Team view with two enquiry cards and validated seat count / quote brief.
- [x] Own branded banner, comparison in horizontal ScrollArea, model/units details,
  factual tool links and three editorial workflow examples instead of testimonials.
- [x] Ten FAQ and support callout; matching header/search/footer and proper
  native inert/focus/Escape. No theme/language switch outside account.
- [x] Package only needed cached reference background with actual-format/hash
  verification; other visuals use existing brand/icons/reference assets.

## Task 3 — verification / docs

- [x] Fresh npm audit before long tests; focused/public/shared/account regression,
  typecheck/lint, assets/packaging/publication and final build.
- [x] HTTP EN/RU, noindex/sitemap, existing pricing links, media MIME/SHA checks.
- [x] Native mobile/tablet/desktop/wide, all switches, quote/guide, comparison,
  search/menu/FAQ/focus, overflow and screenshots; reset viewport afterwards.
- [x] Update UI catalog/index, docs INDEX/state and results. Keep existing work.

## Verified result

Implemented /en/pricing locally from reference18; RU empty, both draft/noindex,
excluded from sitemap. Shared base UI subscription-preview catalog also feeds
existing account dialog, with no activation/backend/payment semantics changed.
Main cards, capacity, synchronized budget switches, teams/seat validation,
comparison selection, guide/details/quote copy, 10 FAQ, search/menu/focus verified.
Reference2-column/max848 and wide3-column layout retained. Foreign customer
claims replaced with factual tool links and native editorial workflow cards.

- Fresh npm audit before long tests: zero vulnerabilities.
- Focused56 tests; full public/shared/account/payment regression298 tests/32files.
- Typecheck, full lint, assets, packaging, publication and final production build passed.
- Public HTTP EN/RU200; noindex/nofollow; sitemap exclusion; incoming Pricing link;
  PNG content-type,78269bytes and manifest SHA match.
- Native390/820/1280/1600px: no page overflow. Two-column desktop, three-wide,
  one-mobile; horizontal comparison scrolls with keyboard. All10FAQ exclusive.
- Monthly/yearly estimate preserves monthly stars; Elite updates comparison.
  Invalid101 seats blocked, valid20 quote copies explicitly; enterprise preview too.
  Last comparison cannot be removed, picker restores selected plans.
- Search finds Russian Nano Banana Pro; mobile menu/search/dialog Escape and
  native inert/focus return passed. Incoming T14 Pricing link clicked successfully.
- Browser error logs empty; temporary viewport reset; final tab /en/pricing retained.

Evidence is ignored under .cache/pricing-rebuild/: logs, audit.json,
http-smoke.json, browser-qa.json, desktop.png, desktop-full.png, mobile.png and
mobile-details.png. In-place review found no unresolved scoped issue.
Existing JSDOM navigation notices did not fail any tests. No commit/push/deploy.
Template count: T01–T18 plus T23 =19/24 locally implemented, not19 indexed pages.
