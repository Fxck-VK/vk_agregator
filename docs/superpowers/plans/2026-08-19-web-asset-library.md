# NeiroHub Web Asset Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a structured hybrid asset library, migrate the current inspiration image safely, extract reusable SVG icons, and enforce the conventions automatically.

**Architecture:** Shared public media lives below `public/assets` and is referenced through a typed path catalog. Theme-aware interactive SVGs are focused React components below `src/components/icons`; feature-only media may remain in feature-local `assets` directories. A dependency-free Node validator protects naming, extensions, duplicate normalized paths and SVG safety without loading assets into the client bundle.

**Tech Stack:** Next.js 16.2, React 19, TypeScript 5.9, Vitest 4, Node.js 22, `next/image`, CSS Modules.

## Global Constraints

- Do not change current UI appearance or `/app` behavior.
- Do not move user uploads, generated artifacts, private URLs, secrets or PII into frontend assets.
- Use lowercase kebab-case for asset filenames.
- Use SVG for trusted vector artwork, AVIF/WebP for photos where practical, and PNG only when its characteristics are required.
- Theme-aware SVG icons use `currentColor` and remain accessible through their consumer.
- Do not create a global barrel that eagerly imports all media.
- Public asset paths are type-safe string constants; feature-local assets may use direct static imports.
- Public-path `next/image` usages keep intrinsic dimensions or `fill`, responsive `sizes`, and default lazy loading unless intentionally priority content.
- SVG optimization remains disabled; do not add `dangerouslyAllowSVG`.
- Preserve unrelated dirty worktree changes, especially `ChatMediaMenu` and composer work.

---

## File Map

- Create `web/platform/src/assets/asset-paths.ts` and its test for stable shared URLs.
- Create `web/platform/public/assets/README.md` for ownership and format rules.
- Create tracked category roots below `public/assets` with `.gitkeep` only where no real asset exists yet.
- Move `web/platform/public/inspiration/paper-crane-cloud.png` to `web/platform/public/assets/images/inspiration/paper-crane-cloud.png`.
- Modify `InspirationGallery` and `WorkspaceLanding` plus their relevant tests to use the catalog.
- Create `web/platform/src/components/icons/IconProps.ts`, `CopyIcon`, `CheckIcon`, direct entry points and tests.
- Modify `ConversationMessageActions.tsx` to consume the shared copy/check icons.
- Create `web/platform/scripts/validate-assets.mjs` and `validate-assets.test.mjs`.
- Modify `web/platform/package.json` and `docs/INDEX.md` to wire validation and documentation.

---

### Task 1: Typed catalog and atomic image migration

**Files:**
- Create: `web/platform/src/assets/asset-paths.test.ts`
- Create: `web/platform/src/assets/asset-paths.ts`
- Create: `web/platform/public/assets/README.md`
- Create: `web/platform/public/assets/brand/logos/.gitkeep`
- Create: `web/platform/public/assets/brand/marks/.gitkeep`
- Create: `web/platform/public/assets/icons/models/.gitkeep`
- Create: `web/platform/public/assets/illustrations/empty-states/.gitkeep`
- Create: `web/platform/public/assets/illustrations/onboarding/.gitkeep`
- Create: `web/platform/public/assets/images/models/.gitkeep`
- Create: `web/platform/public/assets/images/tools/.gitkeep`
- Create: `web/platform/public/assets/images/articles/.gitkeep`
- Move: `web/platform/public/inspiration/paper-crane-cloud.png` -> `web/platform/public/assets/images/inspiration/paper-crane-cloud.png`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

**Interfaces:**
- Produces: `assetPaths.images.inspiration.paperCraneCloud: "/assets/images/inspiration/paper-crane-cloud.png"`.
- Consumes: Next.js public-folder root URLs and existing `next/image` rendering.

- [ ] **Step 1: Write the failing catalog test**

```ts
import { describe, expect, it } from "vitest";

import { assetPaths } from "./asset-paths";

describe("assetPaths", () => {
  it("exposes a stable inspiration image URL without eager imports", () => {
    expect(assetPaths.images.inspiration.paperCraneCloud).toBe(
      "/assets/images/inspiration/paper-crane-cloud.png",
    );
  });
});
```

