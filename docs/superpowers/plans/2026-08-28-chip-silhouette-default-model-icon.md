# Chip Silhouette Default Model Icon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use the user-supplied `chip_silhouette.svg` as the sole placeholder artwork for every model without its own image.

**Architecture:** Store the supplied SVG unchanged in the platform public model-assets directory. `ModelIcon` will show it as the background of the existing fallback container, while custom model artwork continues through `next/image` and falls back to the same container on error.

**Tech Stack:** React, Next.js, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- The source file is `D:/Downloads/chip_silhouette.svg` with SHA-256 `C6EFB1E68DA70E4C287A64B0CD8B0D8695E440E3AC6F6673A76E13D1BB67C3DB`.
- Preserve the SVG contents, white color, and transparent background exactly.
- Do not change icon dimensions, spacing, borders, radii, card dimensions, or other design.
- Keep custom model artwork behavior unchanged.
- Do not push until the user explicitly requests it.

---

### Task 1: Replace the Placeholder Artwork Source

**Files:**
- Create: `web/platform/public/assets/images/models/chip-silhouette.svg`
- Modify: `web/platform/src/assets/asset-paths.ts`
- Modify: `web/platform/src/features/models/ModelIcon/ModelIcon.tsx`
- Modify: `web/platform/src/features/models/ModelIcon/ModelIcon.module.css`
- Test: `web/platform/src/assets/asset-paths.test.ts`
- Test: `web/platform/src/features/models/ModelIcon/ModelIcon.test.tsx`

**Interfaces:**
- Consumes: `assetPaths.images.models.fallback` and `ModelIcon({ className?: string; src?: string | null })`
- Produces: `/assets/images/models/chip-silhouette.svg` as the only default model artwork, with custom `src` retaining priority

- [x] **Step 1: Write failing tests for the supplied placeholder**

Add this asset-path test:

```tsx
it("exposes the supplied model placeholder URL", () => {
  expect(assetPaths.images.models.fallback).toBe(
    "/assets/images/models/chip-silhouette.svg",
  );
});
```

Replace the default-artwork assertion in `ModelIcon.test.tsx` with:

```tsx
it("uses the supplied chip silhouette as the default artwork", () => {
  render(<ModelIcon />);

  const fallback = screen.getByTestId("model-icon-fallback");

  expect(fallback).toHaveStyle({
    backgroundImage: 'url("/assets/images/models/chip-silhouette.svg")',
  });
  expect(fallback.querySelector("svg")).not.toBeInTheDocument();
  expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
});
```

Extend the failed-custom-artwork test with the same `backgroundImage` assertion.

- [x] **Step 2: Run the focused tests and confirm the expected failure**

Run from `web/platform`:

```powershell
npm exec vitest -- run src/assets/asset-paths.test.ts src/features/models/ModelIcon/ModelIcon.test.tsx
```

Expected: FAIL because the path still names `default-model-87465de8.png`, the fallback has no background image, and the old inline SVG remains.

- [x] **Step 3: Add the exact supplied SVG asset**

Use `apply_patch` to create `web/platform/public/assets/images/models/chip-silhouette.svg` with the complete, unchanged text from `D:/Downloads/chip_silhouette.svg`. Verify the copy immediately:

```powershell
Get-FileHash -Algorithm SHA256 D:/Downloads/chip_silhouette.svg
Get-FileHash -Algorithm SHA256 web/platform/public/assets/images/models/chip-silhouette.svg
```

Expected: both hashes equal `C6EFB1E68DA70E4C287A64B0CD8B0D8695E440E3AC6F6673A76E13D1BB67C3DB`.

- [x] **Step 4: Point the shared asset registry at the supplied file**

Change the model fallback entry to:

```ts
models: {
  fallback: "/assets/images/models/chip-silhouette.svg",
},
```

- [x] **Step 5: Replace the hand-written inline SVG with the exact asset**

Import `assetPaths` and reduce `DefaultModelArtwork` to the existing container with a background image:

```tsx
import { assetPaths } from "@/assets/asset-paths";

function DefaultModelArtwork({ classNames }: Readonly<{ classNames: string }>) {
  return (
    <span
      aria-hidden="true"
      className={`${classNames} ${styles.fallback}`}
      data-testid="model-icon-fallback"
      style={{ backgroundImage: `url("${assetPaths.images.models.fallback}")` }}
    />
  );
}
```

Keep the remaining `ModelIcon` selection and custom-image error behavior unchanged.

- [x] **Step 6: Keep the same box dimensions and render the asset without a frame**

Replace only the obsolete inline-SVG rules with background positioning:

```css
.fallback {
  background-position: center;
  background-repeat: no-repeat;
  background-size: contain;
}
```

Delete `.fallback svg`, `.faceCutout`, and `.faceStroke`. Do not alter `.icon` or its responsive rule.

- [x] **Step 7: Run focused component and consumer tests**

Run from `web/platform`:

```powershell
npm exec vitest -- run src/assets/asset-paths.test.ts src/features/models/ModelIcon/ModelIcon.test.tsx src/features/models/ModelCard/ModelCard.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx
```

Expected: all tests pass; default consumers use `model-icon-fallback`, custom artwork still uses `model-icon`, and failed custom artwork returns to the supplied placeholder.

- [x] **Step 8: Run complete verification**

Run from `web/platform`:

```powershell
npm exec vitest -- run --maxWorkers=4
npm run test:assets
npm run lint
npm run typecheck
npm run build
npm run test:packaging
```

Run from the worktree root:

```powershell
git diff --check
git status --short --branch
```

Expected: 0 failures, matching SVG hashes, clean diff formatting, and only the planned asset/component/test/documentation changes.

- [x] **Step 9: Commit locally**

```powershell
git add docs/superpowers/plans/2026-08-28-chip-silhouette-default-model-icon.md web/platform/public/assets/images/models/chip-silhouette.svg web/platform/src/assets/asset-paths.ts web/platform/src/assets/asset-paths.test.ts web/platform/src/features/models/ModelIcon/ModelIcon.tsx web/platform/src/features/models/ModelIcon/ModelIcon.module.css web/platform/src/features/models/ModelIcon/ModelIcon.test.tsx
git commit -m "fix(platform): use supplied default model icon"
```

Expected: a local commit is created and no push occurs.
