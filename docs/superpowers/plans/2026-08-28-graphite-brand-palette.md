# Graphite Brand Palette Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the platform's blue-led palette and lavender outer canvas with the approved graphite hierarchy and restrained violet/blue/pink brand accents.

**Architecture:** Define every dark, light, and system-light color at the global theme boundary, then map shell, panel, card, elevated, and brand-accent roles explicitly in component CSS. Existing compatibility aliases remain so the change is incremental, while contract tests prevent the old canvas and generic SaaS-blue tokens from returning.

**Tech Stack:** Next.js 16, React 19, CSS Modules, CSS custom properties, Vitest.

## Global Constraints

- Dark palette: `#0C0C0F`, `#111217`, `#15161C`, `#1A1B22`, `#20212A`, `#2A2B35`, `#F5F5F7`, `#9B9DA8`, `#9A7CF5`, `#7C8FF7`, `#F09AF0`, `#A9CFFF`.
- Light palette: `#F7F7FA`, `#FFFFFF`, `#F3F3F7`, `#EEEEF4`, `#E8E8F0`, `#DEDFE7`, `#17171B`, `#6B6C76`, `#7563E6`, `#6678E6`, `#D56DD9`, `#8475F0`.
- Brand gradient: `linear-gradient(120deg, #f29af3 0%, #b983f6 48%, #7c8ff7 100%)`.
- Neutral surfaces occupy the interface; brand colors are limited to compact selected, balance, logo, focus, and premium accents.
- No changes to typography, layout, spacing, sizing, radii, copy, icons, or behavior.
- Large page surfaces, cards, and primary buttons must not use the brand gradient.
- Do not push without a separate user request.

---

### Task 1: Global semantic palette

**Files:**
- Modify: `web/platform/src/app/globals.theme.test.ts`
- Modify: `web/platform/src/app/globals.css`

**Interfaces:**
- Consumes: existing `data-theme="dark"`, `data-theme="light"`, and `data-theme="system"` selectors.
- Produces: `--color-background`, `--color-workspace`, `--color-panel`, `--color-surface`, `--color-surface-raised`, `--color-border`, `--color-text`, `--color-text-muted`, three brand tokens, compatibility accent aliases, `--color-focus`, `--color-text-on-accent`, and `--gradient-brand`.

- [ ] **Step 1: Write the failing palette contract**

Replace the current palette expectations in `globals.theme.test.ts` with exact block-level assertions:

```ts
const themeBlock = (selector: string) =>
  stylesheet.match(new RegExp(`${selector}\\s*\\{([\\s\\S]*?)\\n\\}`))?.[1] ?? "";

const darkTheme = themeBlock(':root\\[data-theme="dark"\\]');
const lightTheme = themeBlock(':root\\[data-theme="light"\\]');

it("defines the approved graphite and brand palettes", () => {
  for (const token of [
    "--color-background: #0c0c0f",
    "--color-workspace: #111217",
    "--color-panel: #15161c",
    "--color-surface: #1a1b22",
    "--color-surface-raised: #20212a",
    "--color-border: #2a2b35",
    "--color-text: #f5f5f7",
    "--color-text-muted: #9b9da8",
    "--color-brand-violet: #9a7cf5",
    "--color-brand-blue: #7c8ff7",
    "--color-brand-pink: #f09af0",
    "--color-focus: #a9cfff",
  ]) expect(darkTheme).toContain(token);

  for (const token of [
    "--color-background: #f7f7fa",
    "--color-workspace: #ffffff",
    "--color-panel: #f3f3f7",
    "--color-surface: #eeeef4",
    "--color-surface-raised: #e8e8f0",
    "--color-border: #dedfe7",
    "--color-text: #17171b",
    "--color-text-muted: #6b6c76",
    "--color-brand-violet: #7563e6",
    "--color-brand-blue: #6678e6",
    "--color-brand-pink: #d56dd9",
    "--color-focus: #8475f0",
  ]) expect(lightTheme).toContain(token);

  expect(stylesheet).toContain(
    "--gradient-brand: linear-gradient(120deg, #f29af3 0%, #b983f6 48%, #7c8ff7 100%)",
  );
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm exec vitest -- run src/app/globals.theme.test.ts`

