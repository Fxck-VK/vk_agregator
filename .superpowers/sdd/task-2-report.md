# Task 2 Report: Bounded route prefetch and DEV navigation metrics

## Status

Completed the fixed workspace-route prefetch configuration and the browser-only DEV metrics boundary without visible UI changes.

## TDD record

1. RED: `npm.cmd test -- src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx` failed because the metrics module/component did not exist and `workspaceNavigationItems` was absent.
2. GREEN: implemented the category-only metric module, nonvisual click/pathname component, and the five fixed Sidebar prefetch records.
3. Focused click-test correction: the JSDOM host stub used port 80 while relative anchors resolved to JSDOM's port 3000, so the intended same-origin guard rejected the click. Aligning the stub with `window.location.origin` made the test deterministic without changing production navigation behavior.

## Implementation

- `workspaceNavigationItems` is exported with exactly five fixed routes, each rendered with `prefetch={item.prefetch}`.
- Metrics run only on `localhost`, `127.0.0.1`, and `dev-web.neiirohub.ru`; their window array is capped to the newest 50 records.
- Records are reconstructed from category, source, and rounded finite duration only; paths, query strings, IDs, and other user data are never retained.
- The persistent `WorkspaceFrame` mounts a client-only component that observes unmodified primary same-origin `/app` link clicks in capture phase and completes the measurement after `usePathname()` changes.

## Verification

- `npm.cmd test -- src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx` — passed: 3 files, 61 tests.
- `npm.cmd run typecheck` — passed.
- `git diff --check` — passed.
- Self-review: only the allowed source/test paths and this required report changed; no loading UI, network calls, analytics vendors, cookies, browser storage, server endpoints, or recent-chat prefetch changes were introduced.

## Commit

`feat: prefetch fixed workspace routes`

## Residual risks

- This task provides the data-load recording API but intentionally does not yet wire private files or conversation fetches to call it.