- [ ] **Step 2: Verify the test fails for the missing module**

Run from `web/platform`:

```powershell
npm exec vitest -- run src/assets/asset-paths.test.ts
```

Expected: FAIL because `./asset-paths` does not exist.

- [ ] **Step 3: Create the minimal typed catalog**

```ts
export const assetPaths = {
  images: {
    inspiration: {
      paperCraneCloud: "/assets/images/inspiration/paper-crane-cloud.png",
    },
  },
} as const;
```

- [ ] **Step 4: Move the current image into the shared library**

Run from `web/platform`:

```powershell
@(
  "public\assets\brand\logos",
  "public\assets\brand\marks",
  "public\assets\icons\models",
  "public\assets\illustrations\empty-states",
  "public\assets\illustrations\onboarding",
  "public\assets\images\articles",
  "public\assets\images\inspiration",
  "public\assets\images\models",
  "public\assets\images\tools"
) | ForEach-Object { New-Item -ItemType Directory -Force -Path $_ }
Move-Item -LiteralPath public\inspiration\paper-crane-cloud.png -Destination public\assets\images\inspiration\paper-crane-cloud.png
Test-Path public\assets\images\inspiration\paper-crane-cloud.png
Test-Path public\inspiration\paper-crane-cloud.png
```

Expected: the final two commands print `True`, then `False`.

Track each empty category with a `.gitkeep` created through `apply_patch`; do
not place `.gitkeep` in `images/inspiration` because it contains a real asset.

- [ ] **Step 5: Document the library at `public/assets/README.md`**

```markdown
# NeiroHub static assets

This directory contains trusted, repository-owned static media with stable
public URLs.

- `brand/`: product logos and marks shared by multiple surfaces.
- `icons/models/`: static model marks that are not interactive React icons.
- `illustrations/`: shared empty-state and onboarding artwork.
- `images/`: editorial images grouped by inspiration, models, tools and articles.

Use lowercase kebab-case names. Use SVG for trusted vector artwork, AVIF/WebP
for photos where practical, and PNG only when its transparency or compatibility
is required. Interactive theme-aware icons belong in `src/components/icons`.
Feature-specific files with one owner may live beside their feature in an
`assets` directory.

Never place user uploads, generated artifacts, private storage/provider URLs,
secrets or PII here.
```

- [ ] **Step 6: Switch both consumers to the typed URL**

Add to `InspirationGallery.tsx` and `WorkspaceLanding.tsx`:

```ts
import { assetPaths } from "@/assets/asset-paths";
```

Use this constant in `InspirationGallery.tsx`:

```ts
const imagePath = assetPaths.images.inspiration.paperCraneCloud;
```

Replace both old string paths in `WorkspaceLanding.tsx` with:

```tsx
src={assetPaths.images.inspiration.paperCraneCloud}
```

- [ ] **Step 7: Update URL assertions**

Use in `InspirationGallery.test.tsx`:

```ts
expect(download).toHaveAttribute(
  "href",
  "/assets/images/inspiration/paper-crane-cloud.png",
);
```

Use in `WorkspaceHome.test.tsx`:

```ts
expect(markup).toContain(
  "%2Fassets%2Fimages%2Finspiration%2Fpaper-crane-cloud.png",
);
```

- [ ] **Step 8: Run focused tests and typecheck**

```powershell
npm exec vitest -- run src/assets/asset-paths.test.ts src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
npm run typecheck
```

Expected: all tests PASS and TypeScript exits `0`.

- [ ] **Step 9: Commit only Task 1 files**

```powershell
git add -- web/platform/src/assets web/platform/public/assets web/platform/public/inspiration/paper-crane-cloud.png web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
git commit -m "refactor(web): add typed asset library"
```

### Task 2: Shared theme-aware SVG icons

