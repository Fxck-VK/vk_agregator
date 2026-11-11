# Third hero: text effects controlled from the development flask

Owner correction: only text above the existing photograph changes. No animated
photos, result cards or selector inside the hero. Use the existing bottom-right
development panel for choosing fade, word reveal and stationary phrase highlight.

1. Replace the rejected image previews with third/HeroTextAnimation, using the
   existing copy.hero.capabilities. Remove image-preview files and dictionary
   labels. Restore the hero's preceding geometry and background presentation.
2. Three text effects: a phrase dissolves/softens in one place; words reveal in
   sequence; a fixed list changes its highlighted phrase without scrolling.
   Retain the small-screen hiding rule through1050px from the preceding layout.
3. Reuse the bounded scene clock for6 phrases/2000ms, reduced motion, offscreen
   and document visibility. Switching effects resets the phrase; no media/APIs.
4. Add generic ephemeral pageChoiceStore metadata to LocalDevelopmentTools.
   The page registers only in trusted DEV and unregisters on unmount; ownership
   guards prevent stale cleanup deleting newer controls. SSR snapshot stays null.
   Reuse ModeSwitchPanel. With a page control, release inventory is collapsed
   into details; other pages keep their previous panel presentation.
5. Tests before implementation: owner cleanup and SSR store isolation; text-only
   rendering, all3 choices and reset; existing clock, homepage and release-panel
   regressions. Then types/scoped lint and RU/public-copy checks.
6. Browser checks:360/390/768/1024/1440/1920/2560x1440/3840x2160 and1050/1051.
   Verify controls through the real flask, keyboard, panel bounds, unchanged
   hero image/geometry, no media in text region, no overflow, animation timing,
   reduced motion, offscreen pause and unchanged original page tools.
   Screenshots and geometry: .cache/responsive-qa/2026-11-07-hero-text/.
7. Update UI/variant docs, verify41 protected source hashes and diff. No commit,
   push, deploy, billing/auth/provider or publication changes.

## Verification

-12 targeted tests passed for text controls, page-choice ownership, clock,
  homepage and release inventory. A new2-second regression was observed failing
  at the old delay and then both text-component tests passed with2000ms.
-10 browser widths /30 states passed, including all8 mandatory widths and1050/
  1051px. Below1051px the preceding hidden-text behaviour is retained; the flask
  still fits. Desktop text, keyboard settings, single unchanged photo and
  original-page controls were checked. Reduced motion and offscreen pause passed.
- Types/scoped lint and3858-record RU/public-copy gates passed.41 protected
  original/friend/catalogue hashes unchanged. No deployment or provider calls.
- Final browser timing confirms approximately2000ms between phrase changes in
  all3 effects; fade CSS shares the same duration through a custom property.
  Evidence: `.cache/responsive-qa/2026-11-07-hero-text/timing.json`.
