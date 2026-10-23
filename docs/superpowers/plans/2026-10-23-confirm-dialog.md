# Shared confirmation dialog implementation plan

> Execute inline in the single active checkout; no commit, push or deployment.

**Goal:** Give chat and file deletion one reusable confirmation surface, initially matching the existing chat dialog. The owner approved this design and requested implementation before further visual refinement.

**Architecture:** `components/ui/ConfirmDialog` composes the existing ModalBackdrop, Button, LoadingIndicator and StateNotice. Feature adapters own localized copy, mutation state and callbacks. The common component owns the surface, action layout, cancel-first focus, keyboard containment and pending/error presentation. Backend deletion contracts stay unchanged.

**Constraints:** One localhost:7159 server; no new dependencies; preserve unrelated edits. All copy comes from the current dictionary. Dangerous actions use the same red button. Future visual edits belong to the common CSS module.

## Steps

- [x] Add accessibility and pending-state tests; change the chat integration expectation to safe cancel-first focus. Run the tests and observe the missing behavior.
- [x] Add `ConfirmDialog.tsx` / `ConfirmDialog.module.css`: title, optional lead, description, confirm/cancel/pending labels, error, pending state, callbacks and optional cancel/return-focus refs. Reuse ModalBackdrop animation/scroll lock; focus cancel on open, contain Tab while busy, return focus only when it still belongs to the dialog.
- [x] Replace ConversationDeleteDialog's duplicated structure and remove its styles. Point row-managed archive focus to the cancel button; preserve the mutation and sidebar ownership logic.
- [x] Replace ArtifactDeleteButton's duplicated modal and remove its CSS module; preserve file mutation ordering, retry, timeout and tombstone updates.
- [x] Run affected component/feature tests, full TypeScript check, scoped ESLint and git diff --check. Inspect both actual localhost dialogs without confirming deletion; check narrow-screen CSS and update the UI index/catalog contract.

## Verification commands

From `web/platform`, use the bundled Node runtime with `node_modules/vitest/vitest.mjs run src/components/ui/ConfirmDialog src/components/ui/ModalBackdrop src/features/conversations/ConversationRow src/features/conversations/WorkspaceConversationList src/features/files/artifact-deletion.test.tsx --reporter=dot`; then `node_modules/typescript/bin/tsc --noEmit` and scoped ESLint. Keep the development server's `.next` output intact.

## Evidence

- Initial chat integration test failed as expected: destructive confirmation held focus instead of cancel. The new common component was absent.
- Final affected suite: 9 files, 81 tests passed, including shared accessibility, pending/error, animated close/return focus, chat sidebar sessions/navigation and file deletion integration.
- Full TypeScript check and scoped ESLint passed; git diff --check clean.
- Actual existing localhost:7159 tab checked for both dialog surfaces, cancel-first focus and focus return. Narrow 390x844 view kept text and both actions within the screen; temporary viewport override reset. Screenshots: outputs/confirm-dialog/{file,chat,mobile}.png.
- No real data deleted during browser verification. No backend/API changes, dependencies, alternate checkout/server, commit, push or deployment. Existing optimistic chat removal is preserved; a failed request remounts its confirmation, so focus assertions query the current button.
- Frontend production build not run alongside the active development server; full typecheck, tests/lint and live dev compilation used instead.
