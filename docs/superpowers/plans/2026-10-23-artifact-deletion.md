# Account-owned file deletion

User-approved behavior: deletion removes a file from My Files and its originating
chat, revokes access immediately, purges the original and variants, retries storage
failures, and preserves prompts, other outputs and financial history.

## Design and execution

- [x] Add regression tests for owner/CSRF/idempotency, partial result deletion,
  storage retry, confirmation and cache/chat propagation; verify failures first.
- [x] Add an owner-scoped output deletion repository operation. Lock the parent,
  require its succeeded web job, reject unfinished consumers, mark it unavailable
  and expire all variants atomically. Existing retention performs durable retries.
- [x] Protect job input admission and variant writes against deletion races with
  parent row locks. Keep finalization readiness strict. Deleted result metadata
  is a safe tombstone, never a URL or downloadable artifact.
- [x] Wire a bounded best-effort storage purge and authenticated DELETE endpoint.
- [x] Reuse ModalBackdrop, Button and FileCard for per-file confirmation; propagate
  tombstones through account-scoped cache, previews and conversation rendering.
  Keep preview fixtures separate from real storage and never delete source assets.
- [x] Run focused Go/SQL/frontend checks and inspect the existing localhost:7159 UI.
  Update architecture, retention and UI contracts. Preserve unrelated working files.

No provider calls, credit changes, destructive schema migration, live user-file
deletion, commit, push, deployment or additional local website is authorized.

## Implementation / changed surfaces

- New artifactdeletion service, owner-scoped Postgres deletion and guarded variant
  write operations; web-session DELETE handler, API wiring and rate limit.
- Additive migration 000060 guards deleted inputs, continuations, variants and
  stale status writes. No schema/data deletion is performed by the migration.
- resultservice exposes deleted tombstones only for already succeeded jobs;
  completion readiness stays strict. Image/video/chat DTOs accept tombstones;
  audio results omit deleted readable URLs and keep unrelated outputs.
- Existing maintenance retries explicit expirations independently of age windows.
- Shared ArtifactDeleteButton, FileCard, FilesGrid, FilesWorkspace, VideoFiles,
  MusicFiles/MusicTrackCard and ConversationImageGallery use account-scoped
  deletion state. Music copies delete before the main file to retain retry control.
- Types/dictionaries, workspace cache, preview route/layout/simulator and ignored
  preview deletion metadata support local validation without deleting source assets.
- Architecture, retention, UI/preloading/index docs and state routing updated.

## Verification evidence

- Red/green regression: deleted output initially hid the entire result; new file
  deletion UI/cache tests initially failed for the disabled button/missing state.
- Broad frontend run: 30 files / 263 tests passed; subsequent focused runs passed
  48 tests and 22 tests including busy retry, multi-image preservation and related
  format ordering. TypeScript and scoped ESLint passed.
- Go tests (with default vet): artifactdeletion, resultservice, artifactservice,
  imagepreview, maintenance, websession and cmd/api passed. Explicit Go vet passed.
- API/worker build passed with `-buildvcs=false`: optional Git stamping was
  unavailable in the validation shell. No Git configuration was changed.
- Four real PostgreSQL 16 tests passed in an isolated disposable DB: ownership,
  active consumer/replay/physical cleanup; serialized variant write; uncommitted
  consumer admission; durable cleanup after storage failure. Public Mini App byte
  reads were rechecked and already reject non-ready/deleted output artifacts.
- Existing localhost:7159 UI: confirmation/cancel/focus checked; screenshot saved
  in `outputs/artifact-deletion/confirmation.png`. Disposable preview fixture HTTP
  smoke returned 200 before deletion, 202 for deletion/replay, 404 after deletion
  and 403 for another Origin. The test-only marker was removed afterwards.
- No live user files or paid provider calls were used. DEV/PROD unchanged. Frontend
  production build was not run alongside the active development server; live dev
  compilation, full typecheck, scoped tests and lint were used instead.

Operational limits: storage cleanup can remain pending during outages; existing
backups and previously downloaded bytes follow separate lifecycles. Related audio
formats are independently owner-checked and replayed after partial failure. The
local preview keeps source assets intact; real physical purge is a backend action.