**Files:**
- Create: `web/platform/src/components/icons/IconProps.ts`
- Create: `web/platform/src/components/icons/CopyIcon/CopyIcon.tsx`
- Create: `web/platform/src/components/icons/CopyIcon/index.ts`
- Create: `web/platform/src/components/icons/CheckIcon/CheckIcon.tsx`
- Create: `web/platform/src/components/icons/CheckIcon/index.ts`
- Create: `web/platform/src/components/icons/icons.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx`

**Interfaces:**
- Produces: `IconProps`, `CopyIcon(props: IconProps)`, `CheckIcon(props: IconProps)`.
- Consumes: standard React SVG properties and accessible labels supplied by the containing buttons.

- [ ] **Step 1: Write failing icon contract tests**

```tsx
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { CheckIcon } from "./CheckIcon";
import { CopyIcon } from "./CopyIcon";

describe("shared icons", () => {
  it("renders the copy glyph as a decorative currentColor SVG", () => {
    const markup = renderToStaticMarkup(<CopyIcon className="action-icon" />);
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).toContain('class="action-icon"');
    expect(markup).toContain('data-icon="copy"');
    expect(markup).toContain('stroke="currentColor"');
  });

  it("renders the copied-state check glyph with inherited color", () => {
    const markup = renderToStaticMarkup(<CheckIcon />);
    expect(markup).toContain('data-icon="check"');
    expect(markup).toContain('stroke="currentColor"');
  });
});
```

- [ ] **Step 2: Verify the tests fail for missing modules**

```powershell
npm exec vitest -- run src/components/icons/icons.test.tsx
```

Expected: FAIL because `./CopyIcon` and `./CheckIcon` do not exist.

- [ ] **Step 3: Add the common prop type**

```ts
import type { ComponentProps } from "react";

export type IconProps = Omit<ComponentProps<"svg">, "children">;
```

- [ ] **Step 4: Implement `CopyIcon` and its direct entry point**

```tsx
import type { IconProps } from "../IconProps";

export function CopyIcon(props: Readonly<IconProps>) {
  return (
    <svg aria-hidden="true" data-icon="copy" fill="none" focusable="false" viewBox="0 0 24 24" {...props}>
      <rect height="13" rx="2.5" stroke="currentColor" strokeWidth="1.8" width="11" x="8" y="8" />
      <path d="M16 8V6.5A2.5 2.5 0 0 0 13.5 4h-7A2.5 2.5 0 0 0 4 6.5v7A2.5 2.5 0 0 0 6.5 16H8" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  );
}
```

```ts
export { CopyIcon } from "./CopyIcon";
```

- [ ] **Step 5: Implement `CheckIcon` and its direct entry point**

```tsx
import type { IconProps } from "../IconProps";

export function CheckIcon(props: Readonly<IconProps>) {
  return (
    <svg aria-hidden="true" data-icon="check" fill="none" focusable="false" viewBox="0 0 24 24" {...props}>
      <path d="m5 12.5 4.25 4.25L19 7" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
    </svg>
  );
}
```

```ts
export { CheckIcon } from "./CheckIcon";
```

- [ ] **Step 6: Consume the shared icons without changing behavior**

Add to `ConversationMessageActions.tsx`:

```ts
import { CheckIcon } from "@/components/icons/CheckIcon";
import { CopyIcon } from "@/components/icons/CopyIcon";
```

Delete only the local `CopyIcon` and `CheckIcon` functions. Leave
`RecreateIcon`, `LikeIcon` and `DislikeIcon` unchanged.

- [ ] **Step 7: Run icon and message-action tests**

```powershell
npm exec vitest -- run src/components/icons/icons.test.tsx src/features/conversations/ConversationMessageActions/ConversationMessageActions.test.tsx
npm run typecheck
```

Expected: tests PASS, TypeScript exits `0`, and SVG paths and button names remain unchanged.

- [ ] **Step 8: Commit Task 2 files**

```powershell
git add -- web/platform/src/components/icons web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx
git commit -m "refactor(web): extract shared action icons"
```

### Task 3: Automated asset validation

