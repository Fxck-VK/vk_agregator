# Unified Model Catalog Implementation Plan

> Execute the user-approved migration in the existing isolated worktree. Use
> test-driven-development and bounded subagent implementation/review where work
> is independent. No new approval checkpoint is needed for this approved design.

**Goal:** Make all functional web model lists and controls consume one backend catalog.

**Architecture:** Extend productcatalog and expose one authenticated safe web DTO;
project compatibility loaders from one frontend request cache. Keep private
provider routing and authoritative price calculation in existing services.

**Tech stack:** Go, existing Next.js/React/TypeScript and Zod; no new dependencies.

## Global constraints

- No provider calls from API/frontend; no new real provider tests without scoped budget.
- No invented capabilities, live evidence, enabled routes or refreshed legacy exemptions.
- Preserve current header/new-chat versus in-conversation model selection behavior.
- Reuse existing selectors, cards, controls and all six category labels.
- Do not change payment/auth/ownership/idempotency boundaries or unrelated dirty files.
- No commits, push, deploy or destructive migrations.

## Task 1: Backend catalog and endpoint

Files: new `internal/service/productcatalog/workspace*.go`, new
`internal/adapter/inbound/websession/model_catalog*.go`, existing handler routing,
productcatalog description/quality helpers and generation validators.

Interface: `WorkspaceCatalog(WorkspaceConfig) WorkspaceModelList` returns the
schema documented in the design. Config consumes ready text/image/video models
and the pricing snapshot interface. Names and constraints derive from the existing
registry/resolvers; descriptions and categories have one backend owner.

- [x] Write tests for one text/image/video list, unique IDs, default membership,
  typed controls, safe serialization, disabled/unpriced routes and combination coverage.
- [x] Run `go test ./internal/service/productcatalog ./internal/adapter/inbound/websession`
  and confirm failures for the new behavior before implementing it.
- [x] Implement the builder and endpoint. Keep old endpoints as compatibility views.
- [x] Test server selection rejects combinations absent from the public catalog.
- [x] Run the two packages and model onboarding validation; inspect the scoped diff.

## Task 2: One frontend loader and projections

Files: `features/models/model-catalog-contract.ts`, `model-catalog-cache.ts`,
`generation-model-catalog.ts`, image/video/chat compatibility loaders and tests.

Interface: `loadModelCatalog()` parses `/web/v1/models`, shares in-flight requests
and caches successful responses for 60 seconds. Legacy adapters return their
current UI shapes plus server `description` and `categories`; they contain no
independent model data. `loadGenerationModelCatalog` preserves its consumer shape.

- [x] Add failure-first tests for a single shared request across image/text/video,
  missing/invalid defaults, duplicate IDs, strict safe DTOs and retry after failure.
- [x] Implement schema, cache and projection adapters. Run focused Vitest tests.
- [x] Migrate local preview to a backend-generated catalog fixture and route all
  existing preview catalog views through it. Preserve safe local-only preview gating.

## Task 3: Consumers, cards, filters and controls

Files: ModelsCatalog, ModelCard, ModelSelector sections, WorkspaceModelSelector,
ConversationModelSelector, WorkspacePrompt, image generation/file tools, affected
tests and UI documentation.

- [x] Add tests exposing the current text-category mismatch and model description
  divergence, and proving categories reorder with at most five entries in pickers.
- [x] Render all categories from common data; full catalog is not limited to five.
  Remove functional placeholder entries and independent model description tables.
- [x] Keep curation as IDs only. Use response defaults and valid video combinations
  in controls; retain backend estimate/submit as authoritative paid request checks.
- [x] Audit every functional consumer and migrate remaining direct fetches/lists.
- [x] Run focused UI tests, then typecheck/lint/build and required packaging checks.

## Task 4: Verification and durable documentation

- [x] Record existing-model evidence gaps without upgrading legacy status. Prepare
  a concrete matrix for any later live verification; do not mark unrun checks passed.
- [x] Update architecture/runbook/UI routing for the actual final catalog ownership.
- [x] Run backend regressions, go vet, frontend checks, no-independent-list guard,
  and inspect final status. Have a bounded independent reviewer inspect the migration.
- [x] Report completed migration and remaining factual provider-verification limits.

## Completion evidence — 2026-09-14

The functional web migration is implemented. All consumers use `/web/v1/models`
through the shared validated loader; modality-specific compatibility APIs and
loaders are projections. Curated selections contain IDs only. Server controls
and priced variants drive the UI; generation validation and billing remain on
the backend. No provider execution or financial authority moved into the UI.

Validation:

- Full frontend suite: **192 files, 1280 tests passed**.
- TypeScript, ESLint and production build passed. Asset tests: **6 passed**;
  asset validation and standalone packaging checks passed.
- Backend regression tests: 12 packages passed; preview command compiled.
  Relevant `go vet` checks passed.
- Registry admission check passed with **43 unchanged legacy-unverified entries**.
  Generated preview check passed with **36 eligible offline entries**.
- Every advertised image/video option resolves through the existing server
  resolver without provider calls. Regression checks cover sparse priced image
  defaults, paid/free category integrity, malformed catalog failures and file
  selector category preservation.
- Independent backend and frontend reviews found no remaining actionable
  findings after fixes. Browser checks covered full catalog/text category,
  header/new-chat selection, existing-dialog model changes with preserved draft,
  and the home-page text/image form. No generation was submitted.
- Final `git diff --check` passed. The working tree already contained unrelated
  changes before this migration; they were preserved. No commit/push/deploy.

One full-run history regression exposed leftover one-shot API mocks and an
assertion that started before accepted-send scrolling settled. Test isolation
and the wait boundary were corrected; the final full suite above passed.

This completes catalog integration, not factual re-verification of all provider
capabilities. The [43-record matrix](../../runbooks/model-onboarding/existing-models-2026-09-14.md)
records documentation/contract/admission gaps and unrun live cases. Unknown
inputs remain disabled and unknown video FPS/audio remain null. The two mock
entries require offline checks only; the 41 real entries require an explicitly
scoped DEV/provider/budget authorization before live execution, as specified in
[MODEL_ONBOARDING.md](../../runbooks/MODEL_ONBOARDING.md).
