# Task 2 Fix Report: Bound metrics observer lifecycle

## Status

Completed the three Important review fixes for bounded DEV navigation metrics.

## TDD record

1. RED: the new production-host component test observed one document click listener registration.
2. RED: `_parent`, `_top`, and named-frame link targets each emitted an unrelated fixed-route metric.
3. RED: a 10,001 ms pending navigation emitted a stale metric.
4. GREEN: the metrics regression suite passed all 11 tests after the minimal host-gate, target, and lifecycle changes.

## Fixes

- Exported `isWorkspaceMetricsEnabled()` from the metrics module and returned from the observer effect before document-listener registration on non-DEV hosts.
- Accepted only absent or `_self` anchor targets; all named frames, `_parent`, `_top`, and `_blank` are rejected.
- Replaced the per-target module `Map` with one pending category/timestamp/timer record. A new start replaces the old one; completion clears it; a 10-second timer expires it; durations over the same bound are ignored.
- The pending lifecycle and resulting metrics retain only a route category, timestamps, and timer ID—never URLs, search data, conversation IDs, text, or account data.

## Verification

- `npm.cmd test -- src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx` — passed: 3 files, 66 tests.
- `npm.cmd run typecheck` — passed.
- `git diff --check` — passed.
- Self-review: only Task 2 metrics tests/module/component and this required report changed; no UI, network, storage, or unrelated navigation behavior changed.

## Commit

`fix: bound workspace navigation metrics`

## Residual risks

- The 10-second bound intentionally drops unusually slow client navigations rather than retaining a potentially stale measurement.
