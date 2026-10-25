# T16 material detail implementation plan

Status: completed locally. No commit, push or deployment.

Goal: `/en/stock/items/dried-gypsophila/gypsophila-01`, rebuilt from saved
Magnific `16-photo-detail.html` and desktop/mobile captures. English only;
Russian empty; explicit draft/noindex outside sitemap.

- [x] Add failing route/shell and interaction tests.
- [x] Package existing cached main/related previews with actual descriptors,
  pixel equality and provenance. Author is rawpixel.com as credited by source;
  source license link is provenance, not an Epifera license grant. Original
  4667x7000/JPEG is unavailable: offer only the actual cached PNG/WebP preview.
- [x] Build the photo detail layout: matching ReferenceChrome, artwork variation
  overlay, action panel, information/rights, related rows, tags and footer.
  Generalize matching T14 actions/artwork with opt-in copy; preserve defaults.
  Reuse shared dialogs, media, PopoverPanel, carousels and public-ID bookmarks.
- [x] Add a separately identified featured photo card from the nature category;
  do not map a different landscape record to the floral photograph.
- [x] Verify interactions, focus/inert, responsive widths, no overflow, incoming
  link, noindex/sitemap and served media; run affected/shared tests/types/lint,
  asset/packaging/publication checks and final build. Update UI docs/state.

No foreign JS, tracking, author avatar, subscriber claims, premium entitlement,
original prompt or unverified commercial rights. No backend/private artifact,
provider or generation submission. Editorial variation only stages a new prompt;
source photo is not automatically attached.

## Verification / result

Local template T16 completed; T01–T16 and T23 = 17/24. Source keeps separate
photo, PSD, vector and video-still rows. Thirty-five local media files: 24 image
references, 10 video posters, 1 PNG. MP4/layered/vector originals unavailable;
no playback or source download is claimed. Reference credit and license are
source provenance only. Actual main preview 706x1060.

- Fresh npm audit: zero advisories; assets, packaging and publication tests passed.
- Public/shared regression: 236 tests/26 files. After the final related-row
  refinement, 65 affected T14/T15/T16/shell tests passed.
- Typecheck, full lint and final production build passed.
- HTTP EN/RU 200, noindex/nofollow, Russian empty, sitemap exclusion; 35 served
  files matched MIME/SHA256/byte sizes; T14 still correct; category link/result
  count preserved.
- Native 390/820/1280/1600 checks: no horizontal overflow; mobile editor kept
  inside viewport, zoom opacity1, modal Escape/focus, format menu, positive
  carousel scrolling, real 7-photo shadow filter, bookmark reload/undo, exact
  category-card navigation and staged variation in /en/app/image (no query or
  generation submit) verified. Viewport reset; tab22 left as deliverable.
- Hot-update SSR/client props temporarily diverged during editing; restarted
  only the verified active dev server on the same localhost:7159, then verified
  clean-state native navigation and final HTTP again. No alternate site/port.

Evidence: `.cache/material-detail-rebuild/{browser-qa,http-smoke}.json`,
`desktop.png`, `full-page.png`, `mobile.png`, `mobile-preview.png`,
`integration.log`, `focused-final.log`, `audit.json`, `lint-final.log`,
`build-final.log`. Existing broader uncommitted work preserved; no staging,
commit, push, deploy or backend/Jobs/private artifact/provider change.