**Files:**
- Create: `web/platform/scripts/validate-assets.mjs`
- Create: `web/platform/scripts/validate-assets.test.mjs`
- Modify: `web/platform/package.json`
- Modify: `docs/INDEX.md`

**Interfaces:**
- Produces: `inspectAssetEntries(entries): string[]`, `validateAssetLibrary(projectRoot): Promise<string[]>`, CLI exit `0` for valid assets and `1` for violations.
- Consumes: `public/assets/**`, feature-local `src/features/**/assets/**`, Node built-ins only.

- [ ] **Step 1: Write validator unit tests**

```js
import assert from "node:assert/strict";
import test from "node:test";

import { inspectAssetEntries } from "./validate-assets.mjs";

test("accepts named raster assets and safe SVG", () => {
  assert.deepEqual(inspectAssetEntries([
    { relativePath: "public/assets/images/example-card.webp" },
    { relativePath: "public/assets/icons/models/model-mark.svg", content: '<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0" /></svg>' },
  ]), []);
});

test("rejects invalid names, extensions, duplicates and unsafe SVG", () => {
  const errors = inspectAssetEntries([
    { relativePath: "public/assets/images/Icon 1.png" },
    { relativePath: "public/assets/images/photo.jpg" },
    { relativePath: "public/assets/images/card.webp" },
    { relativePath: "PUBLIC/ASSETS/IMAGES/CARD.WEBP" },
    { relativePath: "public/assets/icons/models/unsafe.svg", content: '<svg onload="alert(1)"><script>alert(1)</script></svg>' },
  ]);

  assert.ok(errors.some((error) => error.includes("kebab-case")));
  assert.ok(errors.some((error) => error.includes("unsupported extension")));
  assert.ok(errors.some((error) => error.includes("duplicate normalized path")));
  assert.ok(errors.some((error) => error.includes("unsafe SVG")));
});
```

- [ ] **Step 2: Verify the test fails for the missing validator**

```powershell
node --test scripts/validate-assets.test.mjs
```

Expected: FAIL because `validate-assets.mjs` does not exist.

- [ ] **Step 3: Implement record validation and discovery**

Create `scripts/validate-assets.mjs`:

```js
import { access, readdir, readFile } from "node:fs/promises";
import { dirname, extname, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const allowedExtensions = new Set([".avif", ".png", ".svg", ".webp"]);
const allowedMetadataNames = new Set([".gitkeep", "README.md"]);
const kebabCaseAssetName = /^[a-z0-9]+(?:-[a-z0-9]+)*\.(?:avif|png|svg|webp)$/;
const unsafeSvgPatterns = [
  /<script\b/i,
  /<foreignObject\b/i,
  /\son[a-z]+\s*=/i,
  /(?:href|xlink:href)\s*=\s*["']\s*(?:https?:|\/\/|javascript:|data:text\/html)/i,
];

function normalizePath(value) {
  return value.split("\\").join("/");
}

export function inspectAssetEntries(entries) {
  const errors = [];
  const seenPaths = new Map();

  for (const entry of [...entries].sort((left, right) =>
    left.relativePath.localeCompare(right.relativePath))) {
    const relativePath = normalizePath(entry.relativePath);
    const normalizedPath = relativePath.toLowerCase();
    const previousPath = seenPaths.get(normalizedPath);
    if (previousPath) {
      errors.push(`${relativePath}: duplicate normalized path of ${previousPath}`);
    } else {
      seenPaths.set(normalizedPath, relativePath);
    }

    const fileName = relativePath.split("/").at(-1) ?? relativePath;
    if (allowedMetadataNames.has(fileName)) continue;

    const extension = extname(fileName).toLowerCase();
    if (!allowedExtensions.has(extension)) {
      errors.push(`${relativePath}: unsupported extension ${extension || "<none>"}`);
      continue;
    }
    if (!kebabCaseAssetName.test(fileName)) {
      errors.push(`${relativePath}: asset filename must use lowercase kebab-case`);
    }
    if (
      extension === ".svg" &&
      unsafeSvgPatterns.some((pattern) => pattern.test(entry.content ?? ""))
    ) {
      errors.push(`${relativePath}: unsafe SVG content`);
    }
  }

  return errors;
}

async function collectFiles(root, prefix, include) {
  const records = [];

  async function visit(directory) {
    const entries = await readdir(directory, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
      const absolutePath = resolve(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(absolutePath);
        continue;
      }
      if (!entry.isFile() || !include(absolutePath)) continue;

      const relativePath = normalizePath(relative(root, absolutePath));
      const extension = extname(entry.name).toLowerCase();
      records.push({
        relativePath: `${prefix}/${relativePath}`,
        content: extension === ".svg" ? await readFile(absolutePath, "utf8") : undefined,
      });
    }
  }

  await visit(root);
  return records;
}

export async function validateAssetLibrary(projectRoot) {
  const publicRoot = resolve(projectRoot, "public", "assets");
  try {
    await access(publicRoot);
  } catch {
    return ["public/assets: missing required asset root"];
  }

  const publicRecords = await collectFiles(publicRoot, "public/assets", () => true);
  const featuresRoot = resolve(projectRoot, "src", "features");
  const featureRecords = await collectFiles(featuresRoot, "src/features", (absolutePath) => {
    const relativePath = relative(featuresRoot, absolutePath).split(sep);
    return relativePath.includes("assets");
  });
  return inspectAssetEntries([...publicRecords, ...featureRecords]);
}

const scriptPath = fileURLToPath(import.meta.url);
const isCli = process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url;
if (isCli) {
  const projectRoot = resolve(dirname(scriptPath), "..");
  const errors = await validateAssetLibrary(projectRoot);
  if (errors.length > 0) {
    for (const error of errors) console.error(error);
    process.exitCode = 1;
  } else {
    console.log("Asset validation passed.");
  }
}
```

