# Model marquee implementation plan

Earlier follow-up, 2026-11-09: owner requested +10% model-strip speed and photo
reveal while mouse-hover motion continues. The earlier hover-pause requirement
below is superseded for this strip. Keyboard/touch/reduced-motion pauses remain.
`useModelMarquee` accepts an optional pixels/second value: ModelsMarquee uses41.8,
ToolsMarquee retains the existing default38. No card geometry/navigation changes.
Verification: three updated existing hover assertions failed before the change;
22 existing model/tool/hook tests, TypeScript and scoped ESLint passed after it.
Native Chromium verified all eight required widths at DPR1/100%: photo reveal
while moving, 41.8px/s nominal speed, preview handoff under a stationary pointer,
keyboard pause, unchanged tool-strip38px/s/hover pause and reduced motion.
Screenshot contact sheet reviewed. Evidence:
`.cache/responsive-qa/2026-11-09-model-hover-motion/`. Local only.

Latest owner follow-up, 2026-11-09: match the tools strip to the models strip.
Both now use the shared41.8px/s default and mouse hover does not pause either.
Tools retain their existing6px hover lift, link destinations, keyboard/touch
pauses, gesture cancellation and reduced-motion handling. The obsolete tools
mouse-hover pause listeners are removed; original/friend layouts are unaffected.
Tools follow-up verification: two updated assertions failed before the fix;
22 existing strip/hook tests and scoped ESLint passed. Native Chromium passed
360/390/768/1024/1440/1920/2560x1440/3840x2160 at DPR1/100%, measuring normal
and hovered motion, the existing6px lift, keyboard pause, equal model-strip
speed and reduced motion. Contact sheet reviewed. Evidence:
`.cache/responsive-qa/2026-11-09-tools-hover-motion/`. Local only.

Goal: implement the owner-approved reference interaction in the third landing:
continuous seamless motion, light name/logo cards, photo reveal and pause on
hover/focus, touch preview, no manual horizontal scrolling and no arrow controls.

Architecture: a local `ModelsMarquee` and motion hook own the continuous strip.
Reuse `HomePicture`, `ModelIcon`, current editorial copy and asset registry.
Keep shared `LandingCarousel`, original/friend variants and all 24 templates intact.
Stack: existing React/TypeScript/CSS and browser scroll/animation APIs; no dependencies.

Constraints: active checkout and localhost:7159 only; Russian publication policy
and private DEV/noindex boundary unchanged. No commit/push/deploy or paid calls.
Respect reduced motion, hidden tabs and off-screen state. Clone groups must not
duplicate keyboard/screen-reader navigation. Keep existing card destinations.

- [x] Add failing interaction tests to the existing model-section consumer.
- [x] Implement continuous wrapping, hover/focus pause and photo reveal.
- [x] Add touch preview, gesture cancellation, outside dismissal and accessible clone handling.
- [x] Run focused tests, types, lint; verify all eight required CSS widths,
  continuous movement, wrapping, hover resume, touch/keyboard and reduced motion.
- [x] Update the landing/UI documentation and record ignored browser evidence.

Files: `features/landing-variants/third/{HomeExplore,ModelsMarquee,useModelMarquee}`,
local CSS/tests and existing landing documentation. Browser evidence goes under
`.cache/responsive-qa/2026-11-07-model-marquee/`.

Owner follow-ups: preserve original logo colors (removed whitening filter) and
increase both card dimensions by exactly 20%, retaining .8 aspect ratio and .5rem
radii. Final widths use clamp(16.2rem,21.6vw,22.2rem). Photo source sizes updated.

Verification: four initial interaction regressions failed against the old
carousel. Final 23 affected tests, TypeScript, scoped ESLint, asset/Russian-copy
checks passed. Browser verified 360/390/768/1024/1440/1920/2560×1440/3840×2160,
DPR1 and zoom100%, default/hover states visually reviewed. Actual +20% geometry,
constant motion, zero drift while paused, resumed movement, wrap max frame step
1.5px, eight accessible links and no arrows passed. Reduced-motion and offscreen
samples stayed still. Real touch first tap/outside dismissal, native 224px swipe
and second-tap navigation to /ru/app/models passed. Veo on-photo logo retained
1018 colored source pixels and filter:none. Read-only review found no blockers.
No commit, push, deploy, new dependency, backend or payment/provider operation.

Owner video follow-up: remove stale hover when the page scrolls under a stationary
mouse. Native headless Chromium delivered boundary events correctly in the initial
reproduction; explicitly suppressing those events reproduced the recorded stale
preview/pause. Three new unit cases failed before implementation. ModelsMarquee
now remembers mouse coordinates and reconciles the topmost hit on captured scroll
and resize, batched through one animation frame; touch and keyboard focus remain
independent. CSS, card dimensions, motion speed, shared consumers and destinations
are unchanged.

Follow-up verification: 26 affected tests passed; TypeScript and scoped ESLint
passed. Browser checks passed all eight required widths (DPR1, zoom100%) with
boundary events deliberately suppressed: native wheel and programmatic away/back
cycles dismissed/reopened the photo without mouse motion, resumed/stopped the
strip and retained keyboard focus. Real touch preview, outside dismissal and
219px native swipe passed. All sixteen away/returned screenshots visually
reviewed. Evidence: .cache/responsive-qa/2026-11-07-model-hover-scroll/.
No publication authorized or performed.

