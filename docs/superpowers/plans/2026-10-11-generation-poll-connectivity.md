# Generation polling connectivity

Status: implemented and verified locally on October 12, 2026. User approved pausing generation checks offline and checking
immediately on reconnect; online intervals and existing UI remain unchanged.

## Design and boundaries

Use one browser-only sequential polling coordinator in
`web/platform/src/lib/generation-polling.ts`. It cancels the scheduled read and
aborts the current read while offline, resumes once immediately online, and
disposes timers, requests and listeners on cleanup. Callers check cancellation
after every asynchronous read before publishing private data. Keep existing
image visibility pauses; do not add visibility pauses to other surfaces.
Chat attempt/deadline budgets count connected time and requests only.

Only status/result GETs resume. Preparation, activation, submission, payment and
other writes are never queued or repeated. Backend workers and authorization
contracts are unchanged. No dependencies, commits, paid calls or deployment.

## Tasks

- [x] Add failing coordinator tests: online cadence, initially offline, reconnect,
  cancellation/stale response, duplicate events, cleanup and connected budgets.
- [x] Implement the coordinator and integrate ConversationHistory (text/image/
  video), generated chat titles, ImageJobTracker, FilesWorkspace retry jobs, music and speech polling.
  Preserve music result sharing with the mounted owner and cancel speech reads.
- [x] Add integration regressions proving pending UI/results survive offline,
  including disconnect during status/result retrieval and unmount cancellation.
- [x] Update workspace loading contract and affected UI catalog/index entries.
- [x] Review cancellation, private owner boundaries, terminal results and errors;
  run focused tests, frontend suite, lint/typecheck/build, packaging/assets and
  documentation checks. Report actual local results without deployment claims.

## Review and verification

Reviewed inline: all coordinator consumers resume GETs only; pending jobs and
already loaded results survive disconnect; cancellation is checked before
publishing status/result/title updates. Workspace/session ownership and backend
job/billing contracts remain unchanged. A review regression exposed automatic
retries after a real error for an already completed image; this was corrected
and manual retry was verified together with the full image-generation form.

Focused coordinator/controllers/cache tests pass, including abort during an
image result read, stale music cache reads, duplicate reconnect events, connected
chat budgets, file retry reconciliation and speech result retrieval. Typecheck,
lint, production build, packaging, asset/Next lint contracts and documentation
validation pass. The first full run had only that pre-fix image regression;
the final full run passed: 252 test files / 1781 tests, with 1 file / 4 tests
skipped. No live provider calls, commit, push or deploy.
