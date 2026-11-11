# Suite sidebar implementation plan

Goal: apply the owner-approved video layout to the third homepage only: copy,
overview action and vertical categories at left; existing photographs at right;
only the active category description expands, with a smooth media crossfade.

Architecture: HomeSuite keeps the current local tab/category state, editorial
data, asset registry, links and overview callback. A scoped HomeSuite CSS module
replaces its old bottom-caption/grid rules. Platform tabs still exclude upcoming
features. No new dependencies, timers, backend calls, content edits or release.

Responsive contract: bound the entire section to the viewport and retain .5rem
top tabs; desktop/tablet use two columns, narrow phones stack the compact text
and vertical category list before the media so neither column becomes illegible.
Respect reduced motion and keyboard tab selection. Preserve original/friend
variants, 24 templates and the already-approved model marquee.

Files: third/HomeSuite.tsx, new third/HomeSuite.module.css and HomeSuite.test.tsx;
remove the obsolete suite-only rules from third/PublicHome.module.css. Update
landing-variants.md, UI catalogue and this verification record.

- [x] Add failing component cases for vertical category semantics, a single
  exposed description/photo, mouse/keyboard switching, overview and retained tabs.
- [x] Build the scoped sidebar/crossfade layout and remove superseded CSS.
- [x] Run focused landing tests, TypeScript and scoped ESLint; browser-check all
  categories/platforms, stable stage geometry, transition and reduced motion.
- [x] Verify 360/390/768/1024/1440/1920/2560x1440/3840x2160 CSS viewports at
  zoom100%/DPR1, plus representative short heights/breakpoint edges. Save and
  visually inspect screenshots under .cache/responsive-qa/2026-11-07-suite-sidebar/.
- [x] Record results and final diff check. No commit, push or deploy.

Verification: the three new component cases failed against the old layout.
31 affected landing/preview tests passed; six directly affected suite/home cases
were rerun after final focus handling and passed. TypeScript, scoped ESLint,
asset validation, Russian-only policy and3858 public-copy records passed.

Browser final matrix: 13 viewports (the eight required widths plus360x667,
390x667,767x900,769x900 and1440x768), seven states each: all five categories and
the two other platform tabs. All91 states passed overflow/column/controls/media
geometry checks, including long links contained inside the text column. Rapid
selection, ArrowDown/End wrapping and retained category across platform changes
passed. The image stage remains fixed while shorter descriptions are selected;
the list reserves its measured longest description. Normal crossfade showed an
intermediate opacity and reached1; reduced motion switched immediately at0s.

Visual review covered each width, narrow/short layouts, category photographs,
workspace/inspiration and the corrected wrapped tablet action. Native focus and
scroll anchoring are kept from moving the viewport during panel changes.
All41 original/friend/catalogue hashes match the preservation baseline. Only the
third local preview changed; no global components, data, backend, dependency,
account/payment behavior or deployment was changed.

Owner spacing correction: preserve the former100px desktop/75px phone gap from
the hero to this section's heading, plus the former70–140px desktop/64px phone
bottom padding. These spaces now live in external margins; the inset remains
inside the viewport budget. The content/panel/photo rectangles are unchanged.
The spacing probe failed before and passed after at all eight required widths,
plus360x667 and1440x768. At1440x900 the heading gap is100px (was22.5px), and the
panel-to-next-heading gap is169.59px (was62.5px). Evidence and boundary screenshots:
.cache/responsive-qa/2026-11-07-suite-spacing/. No JavaScript or shared CSS changes.

Owner composition correction from the second recording: desktop inner padding
was only14px and categories ended290px above the column bottom at1440x900.
The new browser composition assertion failed before the correction. Sidebar
now separates the intro at top from categories anchored at bottom, with generous
responsive insets; headings use balanced wrapping and a280px measure. Other
platform titles use the same scale/top alignment. Phone flow remains compact.
The viewport height cap is54rem rather than70rem, preventing excess height on
large monitors. No interaction/JS/data changes accompanied this correction.

