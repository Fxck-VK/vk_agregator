# web/platform/AGENTS.md - Web Platform Local Rules

This file applies to `web/platform/**`. Read the root `AGENTS.md` first.

## Role

The standalone web platform is a thin client over the shared backend. It is an
independent deployable frontend, not a provider client, billing authority, or
data store.

## Must Not

- Do not call AI, payment, VK, storage, or database providers directly.
- Do not store backend secrets, provider keys, refresh tokens, or raw PII in
  source code, browser storage, logs, analytics, or error reports.
- Do not trust client-supplied account IDs, roles, balances, prices, job
  statuses, ownership claims, or moderation results.
- Do not render user/provider content as trusted HTML.
- Do not expose raw provider or private storage URLs.
- Use the versioned `/web/v1` API rather than retired client contracts.

## Must

- Use the shared Account Layer and server-issued web sessions for identity.
- Use backend APIs for jobs, payments, ledger-backed balances, and artifacts.
- Send stable idempotency keys for paid or otherwise repeat-sensitive writes.
- Treat disabled controls as UX only; backend idempotency and authorization
  remain mandatory.
- Keep authentication material in secure, server-managed cookie/session flows;
  never put credentials in query strings or local storage.
- Escape untrusted content and normalize user-facing errors.
- Keep the app independently buildable, testable, and deployable from the
  operator frontend.

## Implementation Gate

Only Russian website pages may be published on DEV or public servers until the
owner explicitly asks to introduce English. Keep `siteLocales = ["ru"]`, legacy
`/en` → `/ru`308 redirects and Russian-only links/SEO. The mandatory
`scripts/site-language-policy.mjs` build/source gate must remain enabled.
English source dictionaries and reverse audits are retained; they are not an
enabled website language. No environment-specific opt-in is authorized.

The app scaffold and shared components already exist. Reuse the current package
configuration and scripts. New dependencies or runtime surfaces must belong to
the user-authorized feature and include the relevant lint, typecheck, tests,
build, packaging and CI checks for that change.

## Mandatory Responsive Layout Verification

For changes to layout, sizing, typography, text wrapping or responsive behavior,
check affected pages in a browser at every width below. Widths are the actual
CSS viewport (`window.innerWidth`), not physical screen pixels or screenshot
dimensions. Use 100% browser zoom for the baseline and record viewport height,
zoom and device-pixel ratio; a high-DPI screenshot of a narrow viewport does
not replace a wide-viewport check.

| Target | Required viewport |
| --- | --- |
| Small phone | 360 CSS px wide |
| Phone | 390 CSS px wide |
| Tablet portrait | 768 CSS px wide |
| Tablet / small desktop | 1024 CSS px wide |
| Desktop | 1440 CSS px wide |
| Full HD | 1920 CSS px wide |
| 2K / QHD | 2560 x 1440 CSS px |
| 4K / UHD | 3840 x 2160 CSS px |

- The baseline audit covers one representative page for each of T01-T24,
  including applicable course/help/legal child layouts. Workspace and auth
  pages follow the same width matrix. A focused change checks affected pages;
  a shared layout change includes its affected consumers.
- Inspect header/sidebar alignment, text wrapping, card grids, images/video,
  tables, content width, empty space, scroll regions and access to controls.
  Check menus, dialogs, tabs and available loading/error/long-content states.
  Prevent unintended page-wide horizontal scrolling, clipping and overlaps;
  intentional local table/tab scrollers must remain usable.
- Check intermediate widths near changed CSS breakpoints when a layout changes
  between required widths. Keep the existing approved visual design.
- Save route/template, actual viewport, checked states, outcome and screenshot
  paths in ignored `.cache/responsive-qa/` evidence. Use local preview fixtures
  where appropriate; visual verification does not authorize paid actions.
- State exactly which widths/pages passed, failed or were not run. A successful
  build, lint, DOM/unit test or previous partial desktop/mobile check does not
  replace browser layout checks. If browser access is blocked, record that
  limitation and do not mark those cases passed.

This is the required verification procedure, not a statement that the existing
24 templates have already passed the complete matrix.

## UI Reuse Workflow

Preserve the user's existing visual design when implementing billing, business
logic or data integration. Those tasks do not authorize simplifying cards,
removing visual sections/badges/icons, or changing layout, sizes and spacing.
Change the presentation only when the user explicitly requests that visual work.

Resolution selectors must use the shared `ImageQualitySelector`: one option per
actual catalogue size (1K, 2K, 4K, 8K where supported), without medium/high/ultra/pro
suffixes. The shared projection retains the selected backend tier where
available and never synthesizes codes or unsupported sizes. Preserve the raw
request code, catalogue defaults and pricing; see the root resolution UI policy.

For UI changes and text-only UI proposals within `web/platform`, use this order:

1. Find the task in [the short UI index](docs/ui-index.md). If it is already in
   the current task context, reuse it; reread affected entries only as needed.
2. Read only the linked section of [the catalog](docs/ui-catalog.md) and, when
   needed, the matching [example](docs/ui-catalog-examples.md). Do not load the
   entire catalog, all examples or the archived inventory by default.
3. Check the actual component export, props, shared styles and one current
   consumer before recommending or changing it. The catalog is a navigation
   aid; current code determines implemented behavior and the user's request
   determines intended behavior. Do not present an unfinished action as working.
4. Reuse the existing component/style, or extend it when the intended behavior
   fits its purpose. Check other affected consumers when extending a shared
   contract. If no indexed solution fits, search the relevant source directories
   before adding a new component; briefly explain why a separate one is needed.
5. In the same change, update the corresponding catalog entry and any affected
   index row/example when props, behavior, defaults, paths, limitations or reuse
   recommendations change. Keep still-accurate entries as they are; do not
   regenerate the archived inventory for routine work. Check changed links and
   TypeScript examples when their code changes.

When relevant, tell the user which existing component or style already covers
their request. Keep implementation within the requested scope: a text-only
proposal remains text-only, and UI-only documentation is not needed for unrelated
backend work. These instructions add no separate approval step.
