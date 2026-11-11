# Third landing preview

Approved scope: a separate third homepage with our T01 layout and Russian copy,
using the contributor's photographs. Preserve both originals, canonical routes,
all 24 SEO templates and existing DEV-only access. Local implementation only.

- [x] Add a regression for the third route and existing access restrictions.
- [x] Isolate the third layout/styles; project current T01 copy onto the existing
  contributor media slots without mutating either source.
- [x] Add the comparison card and update the variant documentation.
- [x] Run focused route/media/interface tests, types, lint and source/asset checks.
- [x] Inspect third/index at 360, 390, 768, 1024, 1440, 1920, 2560×1440 and
  3840×2160 CSS px; verify original source hashes and PROD denial.

Shared header, footer, search and controls retain their existing contracts.
No backend, authentication, billing, provider, publication or paid action changes.
Evidence: ignored `.cache/responsive-qa/2026-11-06-third-landing/`.

Results: route regression initially failed with NOT_FOUND, then passed. Final
route/media tests: 11/11; existing original-home tests: 6/6. TypeScript, scoped
ESLint, Russian source/catalogue policy and assets passed. Browser matrix:
16/16 cases, DPR 1 and zoom 100%; images, suite tabs, gallery and search checked.
Visual review covered all eight widths. Only the third variant changes hero
shading and closing-title line-height for readability; mobile override removal
was rechecked at actual 360/390 CSS px (`final-mobile-hero.json`).

Preservation: 41 original/friend/catalogue source files match baseline SHA-256;
PROD-view direct URL returns 404 and renders no third photographs. Review found
no blocking issues. No backend/auth/billing changes, commit, push or deployment.

Follow-up: owner requested the /app video in the third preview intro dialog.
It now reuses the workspace asset/poster registry, starts unmuted on opening
and retains native controls and existing dialog geometry. Six affected tests,
TypeScript and scoped ESLint passed; actual playback (13.933333 seconds), poster,
audio state, close action and viewport bounds passed all eight browser widths.
Evidence: `.cache/responsive-qa/2026-11-06-third-app-video/`.

Follow-up: hide complete upcoming suite tabs (MCP/API, agents, plugins), retain
original source-index photo mapping and keyboard wrapping over the three visible
tabs. Remove the announcement and its reserved header/mobile-menu offset only in
the third variant. Existing regressions failed first, then all three passed;
TypeScript and scoped lint passed. Browser evidence:
`.cache/responsive-qa/2026-11-06-third-cleanup/`.

Follow-up: removed the hero eyebrow and decorative pause control/state only in
the third variant; native overview-video controls and reduced motion remain.
Outer suite tabs and all tab buttons use 0.5rem radii. Suite tabs plus the full
card have a viewport-height budget; the section introduction remains before
that budget. The originals gallery budgets its entire heading/action/slide.
Images use the remaining flex/grid space; text is retained without zoom/scaling.

Before: 360×844 suite section measured 1163px, 1440×900 suite 1440px and gallery
1085px. Existing regressions reproduced the eyebrow/pause before removal, then
all three passed. Final TypeScript/scoped ESLint passed. Browser verification:
eight mandatory widths plus 390×667, 1024×768, 1440×900 and 1920×1080; 72 states
(three suite views and three gallery slides per viewport) passed bounds/overflow
checks, with visual review. Computed tab radii are 8px at 16px root size.
Evidence: `.cache/responsive-qa/2026-11-06-third-fit/{before,after}/`.

2026-11-07 owner follow-up: #possibilities fills the actual viewport width and
100svh, including its heading/action and four scenario cards. Remove its1536px
container cap and fixed photo aspect ratio; share caption rows with subgrid.
Keep four columns above1050px and2x2 below, with compact readable phone copy.
Preserve existing text, media, five links, hover and the outer section spacing;
reserve64px below captions so the fixed local controls do not cover them.

The browser probe failed before at all16 viewports, then passed the eight
mandatory widths plus360x667,390x667,767x900,769x900,1050x900,1051x900,1440x768
and1835x946. It verifies actual CSS viewport100%/DPR1, full width/height, four
visible cards, aligned captions, unobscured text, no overflow and five localized
keyboard-focusable links. All16 passed with no browser errors. Contact boards
and desktop/phone snapshots were visually reviewed. Three PublicHome tests,
TypeScript, scoped ESLint and diff --check passed; all41 protected source hashes
are unchanged. Evidence: .cache/responsive-qa/2026-11-07-possibilities-viewport/.
Only the third HomeExplore/Home CSS and related UI docs changed. No release.

Originals gallery spacing follow-up: add external block margins of
clamp(64px,8vw,144px) to the third #originals section. Its viewport budget,
gallery dimensions, text/media, links and three slide controls are unchanged.
The before probe reproduced insufficient spacing; all33 slide states then
passed at11 viewports (the mandatory eight plus360x667,767x900,769x900).
Gallery and section dimensions match the baseline within1px at every size.
At1440x900 the preceding content-to-heading gap grew from152.09px to201.69px;
the gallery-to-next-section gap grew from22.5px to137.69px. All41 source
preservation hashes match; diff --check passes. Evidence:
.cache/responsive-qa/2026-11-07-originals-spacing/. CSS-only, no release.