Expected: FAIL because the new semantic roles and approved values are absent.

- [ ] **Step 3: Implement the global palette**

In every theme block in `globals.css`, use the exact values from Global Constraints. Add these aliases after the literal brand tokens:

```css
--color-accent: var(--color-brand-violet);
--color-accent-strong: var(--color-brand-blue);
--gradient-brand: linear-gradient(120deg, #f29af3 0%, #b983f6 48%, #7c8ff7 100%);
```

Use `--color-text-on-accent: #111217` in dark and `--color-text-on-accent: #ffffff` in light/system-light. Preserve all status colors.

- [ ] **Step 4: Run the test and verify GREEN**

Run: `npm exec vitest -- run src/app/globals.theme.test.ts`

Expected: all tests in the file pass.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/app/globals.css web/platform/src/app/globals.theme.test.ts
git commit -m "feat(platform): add graphite brand palette"
```

### Task 2: Shell layer hierarchy

**Files:**
- Create: `web/platform/src/components/layout/AppShell/AppShell.palette.test.ts`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.module.css`

**Interfaces:**
- Consumes: the layer tokens from Task 1.
- Produces: black outer canvas, distinct workspace surface, panel-colored sidebar, and matching restoration skeleton.

- [ ] **Step 1: Write the failing shell contract**