Owner correction: manual horizontal swiping is not part of the desired interaction.
The latest implementation supersedes the earlier native-swipe behavior recorded
above. The viewport clips user scrolling while retaining programmatic scrollLeft
animation; pan-y/pinch-zoom preserves page scrolling and zoom. Removed manual-pan
and inertia bookkeeping. Pointer drags/cancellation never activate a card; taps,
hover, keyboard focus, original colors and enlarged dimensions are preserved.

Verification: the pre-fix browser regression moved 500px on horizontal wheel.
Two additional unit cases failed for mouse-drag activation/canceled-touch pause.
Final 28 affected tests, TypeScript and scoped ESLint passed. All eight browser
widths passed bidirectional horizontal wheel (0px), mouse drag (0px), automatic
motion and stationary-pointer hover reentry. Native touch swipes left/right at
390/768px stayed at 0px during contact; auto-motion resumed after release. Native
vertical panning scrolled the page125px, and second-tap navigation still opened
/ru/app/models. All sixteen screenshots visually reviewed; card geometry matches
the previous eight-width evidence. Evidence: .cache/responsive-qa/2026-11-07-model-no-manual-scroll/.
No commit, push or deploy.

Owner follow-up after the tool-strip implementation: remove all five category
tabs, retain the full continuous strip. HomeTools now passes all existing items
directly to ToolsMarquee; removed unused filter state/styles and tab-panel ARIA.
The marquee engine, card dimensions and hover lift are unchanged. Current
browser evidence: .cache/responsive-qa/2026-11-07-tools-marquee/no-tabs/.

Vertical-wheel follow-up: the previous pure-vertical check missed mixed trackpad
deltas. Native browser reproduction showed deltaX2/deltaY80 scrolling the page0px
with overscroll-behavior-inline:contain, versus80px with auto; the strip stayed
fixed in both cases. Replaced that single CSS property with overscroll-behavior:auto.
No wheel preventDefault/forwarding or geometry/JS changes. Regression failed before
the fix. All eight widths passed ten vertical/mixed and two horizontal cases each
(96 native wheel checks): page followed deltaY, strip movement0px while hovered.
Hover dismissal and automatic resume passed, nine component tests passed; all
eight screenshots visually reviewed and geometry matched prior evidence. Evidence:
.cache/responsive-qa/2026-11-07-model-wheel-chain/. No commit, push or deploy.

## Tool strip follow-up

Owner-approved scope: use the same seamless marquee for HomeTools; remove its
arrow controls, pause/resume on hover and lift the hovered card slightly. Keep
the approved246×307px desktop/234×307px phone geometry, all five category filters,
copy, icons and links. The agreed lift is6px/220ms; do not stretch cards to fill
the section. Preserve the100svh section and its spacing.

Reuse the existing motion hook in a local ToolsMarquee rather than extending
shared LandingCarousel. Add a content key to remeasure short filtered groups
before paint; retain enough copies even with reduced motion. Stable hover slots
prevent bottom-edge flicker. Preserve keyboard/focus, native vertical scrolling,
touch gesture cancellation, hidden/offscreen/reduced-motion behavior and a
single accessible set of links. Keep original/friend/T01 unchanged.

Checks: failing component and measurement tests, focused regressions/types/lint,
native browser at360/390/768/1024/1440/1920/2560×1440/3840×2160 and short/breakpoint
cases. Record geometry, five filters, smooth lift, pause/resume, cycle wrap,
vertical/mixed wheel, disabled manual horizontal dragging, touch, keyboard and
reduced-motion evidence in .cache/responsive-qa/2026-11-07-tools-marquee/.
No new dependency, backend operation or publication.

Tool follow-up verified locally:27 focused tests, TypeScript and scoped ESLint
passed. Native Chromium passed all eight required widths plus390×667,767×667,
769×667 and1440×650:60 filter/geometry states, unchanged card sizes,6px lift and
stable hover at the vacated lower edge, zero paused drift, resumed motion,
seamless wrap, keyboard traversal,96 wheel/drag cases and the model-hook consumer.
Eight-width reduced-motion checks passed40 filter states with zero movement and
enough copies. Native touch at390/768px preserved vertical panning125px, blocked
horizontal swiping, canceled drag activation and followed the link on one tap.
Frame samples confirmed a monotonic0–6px transition over220ms; offscreen movement
was0px. Screenshots at all eight widths visually reviewed. Protected41 original/
friend/catalog files match their saved hashes. Evidence: summary.json and
extra.json under .cache/responsive-qa/2026-11-07-tools-marquee/.
No commit, push or deploy.

Latest owner sizing follow-up: enlarge tool-card width and height by another20%:
246×307 →295.2×368.4px desktop;234×307 →280.8×368.4px below768px. Only the
two local CSS sizing rules change; type, icons, padding, radii, full tool set and
marquee/hover behavior remain. Six focused tests passed; native Chromium verified
all eight required widths plus360/390/767/769×667, actual1.2 sizing, screen fit,
continuous motion,6px hover lift/pause and native mixed-wheel page scrolling.
Evidence: .cache/responsive-qa/2026-11-07-tools-marquee/scale-1.2/.
