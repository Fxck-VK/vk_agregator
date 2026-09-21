# Workspace preloading implementation plan

Goal: implement the approved loading architecture across the existing workspace.
Architecture: streamed shell, bounded server catalog seed, shared browser resource,
independent private resources and authorized image previews. Reuse AsyncState,
WorkspaceDataCache, the existing API/auth boundary and the simulator.

Constraints: no provider/payment calls, no commits/deploys, no shared private cache,
no made-up balance, no automatic write retries; preserve the dirty working tree.

- [x] Catalog: add server cache/seed and a provider + hook; migrate header, composer,
  new chat, model listing and home model blocks to the same snapshot. Test SSR,
  deduplication, stale/error recovery and scope reset.
- [x] Shell: put identity resolution behind Suspense; load balance/navigation
  independently. Add bounded read requests and a shared delayed/offline notice.
  Test slow and failed supplementary requests with a working composer.
- [x] History/files: retain successful snapshots and image-result metadata,
  avoid composer remounts on refresh, preserve unsent text/files on failures.
  Test refresh with a draft, cached error + retry and account changes.
- [x] Media: serve small previews only after normal artifact authorization;
  constrain decode size and work concurrency, lazy-load cards/thumbnails,
  keep full originals for the viewer. Test authorization and payload size.
- [x] Verify: targeted unit tests, throttled/offline browser scenarios, typecheck,
  lint/build/packaging; update architecture/UI docs with the final behavior.

Reference: web/platform/docs/preloading.md. Execute inline in the current checkout.

Verification completed locally on 2026-09-20:

- Full Vitest suite: 1488 passing. After the final notice placement adjustment,
  all 131 affected component tests passed again.
- TypeScript, ESLint, production build, packaging and six asset tests passed.
- Go websession adapter tests passed, including public image-job geometry.
- Browser: four preloading scenarios, four attachment scenarios, five multi-image
  scenarios and seven missing-page scenarios passed. The final history scenario
  also asserts that delayed feedback stays below the header.
- Visually inspected the slow-history screenshot: stable shell, aligned notice,
  reserved history geometry and editable draft. No live provider/payment requests.

Remaining visual work is the user's separate item-by-item design review, not an
unfinished data-loading task. In-memory drafts do not survive a full page reload.