Owner viewport correction (2026-11-07): the earlier54rem cap covered only tabs
and panel; the title, description and section padding were extra. The new
acceptance boundary is the entire #creative-suite section. Reproduction at
1440x900 measured1217px and at1835x946 measured1228px, so both failed.
Move the height budget to the section (min(100svh,66rem)); the existing shared
section container and local tabs/panel distribute remaining height with flex.
Keep side-by-side copy/media, vertical categories, links, copy, assets and
selection logic. Use compact spacing on short screens and a smaller section
heading on phones, without clipping or an inner scroller. Verify all five
categories and both other tabs at the eight required widths, short heights and
the owner's1835x946 viewport. No shared component or source variant changes.

Fresh browser verification passed all13 viewports/91 states, including explicit
desktop inset, category-bottom and other-platform alignment assertions. At1440x900
the left/top insets are43.19/36px and category-bottom gap0px. Short screens,
breakpoint edges, switching, stage stability and reduced motion passed. Reviewed
the refreshed phone/tablet/large contact boards and full desktop reference
comparisons. No commit/push/deploy.

Whole-section verification: .cache/responsive-qa/2026-11-07-suite-fit/ retains
the failing before measurements and final screenshots/summary. All14 viewports
passed (the mandatory eight,360x667,390x667,767x900,769x900,1440x768,1835x946),
with98 visual states and14 keyboard/retention checks. At1440x900 the whole
section is900px rather than1217px; title, tabs, all controls and media remain
within its bounds. Stage geometry stays stable across all five categories;
there is no page-wide overflow, panel clipping or browser error. Phone/tablet/
desktop contact boards and alternate platform states were visually reviewed.
Six suite/home component tests passed. All41 protected source hashes still
match, and diff --check passes. Implementation changes are confined to the
third HomeSuite CSS module; no TypeScript/JS, copy, assets or release changed.

## Narrow-screen video follow-up

Owner approved the layout in2026-11-07 07-33-34.mp4: upper tabs, large full-width
image first, left-aligned title/copy/action below, no side categories and gentle
tab changes with stable card dimensions. Apply through1050px, matching the
third landing's existing small-desktop breakpoint. Preserve the wider composition.

Reuse HomeSuite, HomePicture, ModeSwitchPanel and current copy/photos/links. Keep
all three allowed surfaces mounted; narrow surfaces share two CSS subgrid rows
so the longest caption determines one stable copy height. Set the photograph to
16:9 and natural section height at this breakpoint. Narrow surfaces crossfade
opacity350ms; inactive ones are immediately aria-hidden/inert. Desktop inactive
surfaces use display:none. Keep category selection across tabs/resizes and obey
reduced motion. No new dependency, release, source-variant or backend change.

Verification starts with a failing surface/accessibility test and before browser
capture (1024px fails image-first/hidden-categories checks). Check all eight widths,
short phones and1050/1051px; compare1440/1920/2560/3840 geometry to before, then
test frame-level fades, rapid selection, keyboard, resize and reduced motion.
Evidence: .cache/responsive-qa/2026-11-07-suite-narrow/.

Narrow follow-up verified:8 focused suite/home tests, TypeScript and scoped ESLint
passed. Browser checks passed14 viewports/42 platform states: the eight required
widths,360/390/767/769×667 and1050/1051×900. Narrow image/card/copy slots are
stable across all three tabs; 16:9 media, hidden categories, uncut copy and native
page flow passed. All eight primary screenshots visually reviewed. Frame samples
confirmed intermediate opacity and stable height for all three transitions;
rapid changes, six viewport resizes, retained category selection, keyboard and
reduced motion passed. Geometry at1440/1920/2560/3840 exactly matches before.
Protected41 source hashes are unchanged. No commit, push or deploy.
