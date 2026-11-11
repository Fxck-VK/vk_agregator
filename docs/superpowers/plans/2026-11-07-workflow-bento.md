# Third landing: workflow composition

Approved scope: the owner accepted the video comparison and requested implementation.
Only `/ru/landing-variants/third#workflow` changes; original/friend/T01 remain intact.

1. Reproduce the current mismatch in a browser before editing: four equally wide
   cards in two columns instead of a tall left card and a wide right card above
   two smaller cards. Use the owner's `2026-11-07 03-37-51.mp4` frames as evidence.
2. Extract `HomeWorkflow` into `third/HomeWorkflow.tsx` and scoped CSS. Reuse
   `LandingSection`, `LandingLink`, `LandingToolIcon`, locale-aware links and
   existing local photos/UI screenshots. Preserve all four copy objects and URLs.
3. Desktop: three equal columns, left card spans both rows, upper right card
   spans two columns. Cap section width at1280px including existing padding;
   keep text/media side by side in the wide card. Lower cards combine their
   text and illustrations on burgundy/teal surfaces. No giant photo footers.
4. Illustrations: catalogue thumbnails, conversation/result composition,
   existing files screenshot, starting-point thumbnails. Decorative illustrations
   are aria-hidden and introduce no nested actions, fake features or private data.
   Subtle hover/focus transitions respect reduced motion. Touch scroll is native.
5. Responsive: preserve the asymmetric desktop composition at1024px and above;
   tablet shows the catalogue and wide card on separate full rows, then the two
   smaller cards together; phone
   stacks cards in reading order. No fixed text clipping or horizontal overflow.
6. Browser QA:360/390/768/1024/1440/1920/2560x1440/3840x2160, plus breakpoint
   neighbours. Assert relative card geometry, contained copy, horizontal overflow,
   image decoding, all four links, hover/focus, reduced motion and vertical scroll.
   Inspect screenshots, save ignored evidence, run affected tests/lint/types.
7. Update landing-variant/UI routing docs; verify41 original/friend source hashes
   and `git diff --check`. No commit, push, deploy or API/billing/auth changes.

## Local verification

- Before:1440px browser measured two676px-wide cards on each row; reference
  geometry assertion failed. After: one tall left card and the right pair of
  equal-height rows; main composition directly inspected against the supplied video.
-12 browser viewports passed:360x844,390x844,768x1024,1024x768,1440x900,
  1920x1080,2560x1440,3840x2160,767x900,769x900,1023x900,1025x900.
  Evidence: `.cache/responsive-qa/2026-11-07-workflow-bento/`.
- Links, decoded images, contained text, hover/focus, native vertical scroll,
  horizontal overflow and reduced motion checked.1023px hover timing was
  retested with observed completion rather than a fixed delay; diagnostic3/3.
-31 affected component/preview tests passed; the3 whole-page tests were rerun
  after removing the obsolete photo mapping. Types/scoped lint and Russian-only
  policy/public-copy gates passed;3858 translation records remain valid.
-41 protected original/friend/catalogue hashes unchanged. No release/full build,
  private account reads, API mutations, provider calls or new media downloads.
