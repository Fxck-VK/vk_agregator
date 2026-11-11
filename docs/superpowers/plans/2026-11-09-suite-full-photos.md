# Full photographs in the creative suite

Status: complete locally; no commit, push or deployment.

**Goal:** Show the complete suite/platform photographs at their native proportions, centred to the right of the existing text, without another image resize/recompression pass.

**Architecture:** Keep the current section height, tabs, active state and category transitions. A bounded media slot centres a frame using the selected photo's verified width/height ratio. Original local WebP delivery is opt-in on `HomePicture` for this block only; other uses retain their current image policy.

**Constraints:** One existing checkout/server on localhost:7159. Russian-only routes. No asset generation, source-photo modification, model/billing changes, commit, push or deployment. Preserve original/friend comparison layouts.

## Task: Fit source-proportioned frames in the existing panel

Files:
- `web/platform/src/features/landing-variants/third/photos.ts`: verified dimensions of the five suite photos and five existing platform photos.
- `third/HomeMedia.tsx`: optional original-file delivery, default unchanged.
- `third/HomeSuite.tsx` and `third/HomeSuite.module.css`: centred frames constrained by both slot dimensions; natural-ratio mobile media row.
- Existing UI index/catalog and landing-variants documentation: full-photo behavior and delivery scope.

Steps:
- [x] Record a native-browser failing baseline: chosen image source, actual frame/source aspect ratio, clipping and panel bounds.
- [x] Add per-photo dimensions and size-contained frames. Keep the link over the active photo, existing rounded corners and keyboard/tab semantics. Portrait/square images must fit fully too.
- [x] Opt these images into original-file delivery with `unoptimized`; retain the current default for every other `HomePicture` consumer.
- [x] Run existing HomeSuite/PublicHome tests, TypeScript and scoped ESLint.
- [x] Verify all three tabs and wide-screen categories at 360/390/768/1024/1440/1920/2560x1440/3840x2160, plus dense-screen and breakpoint cases. Save screenshots and actual geometry in ignored `.cache/responsive-qa/2026-11-09-suite-full-photos`.
- [x] Review screenshots, update the relevant UI documentation and record final checks. Leave all work local.

Verification must assert the delivered source is the original local file, the visible frame ratio matches source metadata, the complete frame fits inside the media slot and tab changes retain usable text/actions. No new mirrored CSS/unit tests; the native geometry check covers the reported defect.

## Results

- Before: all seven 1920x1080 states failed the direct-source/full-frame check.
- After: 60 native Chromium states passed, including all eight mandated widths, DPR2/DPR3 and 1050/1051px. Each downloaded photo's SHA-256 equals the existing source asset. No document/panel overflow; all native aspect ratios match; card height stays stable across the three tabs.
- Four additional portrait-preserving resize/low-height cases and the third comparison URL passed. Mobile follows the complete photo's natural height and may scroll vertically as before.
- Reviewed the narrow, wide, square/portrait and density/breakpoint screenshot sheets.
- Nine existing HomeSuite/PublicHome tests, TypeScript and scoped ESLint passed. No new unit tests, dependencies, global image configuration or source image edits.
- Model input capability restrictions discussed separately with the owner are outside this photo-layout change.
