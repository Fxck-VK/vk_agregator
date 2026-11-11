# Unified landing palette

Status: completed locally; not committed, pushed or deployed.

Owner request: merge similar colours on the canonical/third landing while
preserving layout, photographs and the exact existing brand gradient.

1. Capture geometry, image sources/presentation, gradient and representative
   states at360/390/768/1024/1440/1920/2560/3840 CSS px.
2. Define the local semantic palette; migrate neutral text/surfaces/borders,
   shadows and accent colours. Shared components gain optional colour variables
   only, with existing fallback values for other consumers. Workspace behaviour
   and the saved original/friend variants retain their existing appearance.
3. Compare the same states/widths after the change, re-count colours with the
   same audit method, check contrast, and run affected existing checks.

Photographic assets, overlays, dimensions, fonts, spacing, interactions, billing
and publication rules remain unchanged. Code-drawn workflow illustrations may
adopt the shared palette without changing their geometry or source photographs.
No commit, push, deployment or paid action is authorized.

Implemented a local semantic palette in `third/Palette.module.css`, applied to
the canonical/third landing and its intro portal. Neutral text, surfaces,
borders, shadows and code-drawn illustrations now share those variables.
Shared component extensions are optional colour slots with unchanged defaults;
the workspace, original/friend variants and other public templates retain their
appearance. Brand-gradient definitions and media files were not edited.

Verification:

- Native Chromium before/after at360×844,390×844,768×1024,1024×768,1440×900,
  1920×1080,2560×1440 and3840×2160 CSS px,100% zoom,DPR1:8/8 passed. Layout/font
  geometry, media sources/presentation, photographic overlays and actual tariff
  gradients match. No horizontal overflow or page errors.
- Intro dialog, all three suite tabs, expanded FAQ and keyboard focus passed at
  390 and1440. Saved original/friend footer colours and workspace tariff-modal
  text retain their previous values.
- Existing scoped suites:77 tests in14 files passed. TypeScript and scoped
  ESLint passed. No new implementation-mirroring tests were added.
- Same rendered-CSS counting method as the preceding audit:53→18 base RGB
  colours,69→40 alpha variants,20→6 text colours; photographs/video/SVG artwork
  excluded. Preserved photographic shades and gradient endpoints are included.
- Solid main-text combinations have contrast ≥5.94:1; dark text on the three
  brand-gradient endpoints has contrast ≥6.42:1.

Ignored evidence: `.cache/responsive-qa/2026-11-09-unified-landing-palette/`
(`before.json`, `after.json`, `states.json`, `colours-after.json`, screenshots).
