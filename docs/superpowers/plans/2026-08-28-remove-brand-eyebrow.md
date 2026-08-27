# Remove Brand Eyebrow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the standalone blue `NeiroHub` eyebrow from authenticated page headers without changing other branding or layout.

**Architecture:** Remove the two brand-only presentation nodes at their owning feature components and delete only the translation properties those nodes consume. Protect the scope with focused component assertions so normal NeiroHub copy, titles, descriptions, and sidebar branding remain available.

**Tech Stack:** React 19, Next.js 16, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Preserve the sidebar brand, page titles, descriptions, model selector, and unrelated blue section labels.
- Keep `Библиотека`, `Галерея NeiroHub`, and all non-eyebrow occurrences of `NeiroHub` unchanged.
- Do not change CSS, spacing tokens, layout containers, authentication, backend calls, or navigation.
- Do not commit or push until the user explicitly requests it.

---

### Task 1: Remove the standalone brand eyebrows

**Files:**
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ru.modelsCatalog.title`, `ru.modelsCatalog.description`, `ru.workspace.sections`
- Produces: page headers without a standalone `NeiroHub` paragraph; no new public interface

- [ ] **Step 1: Add failing component assertions**

In `ModelsCatalog.test.tsx`, render the catalog with a pending loader and assert:

```tsx
expect(screen.queryByText("NeiroHub", { exact: true, selector: "header > p" })).not.toBeInTheDocument();
expect(screen.getByRole("heading", { name: ru.modelsCatalog.title })).toBeInTheDocument();
```

In `WorkspaceHome.test.tsx`, render `<WorkspaceHome section="models" />` to static markup and assert that it includes the section title but not a paragraph whose complete content is `NeiroHub`.

- [ ] **Step 2: Run focused tests and verify the new assertions fail**

Run:

```bash
npx vitest run src/features/models/ModelsCatalog/ModelsCatalog.test.tsx src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
```

Expected: the new eyebrow-absence assertions fail against the existing `<p className={styles.eyebrow}>NeiroHub</p>` nodes.

- [ ] **Step 3: Remove the two presentation nodes and translation properties**

Delete this line from `ModelsCatalog.tsx`:

```tsx
<p className={styles.eyebrow}>{ru.modelsCatalog.eyebrow}</p>
```

Delete this line from the generic section branch of `WorkspaceHome.tsx`:

```tsx
<p className={styles.eyebrow}>{ru.workspace.eyebrow}</p>
```

Delete only these properties from `ru.ts`:

```ts
eyebrow: "NeiroHub",
```

under `modelsCatalog` and `workspace`.

- [ ] **Step 4: Run focused tests and verify they pass**

Run the same Vitest command. Expected: both files pass, while the catalog and workspace headings remain present.

- [ ] **Step 5: Run project verification**

Run:

```bash
npm run lint
npm run typecheck
npm test
npm run build
git diff --check
```

Expected: all commands exit successfully and the final diff contains no CSS or unrelated branding changes.
