# Promote the completed third landing to the canonical home

User-approved scope: replace the current `/ru` landing with the collaboratively
refined third variant. Preserve original/friend/third comparison pages, the
shared T01 data source, current publication status and signed-in routing.
Work in the existing checkout/server. No commit, push or deployment requested.

- [x] Verify the canonical home renders the chosen layout, while original preview
  stays independent of the canonical T01 renderer (existing regression tests).
- [x] Switch only T01's renderer to the existing third component; explicitly
  render the preserved original in its guarded preview using the same RU data.
- [x] Preserve auth redirect/recovery, preview access, draft/noindex, catalogue
  inputs, and all other template branches. Update current routing documentation.
- [x] Run scoped tests/types/lint and native browser checks for `/ru` at 360,390,
  768,1024,1440,1920,2560×1440,3840×2160; check comparison routes and intro video.


## Verification

- T01 uses `features/landing-variants/third/PublicHome`; original preview directly
  imports `features/public-home/PublicHome`. Both use existing T01 RU data.
- 44 affected tests passed (canonical home/session/noindex, guarded previews,
  all24 data-driven template renderers and third-home interactions).
- TypeScript and scoped ESLint passed; scoped git diff --check passed.
- Native Chromium/DPR1/zoom100% passed at 360×844,390×844,768×1024,1024×768,
  1440×900,1920×1080,2560×1440,3840×2160: hero, creative-suite, models,
  workflow, tools, possibilities and originals. Screenshots visually reviewed.
- Canonical/original/friend/third routes all HTTP200 in trusted DEV and keep
  their intended distinct layout. Root remains noindex/nofollow, English URLs
  redirect to RU, and intro uses the existing Epifera MP4 with native controls.
- Ignored evidence: `.cache/responsive-qa/2026-11-08-promote-third-home/`:
  `summary.json`, `route-smoke.json`, screenshots and contact sheet.
- Historical home implementations and the other23 renderer branches were not
  edited. Publication status and account/provider/billing policies stay intact.
- No commit, push, deployment, paid action or real account mutation.
