# Immediate return to chats and files

Approved behavior: reopening chats and files displays the last successfully loaded data immediately, while a background read checks for updates. No new stale-data delay or UX redesign.

## Scope and boundaries

- Keep the existing account-owned conversation history and image files caches, including their current bounds and error/retry behavior.
- Revalidate the shared conversation list when an authenticated chat screen mounts. Coalesce with an existing read; preserve pending local conversations and drafts.
- Add the missing video first-page and result metadata cache to `WorkspaceDataCache`. Store at most 60 video results; keep private data in account-owned memory only.
- Cached video cards (including an empty successful page) remain visible during refresh and temporary errors. Explicit refresh still replaces the first page as before.
- Abort video list reads on leave/replacement and reject late responses. Account changes dispose the existing keyed workspace provider.
- Do not change authentication, access checks, media delivery, pagination semantics, mutations, pricing or billing.

## Implementation and verification

1. Add failing navigation tests for cached video returns and the conversation-list refresh. Add real leave/return coverage for existing image files and chat history.
2. Extend the account-owned cache and wire video initial state, successful reads and warm error feedback.
3. Add a guarded conversation-list revalidation hook to authenticated chat entry screens; leave server-seeded standalone providers unchanged.
4. Check delayed responses, warm errors, empty pages, draft/preview continuity, StrictMode cleanup and account isolation.
5. Run focused regressions, lint, typecheck, production build, packaging and documentation validation. Keep previous uncommitted work intact. No push/deploy is requested.

## Completed locally

- The navigation regressions initially reproduced five failures (video cache/empty page/warm errors and missing chat-list entry refresh), then passed after implementation.
- Actual image/history leave-and-return tests preserve a draft, the selected category and an open preview during revalidation. Video tests cover warm failure/retry, empty pages, account boundaries and discarded StrictMode responses.
- Full Vitest: 250 files passed, 1 skipped; 1748 tests passed, 4 skipped.
- `npm run lint`, `npm run typecheck`, `npm run build`, `npm run test:packaging`, `npm run test:assets`, `npm run test:next-lint-contract` passed.
- Documentation validation and whitespace checks passed. Source review confirms account-owned memory only for added private metadata; no changes to authorization or write paths. Previous work remains uncommitted alongside this change on `fastlife_dev`.
