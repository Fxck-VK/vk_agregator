# Models Catalog Visual Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` or `executing-plans` to implement this plan task-by-task.

**Goal:** Give `/app/models` the approved catalogue layout while retaining only truthful API-backed model information and existing generator navigation.

**Architecture:** `ModelsCatalog` keeps loading, search, and selected-category state. `ModelCatalogToolbar` becomes the reusable search-and-category control. `ModelCard` derives presentation details from `ImageModel`; no backend contract changes are required.

**Tech Stack:** TypeScript, React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve `loadImageModelCatalog` as the only data source.
- Do not invent prices, provider names, ratings, usage counts, or descriptions.
- Keep per-model links unprefetched and URL-encoded.
- Keep the two-column desktop / one-column mobile layout.
- Do not change the backend.

## Task 1: Add catalogue behaviour tests

**Files:** `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx`, `web/platform/src/features/models/ModelCard/ModelCard.test.tsx`, and `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.test.tsx`.

- [ ] Add assertions for the section heading, category tabs, minimum verified price, and placeholder categories.
- [ ] Run the focused models test command and confirm the tests fail because the new UI does not yet exist.

## Task 2: Implement search, categories, and truthful card data

**Files:** `web/platform/src/i18n/ru.ts`, `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`, `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx`, and `web/platform/src/features/models/ModelCard/ModelCard.tsx`.

- [ ] Replace the old technical filter controls with one search field and the category tab list.
- [ ] Keep current models under `Популярные` and `Изображения`; render a clear future-category state for unsupported categories.
- [ ] Derive a card price only from `price_by_quality` and retain the secure generator link.
- [ ] Run the focused models test command and confirm it passes.

## Task 3: Apply the responsive visual design

**Files:** `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.module.css`, `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css`, `web/platform/src/features/models/ModelCard/ModelCard.module.css`, and `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`.

- [ ] Add a full-width search, horizontal chip scrolling, dark NeiroHub cards, a desktop two-column grid, and a mobile single-column grid using global tokens.
- [ ] Add stylesheet assertions, then run the focused models test command.

## Task 4: Verify and deploy the frontend

**Files:** only the reviewed files from Tasks 1–3 plus these documents.

- [ ] Run the complete frontend test suite, typecheck, lint, and production build.
- [ ] Commit only reviewed files, push the current commit to `dev-deploy`, and wait for CI, Docker image, and Deploy DEV workflows for that SHA to succeed.
