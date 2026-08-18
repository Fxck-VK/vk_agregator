# Public Design System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Formalize the current NeiroHub visual language as reusable public UI and wrap public routes in an independent `PublicShell` without changing `/app`.

**Architecture:** Extend the existing CSS custom-property contract, implement small server-first public primitives in isolated component folders, and keep only theme preference interaction in one client component. Compose those primitives through a public-only shell in the existing `(public)` route group.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript 5.9, CSS Modules, Vitest 4, Testing Library.

## Global Constraints

- Preserve the existing NeiroHub colors and overall character.
- Do not modify `/app`, its shell, session flow, sidebar, workspace header, or account behavior.
- Do not add public content templates beyond the existing `/` page.
- Do not expose prices, balances, availability, or other backend-owned facts in public components.
- Keep every React component in its own folder with a colocated CSS Module.
- Do not commit or push unless the user explicitly requests it.

---

### Task 1: Semantic design-token contract

**Files:**
- Modify: `web/platform/src/app/globals.css`
- Modify: `web/platform/src/app/globals.theme.test.ts`

**Interfaces:**
- Produces stable CSS custom properties consumed by every later public component.

- [ ] Write failing assertions for container, typography, radius, shadow, header, interaction, and status-color tokens in dark and light palettes.
- [ ] Run `npm test -- src/app/globals.theme.test.ts` and verify failure is caused by the missing tokens.
- [ ] Add the minimal CSS custom properties while preserving every existing token value.
- [ ] Re-run the focused test and verify it passes.

### Task 2: Public content primitives

**Files:**
- Create one `.tsx` and one `.module.css` under `web/platform/src/components/public/` for `PageContainer`, `SectionHeading`, `PrimaryButton`, `SecondaryButton`, `ContentCard`, `ModelPreviewCard`, `FAQ`, and `EmptyState`.
- Create: `web/platform/src/components/public/public-components.test.tsx`

**Interfaces:**
- Produces typed, account-independent presentation components for later public templates.

- [ ] Write failing semantic rendering tests for all eight primitives.
- [ ] Run the focused component test and verify missing-module failures.
- [ ] Implement the minimal typed components and shared visual behavior using only global design tokens.
- [ ] Re-run the focused component test and verify it passes.

### Task 3: Public theme switcher and shell

**Files:**
- Create component folders for `PublicThemeSwitcher`, `PublicHeader`, `PublicFooter`, and `PublicShell`.
- Create: `web/platform/src/components/public/PublicThemeSwitcher/PublicThemeSwitcher.test.tsx`
- Create: `web/platform/src/components/public/PublicShell/PublicShell.test.tsx`
- Modify: `web/platform/src/i18n/public/ru.ts`
- Modify: `web/platform/src/i18n/public/en.ts`

**Interfaces:**
- `PublicThemeSwitcher` consumes the existing `readThemePreference` and `applyThemePreference` APIs.
- `PublicShell` consumes the exact `PublicDictionary` contract and renders header, main, and footer.

- [ ] Write failing tests for theme selection and shell landmark composition.
- [ ] Run focused tests and verify failure from missing components/dictionary fields.
- [ ] Add exact bilingual labels and implement the four components.
- [ ] Re-run focused tests and verify they pass.

### Task 4: Public layout and current homepage migration

**Files:**
- Modify: `web/platform/src/app/(public)/layout.tsx`
- Modify: `web/platform/src/app/(public)/page.tsx`
- Modify: `web/platform/src/app/(public)/page.module.css`
- Modify: `web/platform/src/app/(public)/page.test.tsx`
- Modify: `web/platform/src/app/surface-boundaries.test.ts`

**Interfaces:**
- The public layout gets the Russian public dictionary and wraps children in `PublicShell`.
- The current homepage uses `PageContainer`, `SectionHeading`, `PrimaryButton`, and `ContentCard`.

- [ ] Extend public route tests so they fail until the shell and shared primitives are connected.
- [ ] Run the focused page and boundary tests and verify expected failures.
- [ ] Connect the layout and migrate the current page without adding new homepage sections.
- [ ] Re-run focused tests and verify they pass.

### Task 5: Regression and production verification

**Files:**
- Review only all files changed by Tasks 1–4.

**Interfaces:**
- Produces verification evidence for the chapter definition of done.

- [ ] Run focused public component and route tests.
- [ ] Run `npm test`.
- [ ] Run `npm run typecheck`.
- [ ] Run `npm run lint`.
- [ ] Run `npm run test:packaging`.
- [ ] Run `npm run build`.
- [ ] Inspect `git diff --check`, `git status --short`, and the scoped diff to confirm `/app` was not modified.