Owner clarification supersedes the external-spacing implementation above:
compact the gallery inside its unchanged section, rather than increasing the
distance between sections. Remove both external margins; internal vertical
padding becomes clamp(40px,7svh,96px), with the image receiving the remaining
height. Width, type sizes, text, links and slide controls stay unchanged.
The corrected browser acceptance failed first against the margin version.
All36 slide states now pass at12 viewports: eight mandatory widths plus360x667,
390x667,767x900,769x900. Assertions include text inside the photo frame, no
overflow, at least40px of inner air and zero outer margins. At1440x900 gallery
height is671.67px instead of752.67px, with63px inner air above/below; the section
remains900px. Evidence: .cache/responsive-qa/2026-11-07-originals-compact/.

Three follow-ups completed together: platform switch, tool-section fit and
short-screen hero. HomeSuite now consumes the existing ModeSwitchPanel with
its persistent520ms translated/resizing indicator, local white/dark appearance,
and synchronized label colors. No shared component implementation was changed.
The new persistent-indicator test failed before integration;13 suite/home/shared
switch tests then passed, along with TypeScript and scoped ESLint.

HomeTools uses a full-width100svh section but, following the owner's correction,
retains the original246x307px desktop /234x307px phone cards. Header typography,
card content, colors and icons remain unchanged. The content group is vertically
centered with spacing; cards keep the original left alignment and never stretch.
The stretched-card trial is superseded by the cards-restored evidence.
The hero removes fixed minimum heights above the viewport; compact phone/short
spacing and type preserve both actions and the logo strip inside the first screen.

Final browser checks pass18 viewports (the mandatory eight plus360x667,390x667,
767x900,769x900,1050x900,1051x900,768x667,784x667,1024x667,1440x650) at100%/DPR1.
All90 filter states preserve exact card dimensions and fit the viewport. Actual
animation samples contain intermediate monotone positions; rapid retargeting,
three resize cases, stable label contrast and reduced motion pass. Carousel
arrows, filter reset, keyboard and vertical wheel propagation pass. No page errors.
Evidence: .cache/responsive-qa/2026-11-07-screen-motion/cards-restored/ and
cards-restored-short/. All41 protected source hashes are unchanged; no release.

## Narrow-height correction

Owner screenshots show excessive blank space above/below HomeTools and between
the hero copy and partner strip in tall narrow viewports. Native reproduction
at1024×2440 measured963px above the tool heading and a2440px hero;390×2160 had
a1560px gap before the partner strip. Cause: fixed100svh plus flex growth and
vertical centering; width-only baseline checks had missed the tall aspect ratio.

Atwidth<=1050, hero and tools now use content height. HeroInner no longer grows;
the partner strip follows the actions. The tools container has auto height and
start alignment. Wider viewport sizing, card dimensions, copy, assets, links,
full-bleed strip and motion remain. No JS or shared component change.
Evidence: .cache/responsive-qa/2026-11-07-narrow-height/ with failing before data,
all eight baseline widths, tall390/768/1024/1050 cases and short/breakpoint checks.
# Scenario photo masonry, 2026-11-07

The natural-height/under-photo-caption proposal below was superseded by the
owner's next correction: fit the complete section in one screen and overlay text.

Approved scope: preserve the four full contributor photographs, captions and
destinations in the third preview; replace equal cropped rows with the existing
`MasonryGrid`. All four source files are1856×2304, so natural photo ratios are
identical; caption lengths determine small offsets between subsequent cards.

- [x] Reuse `MasonryGrid` with semantic list items and ordinary links.
- [x] Add optional intrinsic dimensions to third-only `HomePicture`; other
  consumers retain fill behavior. Scenario images use width100%/height auto.
- [x] Use four columns above1050px, two from600px and one below600px.
  Keep the complete captions and let narrow sections grow with their contents.
- [x] Verify all eight required viewport widths, breakpoint neighbors, tall
  narrow windows, intrinsic ratios, non-overlap, links, keyboard and hover.
- [x] Run focused tests/types/lint and update the scenario UI documentation.

Result: all16 browser cases passed, at CSS widths360/390/599/600/768/1024/
1050/1051/1440/1920/2560/3840, including QHD/UHD and tall narrow windows.
Four focused tests, TypeScript and scoped ESLint passed. Full build/release was
not run for this local layout change. Screenshots include the keyboard-focus
state; evidence is `.cache/responsive-qa/2026-11-07-possibilities-masonry/`.

## Final scenario correction and model entry

- [x] Keep all four full photographs within100svh using remaining-container
  height to limit grid width; retain source ratios, links and texts.
- [x] Move scenario titles/descriptions over the photo on a bottom gradient.
  Use4columns on desktop and2×2 below1050px, including phones.
- [x] Route model cards through `/app/launch?model=...`, preserving the chosen
  ID across guest login. Reuse verified-session lookup, guarded workspaces,
  existing safe-return cookie and sign-in completion. Local samples are guests.
- [x] Run failing caption/link regressions before implementation, then focused
  tests including auth, safe returns, proxy and login. Final91 tests passed;
  TypeScript/scoped ESLint passed.
- [x] Verify16 native CSS viewport cases, all8required widths and QHD/UHD,
  breakpoint neighbors, short phones and tall narrow views. All4photos/texts
  contained without clipping; real guest click reached login and retained model.
  Evidence: `.cache/responsive-qa/2026-11-07-scenario-overlay/`.

Verified-member/refresh-only branches use server unit tests; no actual account
login, deployment, purchase or generation was performed for this change.

Owner correction: scenario cards are now static articles without arrows or links;
the separate model strip retains its approved login/model navigation. The four
home tests and scoped ESLint passed. Browser checks passed at360/390/768/1024/
1440/1920/2560/3840 CSS px; clicking stays on the page, all card/photo/text bounds
match the previous layout. Evidence: `2026-11-07-static-scenarios/`.
