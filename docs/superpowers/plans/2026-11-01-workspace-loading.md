# Workspace loading implementation plan

> **For agentic workers:** Use executing-plans for integration and the bounded
> independent tasks described below. The owner has approved the design; execute
> in the current checkout. No worktree, commit, push or deployment.

**Goal:** Remove redundant cold catalogue work and static transfers while
preserving the approved workspace UI and backend authority.

**Architecture:** Nginx privately revalidates only successful build assets. The
server streams one validated catalogue promise to the shared client cache and
provider; bounded non-personal session state supports immediate presentation.
The existing player gains opt-in poster deferral; existing local diagnostics
gain safe read timings.

**Tech Stack:** Next.js16.4.2/React19.2.7, TypeScript, existing Nginx, Vitest,
Playwright and Sharp. No added dependencies.

**Status:** implemented and verified locally on2026-11-01. No commit/push/deploy.

**Evidence:** ignored `.cache/video-loading-2026-11-01/` contains baseline/after
browser measurements, RED regressions,339 integrated tests,60 final regression
tests (overlapping suites),37 native Nginx gate tests, zero-advisory npm audit,
infrastructure, types, production build, packaging/assets and lint outputs.
`.cache/responsive-qa/2026-11-01-workspace-loading/report.html` and `results.json`
contain8-width screenshots, native playback and isolated actual-component
catalogue scenarios. Cold300ms delivery rendered in306ms without a browser
catalogue request; a stored catalogue appeared in36ms and refreshed in304ms.
Server failure used one browser fallback. These are controlled local scenarios,
not authenticated DEV latency. A preliminary build failed on a sandbox-denied
Windows junction; the permitted native rerun passed without source workarounds.

**Visual limit:** all16 screenshots inspected. Video/poster checks pass at every
required width. At768px the hero heading clips: the existing CSS in HEAD has
`white-space: nowrap`, with normal wrapping only below768px. No CSS changed in
this task; no before/after screenshot proof is asserted. Overall layout acceptance
at768px remains open, while the other seven inspected widths showed no visible
header/sidebar/composer/model overlap. The task preserves the approved geometry.

**Review:** independent review findings about original snapshot age, direct
account transitions, UTF-8 cache bounds, promise-change subscriber disposal and
missing read diagnostics were reproduced/resolved. Final scoped re-review found
no remaining blocking defects.

## Global constraints

One checkout and localhost:7159; Russian-only; existing layout unchanged;
auth, backend quotes, admission and private-data boundaries preserved.
See ../specs/2026-11-01-workspace-loading-design.md for the approved contract.

### 1. Private revalidation of static files

Files: deployments/nginx/dev-web.conf and the existing DEV gate regression tests.

- [x] Add negative/conditional-response tests: unauthorized assets rejected;
  successful assets private/revalidated; account/HTML/errors no-store.
- [x] Observe RED, implement the narrow response policy and preserve the gate.
- [x] Run existing gate and deployment contract tests. Report actual statuses.

### 2. Optimized deferred poster

Files: VideoPlayer component/tests, WorkspaceLanding consumer, asset-paths,
workspace poster WebP and matching UI catalog entry.

- [x] Add RED tests for opt-in viewport loading and unchanged default playback.
- [x] Encode the existing poster with the installed image tooling, compare the
  image visually, retain the old PNG and register the new asset.
- [x] Implement observer-driven poster loading without a new layout wrapper.
- [x] Verify poster requests wait offscreen, then playback and replay work.

### 3. Single catalogue bootstrap and safe retained state

Files: model-catalog-server, model-catalog-cache, GenerationCatalogProvider,
catalogue contract types, workspace layout/frame and logout cache cleanup.

- [x] Add RED tests for delayed server delivery, stale snapshots, corrupt or
  expired storage, account invalidation and late-response suppression.
- [x] Replace the 200ms handoff gap with an available snapshot plus a streamed
  pending result. Adopt that result before child loaders run.
- [x] Restore a bounded validated session snapshot after hydration; preserve
  freshness age and invalidate on logout/session changes.
- [x] Keep shared browser/server reads coalesced and fallback through the
  existing authenticated API only after server delivery fails.
- [x] Run catalogue, hydration, ownership and affected consumer regressions.

### 4. Safe read measurements and integrated verification

Files: workspace-navigation metrics, identity delivery metadata, model read,
balance and conversation loaders, diagnostics tests and preloading docs.

- [x] Add RED tests for numeric bounded operation-only timings and disabled
  collection on public hosts; exclude payloads/identity and duplicates.
- [x] Measure completed reads and server identity/catalogue delivery in the
  existing local/DEV in-memory diagnostics, without visible UI changes.
- [x] Run focused Vitest checks, lint, types, asset checks and relevant native
  infrastructure regressions. Production build/packaging when integration is ready.
- [x] Verify all eight required widths with screenshots and cold/warm
  browser metrics. Keep evidence in ignored .cache/responsive-qa/.
- [x] Request a narrow independent review; resolve important findings.
- [x] Update preloading/DEV architecture documentation and index routing;
  report local-only status, measured improvements and remaining DEV limitations.
