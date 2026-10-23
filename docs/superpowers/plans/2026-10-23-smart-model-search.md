# Smart model search implementation plan

**Approved scope:** Search all models supplied to the existing selector, regardless
of its active category or five-item curated sections. Recognize Russian model
names, partial names, minor typos and a wrong keyboard layout. Rank exact matches
first. Do not add models, dependencies, paid requests or provider capabilities.

**Architecture:** A pure `searchModels<T extends { id: string; name: string }>`
helper returns the original model objects in relevance order. The selector uses
one deduplicated results section for a nonempty query; clearing it restores the
existing category feed. Full-catalog filtering reuses the matcher while retaining
its explicit category filter. Availability remains the caller/backend's boundary.

## Steps

- [x] Add regression tests for models beyond the five-item limit, category-independent
  selector search, selection, clearing, aliases, typos, keyboard layout, ranking,
  exact version numbers, unrelated queries and unavailable models.
- [x] Run the focused tests and confirm the intended failures.
- [x] Add the dependency-free matcher and connect the existing selector/catalog.
  Add localized results labels and update the affected UI catalog/index entries.
- [x] Run the focused model tests, TypeScript and scoped ESLint; inspect the live
  existing localhost:7159 selector at its existing viewport and inspect the final diff.

**Constraints:** Work in the single active checkout. Preserve unrelated dirty files.
Do not create another preview or worktree. Do not commit, push or deploy.

## Verification

- 22 test files / 185 tests passed: search, selectors, catalog, conversation/file
  adapters, generation catalog and i18n contracts. New regression assertions first
  reproduced the curated-limit/Russian-input failures. A separate red/green check
  covered preserving numeric version order (6.5 must not match 5.6).
- Full TypeScript check and scoped ESLint passed. `git diff --check` passed.
- Existing browser UI confirmed `нана банана про` finds Nano Banana Pro outside
  the five-item collections, `пзе` finds GPT models, and selecting the result
  navigates to `/ru/app/chats?model=nano_banana_pro` without submitting a job.
  Screenshot: `outputs/smart-model-search/desktop.png`.
- No layout/styles changed. Browser viewport was preserved; no separate mobile
  viewport run or production build was needed for this client search change.

## Changed surfaces

- `web/platform/src/features/models/model-search.ts` and its unit test:
  bounded matching/ranking, version digits, supplied-set restriction and dedup.
- `web/platform/src/i18n/model-search-data.ts`, `ru.ts`, `en.ts`: shared input
  vocabulary and translated results heading, independent of selected UI locale.
- `WorkspaceModelSelector/ModelSelector.tsx` and three selector tests: global
  results without curated limits, normal selection, focus and cleared-query feed.
- `ModelsCatalog/model-filters.ts` and test: shared matching with category retained.
- UI catalog/index and this plan/index routing document durable behavior.

No auth, billing, provider routing, model admission or backend contracts changed.