- [ ] **Step 4: Run validator tests and live validation**

```powershell
node --test scripts/validate-assets.test.mjs
node scripts/validate-assets.mjs
```

Expected: both exit `0`; the second prints `Asset validation passed.`.

- [ ] **Step 5: Wire validation into `package.json`**

Keep unrelated scripts and set:

```json
{
  "test": "vitest run && npm run test:assets",
  "test:assets": "node --test scripts/validate-assets.test.mjs",
  "validate:assets": "node scripts/validate-assets.mjs"
}
```

- [ ] **Step 6: Link this plan from `docs/INDEX.md`**

```markdown
| NeiroHub web static assets, reusable SVG icons and feature-local media | approved design: `docs/superpowers/specs/2026-08-19-web-asset-library-design.md`; implementation plan: `docs/superpowers/plans/2026-08-19-web-asset-library.md` |
```

- [ ] **Step 7: Run full frontend verification**

Run from `web/platform`:

```powershell
npm run validate:assets
npm run lint
npm run typecheck
npm test
npm run build
```

Expected: every command exits `0`; build has no missing image and the Next.js
configuration does not enable `dangerouslyAllowSVG`.

- [ ] **Step 8: Review scope and status**

```powershell
git diff --check
git status --short
git diff -- web/platform/src/assets web/platform/public/assets web/platform/src/components/icons web/platform/scripts web/platform/package.json web/platform/src/features/inspiration/InspirationGallery web/platform/src/features/workspace/WorkspaceLanding web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx docs/INDEX.md docs/superpowers/plans/2026-08-19-web-asset-library.md
```

Expected: asset-library changes plus already-existing unrelated dirty files; no unrelated file staged.

- [ ] **Step 9: Commit Task 3 files**

```powershell
git add -- web/platform/scripts/validate-assets.mjs web/platform/scripts/validate-assets.test.mjs web/platform/package.json docs/INDEX.md docs/superpowers/plans/2026-08-19-web-asset-library.md
git commit -m "chore(web): validate static assets"
```

## Rollback

Revert the three asset-library commits in reverse order. Task 1 is atomic: its
revert restores the old physical image location and all consumer URLs. No
backend state, database row, user upload or generated artifact changes.
