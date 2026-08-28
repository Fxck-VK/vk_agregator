# Inline Default Model Artwork Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the shared default model artwork without a network request so a broken-image marker cannot appear when no model-specific artwork exists.

**Architecture:** `ModelIcon` will render its existing inline smiling-chip SVG immediately when `src` is absent. A supplied custom `src` will continue to use `next/image`, and an image load error will replace it with the same inline SVG.

**Tech Stack:** React, Next.js `Image`, TypeScript, Vitest, Testing Library, CSS Modules

## Global Constraints

- Do not change icon dimensions, spacing, card dimensions, borders, radii, or other visual styling.
- Do not add a new asset or dependency.
- Preserve support for model-specific artwork supplied through `src`.
- Do not push this change until the user explicitly requests a push.

---

### Task 1: Make Default Artwork Independent of File Delivery

**Files:**
- Modify: `web/platform/src/features/models/ModelIcon/ModelIcon.tsx`
- Test: `web/platform/src/features/models/ModelIcon/ModelIcon.test.tsx`
- Test: `web/platform/src/features/models/ModelCard/ModelCard.test.tsx`
- Test: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx`
- Test: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`

**Interfaces:**
- Consumes: `ModelIcon({ className?: string; src?: string | null })`
- Produces: an inline default artwork for absent or failed `src`; a `next/image` element only for a valid custom `src`

- [x] **Step 1: Write failing component and consumer tests**

Update the assertions so a default render expects `data-testid="model-icon-fallback"` and no `data-testid="model-icon"`. Add a custom-source case that expects the custom image, and change the error case to fire on that custom image before expecting the inline fallback.

```tsx
it("renders the embedded default artwork without requesting an image", () => {
  render(<ModelIcon />);

  expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
  expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
});

it("renders supplied model artwork", () => {
  render(<ModelIcon src="/assets/images/models/custom.png" />);

  expect(screen.getByTestId("model-icon")).toHaveAttribute(
    "src",
    expect.stringContaining("custom.png"),
  );
});

it("replaces failed supplied artwork with the embedded default artwork", () => {
  render(<ModelIcon src="/assets/images/models/missing.png" />);
  fireEvent.error(screen.getByTestId("model-icon"));

  expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
  expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
});
```

- [x] **Step 2: Run focused tests and verify the new default assertions fail**

Run from `web/platform`:

```powershell
npm exec vitest -- run src/features/models/ModelIcon/ModelIcon.test.tsx src/features/models/ModelCard/ModelCard.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx
```

Expected: FAIL because the current default path still renders an `<img>` for `default-model-87465de8.png`.

- [x] **Step 3: Implement immediate inline default artwork**

Extract the existing SVG branch into a local `DefaultModelArtwork` component and select it before rendering `Image`:

```tsx
function DefaultModelArtwork({ classNames }: Readonly<{ classNames: string }>) {
  return (
    <span aria-hidden="true" className={`${classNames} ${styles.fallback}`} data-testid="model-icon-fallback">
      <svg fill="none" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">
        <path
          d="M23 5v8M32 5v8M41 5v8M23 51v8M32 51v8M41 51v8M5 23h8M5 32h8M5 41h8M51 23h8M51 32h8M51 41h8"
          stroke="currentColor"
          strokeLinecap="round"
          strokeWidth="6"
        />
        <rect fill="currentColor" height="40" rx="10" width="40" x="12" y="12" />
        <circle className={styles.faceCutout} cx="25" cy="29" r="3" />
        <circle className={styles.faceCutout} cx="39" cy="29" r="3" />
        <path
          className={styles.faceStroke}
          d="M23 39c2.5 3 5.5 4.5 9 4.5s6.5-1.5 9-4.5"
          fill="none"
          strokeLinecap="round"
          strokeWidth="3.5"
        />
      </svg>
    </span>
  );
}

export function ModelIcon({ className, src }: Readonly<ModelIconProps>) {
  const classNames = [styles.icon, className].filter(Boolean).join(" ");
  const [failedSource, setFailedSource] = useState<string | null>(null);

  if (!src || failedSource === src) {
    return <DefaultModelArtwork classNames={classNames} />;
  }

  return (
    <Image
      alt=""
      aria-hidden="true"
      className={classNames}
      data-testid="model-icon"
      height={245}
      onError={() => setFailedSource(src)}
      src={src}
      unoptimized
      width={205}
    />
  );
}
```

Remove the now-unused `assetPaths` import. Do not modify `ModelIcon.module.css` or the existing SVG paths.

- [x] **Step 4: Run the focused tests and verify they pass**

Run the same focused Vitest command from Step 2.

Expected: all four test files pass; default consumers find inline artwork and the custom-source test still finds an image.

- [x] **Step 5: Run the complete platform verification**

Run from `web/platform`:

```powershell
npm test
npm run lint
npm run typecheck
npm run build
npm run test:packaging
```

Then run from the worktree root:

```powershell
git diff --check
git status --short --branch
```

Expected: all commands pass; the diff contains only the planned component, tests, and documentation changes.

- [x] **Step 6: Commit locally**

```powershell
git add docs/superpowers/plans/2026-08-28-inline-default-model-artwork.md web/platform/src/features/models/ModelIcon/ModelIcon.tsx web/platform/src/features/models/ModelIcon/ModelIcon.test.tsx web/platform/src/features/models/ModelCard/ModelCard.test.tsx web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx
git commit -m "fix(platform): render default model artwork inline"
```

Expected: a local commit is created and the branch remains unpushed.
