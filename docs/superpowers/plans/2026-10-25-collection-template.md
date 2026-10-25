# T13 — example / prompt collection

Status: completed locally. No commit, push or deployment authorized.

## Approved scope

Rebuild the saved English Magnific reference 13 as an Epifera collection at
`/en/inspiration/cinematic-scenes`. Keep its Russian counterpart empty and
both locale declarations draft/noindex, outside sitemap. This is one reusable
collection template, not a stock catalog, asset detail, or backend collection CMS.

## Reference and reuse decisions

- Source: `references/magnific-html-js/13-collection.html` and saved desktop/mobile
  captures under `.cache/magnific-ui-capture/13-collection/`.
- The real source is `/collection?id=15879768`, titled Image Stock Styles Cinematic.
  The archive contains 48 images despite a remote count of 64. Display the real
  local member count. The desktop capture is scrolled; inspect intro markup and
  mobile capture too. The mobile capture includes an accidentally open tools
  drawer; reproduce the drawer interaction, not a permanently obscured page.
- Reuse BrandLogo, HelpIcon, MediaImage, MediaPreviewDialogTemplate, shared modal
  primitives and LandingFooter only for matching parts. The HelpCenterFrame has
  a reading sidebar absent here; the landing header differs. Use collection-owned
  rail/search/topbar/grid styles. MasonryGrid uses CSS columns, whereas this
  reference has ordered rows with equal-height cards per row: use a row grid.
- Package only already cached reference images; retain source provenance and
  decoded-pixel verification. No foreign subscriber counts, premium entitlements,
  logos, trackers, credentials, runtime scripts or analytics.
- Suggested prompts are new editorial suggestions, not claimed original prompts
  or Epifera-generated results. No provider/model capability claims.

## Behavior

- Ordered 48-member collection, intro, local media, responsive 1/2/3/4-column grid.
- Search over the full collection; clear/no-match states; mobile search dialog.
- Preview with previous/next, thumbnails, keyboard, suggested prompt copying and
  in-memory prompt handoff to the existing image workspace. No generation submit.
- Save/unsave the public collection slug on this device only, with accurate status
  and blocked-storage error handling. Never persist prompt or personal data.
- Share copies the canonical current collection URL, with clipboard error fallback.
- Responsive navigation/tools drawer, safe actual Epifera destinations, footer.
- Modal background inert, keyboard focus trap/return, Escape/backdrop closing.

## Execution / verification

1. Add focused failing tests for route publication, shell isolation, collection
   data/ordering, search, previews, clipboard/storage failures and focus return.
2. Implement typed collection data, scoped media, shell/cards/preview/route.
3. Run tests, npm audit, types, lint, assets, packaging, publication and build.
4. Verify actual localhost HTTP and desktop/mobile browser interactions and
   overflow, save screenshot evidence; update UI catalog/index and state.

No backend/auth/billing/job, public indexing or deployment changes. Reference
artwork still needs rights-cleared replacement before a public indexed launch.

## Verified result

- Implemented 48 ordered reference cards and their suggested prompts, search with
  empty/clear states, real Use popovers, shared preview navigation/keyboard, prompt
  copy/failure and memory-only handoff, save/unsave on this device, share fallback.
- Browser confirmed the prompt arrives at /en/app/image with no URL query and no
  automatic submission. Save survives hydration/reload and was reverted after QA.
- Added an incoming link from the image-generator related-tools section.
- 180 public/shared regression tests passed; final focused collection suite: 15
  tests. npm audit: zero advisories. Types and full lint passed; final targeted lint
  also passed. Asset, packaging and publication-policy checks passed; final
  production build succeeded. JSDOM's existing navigation-not-implemented notices
  are covered by actual browser navigation, not treated as native navigation proof.
- HTTP: EN/RU 200, one English shell/48 members, empty Russian body, noindex on
  both routes, excluded from sitemap; all 48 served asset hashes verified.
- Actual browser widths 390/820/1280/1600 verified 1/2/3/4 columns and no horizontal
  overflow. Mobile search/tools/preview, keyboard return, clipboard and handoff
  checked. Temporary viewport override reset; collection tab marked deliverable.
- Evidence: .cache/collection-rebuild/{desktop.png,mobile.png,mobile-preview.png,
  http-smoke.json,integration-final.log,last-focused.log,build-final.log,audit.json}.
- No backend/auth/billing/job changes. Other existing uncommitted work preserved.
  T01-T13 and T23 layouts are now implemented locally: 14 of 24.
