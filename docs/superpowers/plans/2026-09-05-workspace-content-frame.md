# Workspace Content Frame Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the home, files, models, and inspiration pages the same centered `46rem` content boundaries and responsive side gutters.

**Architecture:** Global CSS tokens define the approved `66rem` outer shell, `46rem` content width, and gutter. A focused `WorkspacePageFrame` layout component reproduces the landing page's two-level geometry and intermediate-width alignment, while the home page consumes the same tokens for its existing frame.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest.

## Global Constraints

- Desktop content width is exactly `46rem`.
- Desktop outer shell width is exactly `66rem`.
- Mobile side gutter below `48rem` is exactly `var(--space-4)`.
- Chat and image-generation workspaces remain unchanged.

---

### Task 1: Add the shared workspace page frame

**Files:**
- Create: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.tsx`
- Create: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.module.css`
- Create: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`
- Modify: `web/platform/src/app/globals.css`

**Interfaces:**
- Consumes: global `--workspace-page-shell-width`, `--workspace-content-frame-width`, and `--workspace-page-inline-gutter` tokens.
- Produces: `WorkspacePageFrame(props: HTMLAttributes<HTMLDivElement>)`.

- [x] **Step 1: Write the failing stylesheet and consumer contract tests.**

```ts
expect(globals).toContain("--workspace-content-frame-width: 46rem");
expect(frameStyles).toContain("padding-inline: var(--workspace-page-inline-gutter)");
expect(frameStyles).toContain("inline-size: min(100%, var(--workspace-page-shell-width))");
```

- [x] **Step 2: Run `npm exec vitest run src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts` and confirm failure because the shared frame is absent.**
- [x] **Step 3: Add the global tokens and minimal frame component.**

```tsx
export function WorkspacePageFrame({ children, className, ...props }: WorkspacePageFrameProps) {
  return (
    <div className={[styles.frame, className].filter(Boolean).join(" ")} {...props}>
      <div className={styles.content}>{children}</div>
    </div>
  );
}
```

```css
.frame {
  inline-size: min(100%, var(--workspace-page-shell-width));
  margin-inline: auto;
  padding-inline: var(--workspace-page-inline-gutter);
}

.content {
  inline-size: min(100%, var(--workspace-content-frame-width));
  margin-inline: auto;
}
```

- [x] **Step 4: Re-run the focused test and confirm it passes.**

### Task 2: Migrate standard workspace pages

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.module.css`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.module.css`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`

**Interfaces:**
- Consumes: `WorkspacePageFrame` and the global frame tokens.
- Produces: identical content boundaries on the four approved pages.

- [x] **Step 1: Extend the failing contract to require all three standard pages to use `WorkspacePageFrame` and the home page to consume the shared tokens.**

```ts
expect(filesSource).toContain("<WorkspacePageFrame>");
expect(modelsSource).toContain("<WorkspacePageFrame>");
expect(inspirationSource).toContain("<WorkspacePageFrame>");
expect(homeStyles).toContain("inline-size: min(100%, var(--workspace-content-frame-width))");
```

- [x] **Step 2: Run the focused contract and confirm the missing integrations fail.**
- [x] **Step 3: Wrap the three page sections, remove their local horizontal widths/padding, and map the home styles to the shared tokens.**

```tsx
<WorkspacePageFrame>
  <section aria-labelledby="inspiration-title" className={styles.gallery}>
    <h1 id="inspiration-title">{ru.inspiration.title}</h1>
  </section>
</WorkspacePageFrame>
```

```css
.catalog {
  display: grid;
  inline-size: 100%;
  padding-block: var(--space-8);
}
```

- [x] **Step 4: Run focused component tests, TypeScript, ESLint, and visual browser checks.**
