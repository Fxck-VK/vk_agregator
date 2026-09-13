# Right Panel Model Selector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-width, outlined, non-truncating model-selector trigger to every file-preview right panel without changing the workspace-header trigger.

**Architecture:** Introduce a typed `panel` presentation variant on the existing shared `ModelSelector`, expose it as a root data attribute, and keep all visual overrides in the shared CSS module. Select that variant only from `FileTaskModelSelector`, which already adapts all four right-panel tasks to the shared selector.

**Tech Stack:** React 19, TypeScript 5.9, CSS Modules, Vitest, Testing Library

## Global Constraints

- Preserve the default compact selector used by the workspace header.
- Apply the panel variant to Animate, Enhance, Remove background, and Edit through their shared adapter.
- Use `rgb(255 255 255 / 18%)` for the neutral outline and `var(--radius-sm)` for `0.5rem` corners.
- Use a transparent trigger background so the right panel remains the single source of its interior color.
- Wrap long model names instead of clipping them or replacing text with an ellipsis.
- Keep hover visually static while preserving keyboard-focus visibility.

---

### Task 1: Add and adopt the panel selector variant

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes: the existing `ModelSelector` trigger, `FileTaskModelSelector`, `--color-accent`, and `--radius-sm`
- Produces: `ModelSelectorVariant = "compact" | "panel"` and an optional `variant` prop that defaults to `compact`

- [x] **Step 1: Write failing component and stylesheet contracts**

```tsx
render(<ModelSelector models={taskModels} onSelect={onSelect} selectedModelId="video-generator" variant="panel" />);
expect(screen.getByRole("button", { name: /Генератор видео/ }).parentElement).toHaveAttribute(
  "data-variant",
  "panel",
);
```

```ts
expect(panelTriggerRule).toContain("inline-size: 100%");
expect(panelTriggerRule).toContain("max-inline-size: none");
expect(panelTriggerRule).toContain("border-color: rgb(255 255 255 / 18%)");
expect(panelTriggerRule).toContain("border-radius: var(--radius-sm)");
expect(panelTriggerRule).toContain("background: transparent");
expect(panelTextRule).toContain("overflow: visible");
expect(panelTextRule).toContain("text-overflow: clip");
expect(panelTextRule).toContain("white-space: normal");
expect(panelTextRule).toContain("overflow-wrap: anywhere");
expect(panelHoverRule).toContain("border-color: rgb(255 255 255 / 18%)");
expect(panelHoverRule).toContain("background: transparent");
expect(panelHoverRule).toContain("box-shadow: none");
```

- [x] **Step 2: Run the focused tests and verify RED**

Run: `npm exec vitest run -- src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts -t "panel variant"`

Expected: FAIL because `variant` and the panel-specific CSS rules do not exist.

- [x] **Step 3: Implement the typed variant and right-panel styling**

```tsx
export type ModelSelectorVariant = "compact" | "panel";

type ModelSelectorProps = {
  className?: string;
  dialogId?: string;
  dialogLabel?: string;
  models: readonly ModelSelectorModel[];
  onSelect: (model: ModelSelectorModel) => void;
  renderInPortal?: boolean;
  selectedModelId: string;
  status?: ModelSelectorStatus;
  triggerAriaLabel?: (name: string, isOpen: boolean) => string;
  variant?: ModelSelectorVariant;
};

// Destructure `variant = "compact"` with the existing props and add the data attribute
// to the existing root element:
<div className={[styles.root, className].filter(Boolean).join(" ")} data-variant={variant} ref={rootRef}>
```

```css
.root[data-variant="panel"] {
  inline-size: 100%;
}

.root[data-variant="panel"] .trigger {
  inline-size: 100%;
  max-inline-size: none;
  min-block-size: 3.5rem;
  border-color: rgb(255 255 255 / 18%);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  background: transparent;
}

.root[data-variant="panel"] .triggerText {
  min-inline-size: 0;
  overflow: visible;
  overflow-wrap: anywhere;
  text-overflow: clip;
  white-space: normal;
}

.root[data-variant="panel"] .trigger:hover {
  border-color: rgb(255 255 255 / 18%);
  background: transparent;
  box-shadow: none;
}
```

Pass `variant="panel"` from `FileTaskModelSelector` and leave `WorkspaceModelSelector` on the default.

- [x] **Step 4: Run focused verification and verify GREEN**

Run: `npm exec vitest run -- src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: all focused tests pass with zero failures.

- [x] **Step 5: Run static verification**

Run: `npm run typecheck`

Expected: exit code `0`.

## Verification Results

- Initial RED: two variant-contract tests failed because the component and CSS had no panel variant; all four file-task adoption cases failed because the adapter did not request it.
- Hover RED: the style contract failed on the previous accent border and missing static-hover rule.
- GREEN: 80 of 80 related model-selector and file-preview tests passed.
- TypeScript and ESLint exited successfully.

Run: `npm run lint`

Expected: exit code `0`.
