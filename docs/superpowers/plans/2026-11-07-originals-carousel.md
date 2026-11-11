# Third landing originals carousel

Approved: reproduce the supplied Magnific video's centred carousel with our
three existing materials. Preserve heading, external CTA, card copy and links.
One72%-wide central card (82% on phones), visible smaller neighbours, smooth
horizontal movement of the complete card. Keep dots and no arrow controls.

The shared LandingCarousel has no controlled selected/centred/loop contract;
use an isolated third-variant component and motion hook with HomePicture and
LandingLink. No new dependency, media download, provider call or publication.

- [x] Add failing tests for selection, accessible duplicates, wrap, gestures,
  automatic movement, hover/focus/visibility pauses and reduced motion.
- [x] Implement bounded clone normalization, centred-track CSS, compact overlays
  and preserved dots. Vertical touch scrolling remains native.
- [x] Integrate HomeOriginals and remove its unused old banner styles.
- [x] Verify all eight required CSS widths plus short/breakpoint windows,
  keyboard, swipe, repeated cycles, rapid selections, reduced motion and links.
- [x] Run focused tests, TypeScript and scoped lint; update UI docs/evidence.

Local website remains localhost7159; only third-preview originals are changed.
Source photographs remain photographs; the supplied reference video is evidence
of layout/behaviour, not authorized new Epifera video content.

Evidence: `.cache/responsive-qa/2026-11-07-originals-carousel/`. Final matrix:
33/33 states (three slides at11viewports, including360/390/768/1024/1440/1920/
2560×1440/3840×2160 plus short phones and767/769breakpoint neighbors). Native
motion checks passed: intermediate transforms, both visible neighbours, loop
normalization without scale jump, rapid choices, neighbour click, mouse drag,
native vertical wheel, auto advance, hover pause, reduced motion, actual CDP
touch swipe and vertical touch scroll. Nine focused tests passed.

Initial phone audit exposed hidden side previews at86%;82%width/8px gap and0.94
neighbor scale fixed it. The gesture harness now waits for the motion state to
settle, rather than starting a drag during the previous CSS transition. Actual
gesture assertions passed afterward; no failed result is counted as passing.

## Boundary navigation correction

Owner reported a rewind when clicking the first neighbour after the last card.
The step path was correct, but go(index) animated to an absolute middle-group
index. New neighbour/dot regressions failed with position3 instead of6.
Both entry paths now select the nearest equivalent physical copy. Requests
during motion/normalization are queued; explicit Next/Previous retain direction.
No CSS, card dimensions, text, media, links or auto-advance interval changed.

- [x] Pass neighbour/dot and queued-action unit regressions plus home integration.
- [x] Verify direction for both pointer paths at all8required widths and rerun
  native motion/touch/auto checks. Evidence: `2026-11-07-originals-direction/`.
- [x] TypeScript/scoped ESLint and diff checks; local only, no deployment.

Final:12focused tests,16native direction cases and all11existing native
motion/touch checks passed. At every required viewport, last→first used physical
position6 and a decreasing track translation before the invisible reset to3.
The home integration test now waits for scene activation and finishes simulated
transitions; fake-timer tests flush successive RAF commits separately.