Create `AppShell.palette.test.ts`:

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("AppShell palette roles", () => {
  it("uses neutral outer, workspace, and panel layers", () => {
    const shell = read("src/components/layout/AppShell/AppShell.module.css");
    const sidebar = read("src/components/layout/Sidebar/Sidebar.module.css");
    const restoration = read(
      "src/features/session/SessionRestorationShell/SessionRestorationShell.module.css",
    );

    expect(shell).not.toContain("#9494F8");
    expect(shell).toMatch(/--app-shell-canvas:\s*var\(--color-background\)/);
    expect(shell).toMatch(/\.workspace\s*\{[^}]*background:\s*var\(--color-workspace\)/s);
    expect(sidebar).toMatch(/\.panel\s*\{[^}]*background:\s*var\(--color-panel\)/s);
    expect(restoration).toContain("background: var(--color-panel)");
  });
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm exec vitest -- run src/components/layout/AppShell/AppShell.palette.test.ts`

Expected: FAIL on the old lavender canvas and missing workspace/panel mappings.

- [ ] **Step 3: Map the shell layers**

Make these exact color-only substitutions:

```css
/* AppShell.module.css */
--app-shell-canvas: var(--color-background);
.workspace { background: var(--color-workspace); }

/* Sidebar.module.css */
.panel { background: var(--color-panel); }

/* SessionRestorationShell.module.css */
.sidebar { background: var(--color-panel); }
```

Keep the existing `0.125rem` edge gap, margins, dimensions, radii, shadows, and scrolling rules unchanged.

- [ ] **Step 4: Run related tests and verify GREEN**

Run: `npm exec vitest -- run src/components/layout/AppShell src/components/layout/Sidebar src/features/session/SessionRestorationShell`

Expected: all related tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/components/layout/AppShell/AppShell.palette.test.ts web/platform/src/components/layout/AppShell/AppShell.module.css web/platform/src/components/layout/Sidebar/Sidebar.module.css web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.module.css
git commit -m "feat(platform): apply graphite shell layers"
```

### Task 3: Card, input, and elevated surface roles

**Files:**
- Create: `web/platform/src/app/palette-surfaces.contract.test.ts`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Modify: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.module.css`
- Modify: `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`
- Modify: `web/platform/src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css`
- Modify: `web/platform/src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css`
- Modify: `web/platform/src/features/account/ProfileLoginMethods/ProfileLoginMethods.module.css`
- Modify: `web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css`
- Modify: `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css`
- Modify: `web/platform/src/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation.module.css`
- Modify: `web/platform/src/features/image-generation/ImageJobHistory/ImageJobHistory.module.css`
- Modify: `web/platform/src/features/image-generation/ImageJobTracker/ImageJobTracker.module.css`
- Modify: `web/platform/src/features/image-generation/ImageGenerationResult/ImageGenerationResult.module.css`

**Interfaces:**
- Consumes: `--color-surface` for cards/inputs and `--color-surface-raised` for hover/elevated states.
- Produces: consistent layer separation without changing geometry.

- [ ] **Step 1: Write the failing surface-role contract**

Create a test that reads CSS class blocks and checks representative components:

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");
const rule = (css: string, selector: string) => {
  const match = css.match(new RegExp(`${selector}\\s*\\{([\\s\\S]*?)\\n\\}`));
  if (!match) throw new Error(`Missing selector ${selector}`);
  return match[1];
};

describe("palette surface roles", () => {
  it.each([
    ["src/features/models/ModelCard/ModelCard.module.css", "\\.card"],
    ["src/features/files/FileCard/FileCard.module.css", "\\.card"],
    ["src/features/workspace/FeaturedModels/FeaturedModels.module.css", "\\.card"],
    ["src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css", "\\.card"],
    ["src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css", "\\.card"],
  ])("uses the card surface in %s", (path, selector) => {
    expect(rule(read(path), selector)).toContain("background: var(--color-surface)");
  });

  it("keeps neutral hover and elevated states on the raised surface", () => {
    const selector = read(
      "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css",
    );
    expect(selector).toMatch(
      /\.option:hover,[\s\S]*?\.optionSelected\s*\{[^}]*background:\s*var\(--color-surface-raised\)/,
    );
  });
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm exec vitest -- run src/app/palette-surfaces.contract.test.ts`

Expected: FAIL because several base cards still use the old `surface-raised` role.

- [ ] **Step 3: Reassign only base surfaces**

Change base card/input containers from `background: var(--color-surface-raised)` to `background: var(--color-surface)`. Preserve `--color-surface-raised` for hover, focus-visible, selected-neutral, popover, menu, tooltip, scrollbar control, and other elevated rules. Do not edit any dimensions, spacing, borders, radii, or typography.

The required base selectors include:

```text
FeaturedModels .card
FeaturedModelShortcuts .shortcut
FileCard .card
ModelCard .card
ProfileBalanceCard .card
ProfileIdentityCard .card
ProfileLoginMethods .section
ProfileReferralFaq .item
ProfileReferralProgram .card
ImageGenerationConfirmation .card
ImageJobHistory .item
ImageJobTracker .tracker
ImageGenerationResult .result
ChatComposer input/composer base surfaces
ModelCatalogToolbar search/filter base surfaces
```

- [ ] **Step 4: Run surface and existing style tests**

Run: `npm exec vitest -- run src/app/palette-surfaces.contract.test.ts src/features/models src/features/files src/features/account src/components/chat`

Expected: all selected tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/app/palette-surfaces.contract.test.ts web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css web/platform/src/features/files/FileCard/FileCard.module.css web/platform/src/features/models/ModelCard/ModelCard.module.css web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css web/platform/src/components/chat/ChatComposer/ChatComposer.module.css web/platform/src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css web/platform/src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css web/platform/src/features/account/ProfileLoginMethods/ProfileLoginMethods.module.css web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css web/platform/src/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation.module.css web/platform/src/features/image-generation/ImageJobHistory/ImageJobHistory.module.css web/platform/src/features/image-generation/ImageJobTracker/ImageJobTracker.module.css web/platform/src/features/image-generation/ImageGenerationResult/ImageGenerationResult.module.css
git commit -m "feat(platform): align graphite component surfaces"
```

### Task 4: Restrained brand gradient and accessible accent foregrounds

**Files:**
- Create: `web/platform/src/app/brand-accents.contract.test.ts`
- Modify: `web/platform/src/components/public/PublicHeader/PublicHeader.module.css`
- Modify: `web/platform/src/components/public/PublicFooter/PublicFooter.module.css`
- Modify: `web/platform/src/components/public/PrimaryButton/PrimaryButton.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css`
- Modify: `web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.module.css`
- Modify: `web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.module.css`

**Interfaces:**
- Consumes: `--gradient-brand` and `--color-text-on-accent` from Task 1.
- Produces: gradient logo/hero/balance accents and readable solid accent controls.

- [ ] **Step 1: Write the failing accent contract**

Create `brand-accents.contract.test.ts`:

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("restrained brand accents", () => {
  it.each([
    "src/components/public/PublicHeader/PublicHeader.module.css",
    "src/components/public/PublicFooter/PublicFooter.module.css",
  ])("uses the brand gradient for the compact mark in %s", (path) => {
    expect(read(path)).toMatch(/\.brandMark\s*\{[^}]*background:\s*var\(--gradient-brand\)/s);
  });

  it("uses gradient text only for the landing hero accent and balance", () => {
    expect(read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css")).toMatch(
      /\.heroCopy h1 span\s*\{[^}]*background:\s*var\(--gradient-brand\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
    expect(read("src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css")).toMatch(
      /\.balance strong\s*\{[^}]*background:\s*var\(--gradient-brand\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
  });

  it("does not put the gradient on the primary landing button", () => {
    const css = read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css");
    const primary = css.match(/\.primaryButton\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
    expect(primary).not.toContain("--gradient-brand");
  });
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm exec vitest -- run src/app/brand-accents.contract.test.ts`

Expected: FAIL because the compact marks and gradient text still use a flat accent.

- [ ] **Step 3: Apply the compact accents**

Use these declarations without changing layout:

```css
.brandMark { background: var(--gradient-brand); }

.heroCopy h1 span,
.balance strong {
  background: var(--gradient-brand);
  background-clip: text;
  color: transparent;
}
```

In `PrimaryButton.module.css`, `WorkspaceLanding.module.css`, `WorkspaceLoginAction.module.css`, and `WorkspaceLogoutBoundary.module.css`, replace hardcoded white foregrounds on solid `var(--color-accent)` controls with `color: var(--color-text-on-accent)`. Do not add gradients to those controls.

- [ ] **Step 4: Run accent and component tests**

Run: `npm exec vitest -- run src/app/brand-accents.contract.test.ts src/components/public src/features/workspace src/features/account`

Expected: all selected tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/app/brand-accents.contract.test.ts web/platform/src/components/public/PublicHeader/PublicHeader.module.css web/platform/src/components/public/PublicFooter/PublicFooter.module.css web/platform/src/components/public/PrimaryButton/PrimaryButton.module.css web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css web/platform/src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css web/platform/src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.module.css web/platform/src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.module.css
git commit -m "feat(platform): apply restrained brand accents"
```

### Task 5: Full verification and local visual review

**Files:**
- Modify only if a verification failure exposes a palette regression; add a failing regression test before any fix.

**Interfaces:**
- Consumes: all previous tasks.
- Produces: verified dark/light and desktop/mobile palette behavior.

- [ ] **Step 1: Run the full automated suite**

Run each command from `web/platform`:

```bash
npm exec vitest -- run --maxWorkers=4
npm run test:assets
npm run lint
npm run typecheck
npm run build
npm run test:packaging
```

Expected: every command exits `0` with no failed tests or lint warnings.

- [ ] **Step 2: Audit the diff**

Run:

```bash
git diff --check origin/dev-deploy
git diff --unified=0 origin/dev-deploy -- web/platform/src | rg "^[+-].*(padding|margin|width|height|gap|border-radius|font-size|font-weight|line-height)"
```

Expected: `git diff --check` is empty; the geometry/typography audit has no production CSS changes from this palette task.

- [ ] **Step 3: Verify in the local browser**

Build and serve the platform. Inspect `/` and `/app` at desktop and `390x844` mobile sizes in explicit dark and light themes. Confirm:

- outer desktop canvas is nearly black, not lavender;
- workspace, sidebar, cards and hover layers are visually distinct;
- the right-hand workspace background is `#111217` in dark theme;
- brand gradient appears only on compact marks, the hero accent word and balance;
- focus, selected and disabled states remain visible;
- mobile removes the desktop edge canvas exactly as before.

- [ ] **Step 4: Confirm repository state**

Run:

```bash
git status --short --branch
git log --oneline -6
```

Expected: working tree is clean and the palette commits are ahead of `origin/dev-deploy`; do not push.
