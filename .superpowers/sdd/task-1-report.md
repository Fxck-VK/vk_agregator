# Task 1 Report: Account-tab cache boundary

## Status

Completed the account-scoped workspace cache boundary and mounted it below the keyed conversation-list provider.

## TDD record

1. RED: `npm.cmd test -- src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx` failed because `WorkspaceDataCache.tsx` did not yet exist.
2. GREEN: after the cache/provider implementation, the new cache suite passed with 5 tests.
3. RED: the new `WorkspaceFrame` boundary test failed because the cache provider was not yet mounted in the frame.
4. GREEN: mounting the provider inside `WorkspaceConversationListProvider key={accountId}` passed the focused suites.

## Implementation

- `WorkspaceDataCache` stores ready conversation histories in an eight-entry `Map` LRU; reads and replacements update recency.
- Non-ready history states are ignored.
- The cache retains only one parsed image-files first page.
- `WorkspaceDataCacheProvider` owns its cache via `useRef(createWorkspaceDataCache())` and the hook requires that provider.
- A changed account remounts the keyed conversation provider and therefore allocates a new nested cache.

## Verification

- `npm.cmd run typecheck` — passed.
- `npm.cmd test -- src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx` — passed: 2 files, 20 tests.
- Self-review: all source/test edits are in the allowed paths; no browser storage, module-global cache, backend/API, artifact, prompt, price, balance, credential, account-ID, URL, `not_found`, or `unavailable` cache was added.

## Commit

`feat: add account-tab workspace cache`

## Residual risks

- The boundary is ready for consumers; this task intentionally does not wire individual conversation or files data fetches to use it.
