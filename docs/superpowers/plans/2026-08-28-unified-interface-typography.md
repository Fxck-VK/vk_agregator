# Unified Interface Typography Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the approved Geist Sans typeface and compact semantic typography scale consistently across NeiroHub without changing layout or component geometry.

**Architecture:** Load the variable Geist font once in the App Router root layout and expose it as `--font-geist-sans`. Define semantic typography role tokens in `globals.css`, keep the existing public token names as aliases, and migrate existing CSS Modules by role rather than applying broad element selectors.

**Tech Stack:** Next.js 16.2 App Router, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Desktop roles are exactly: display 40px/44px/600, section 32px/38px/600, supporting 18px/27px/400, body 16px/24px/400–500, navigation 15px/22px/500, UI 14px/20px/500–600, caption 13px/18px/600.
- Display tracking is `-0.03em`; section tracking is `-0.025em`; ordinary UI tracking is `normal` except body may use at most `-0.01em`.
- Brand text is 18px/24px/600.
- Narrow display is 32px/38px; narrow section is 28px/34px.
- Geist Mono is not introduced; code and preformatted text keep their current monospace fallback.
- Do not change spacing, panel or card dimensions, borders, radii, colors, copy, data flow, or behavior.
- Markdown headings, icon glyphs, numeric display values, and media overlays remain explicit local exceptions.

---

### Task 1: Root font and semantic token contract

**Files:**
- Modify: `web/platform/src/app/layout.tsx`
- Modify: `web/platform/src/app/layout.test.tsx`
- Modify: `web/platform/src/app/globals.css`
- Modify: `web/platform/src/app/globals.theme.test.ts`

**Interfaces:**
- Consumes: Next.js `Geist` from `next/font/google` with `cyrillic`, `latin`, and `latin-ext` subsets.
- Produces: root CSS variable `--font-geist-sans` and semantic tokens `--font-size-*`, `--line-height-*`, `--font-weight-*`, and `--letter-spacing-*` used by later tasks.

- [ ] **Step 1: Add failing font and token assertions**

Mock the framework font loader in `layout.test.tsx` and assert that its variable class reaches `<html>`:

```tsx
vi.mock("next/font/google", () => ({
  Geist: vi.fn(() => ({ variable: "font-geist-sans-test" })),
}));

it("loads Geist Sans as the global interface font", async () => {
  const markup = renderToStaticMarkup(await RootLayout({ children: <main>Тест</main> }));
  const document = new DOMParser().parseFromString(markup, "text/html");

  expect(document.documentElement.classList.contains("font-geist-sans-test")).toBe(true);
});
```

Replace the old public-type assertions in `globals.theme.test.ts` with exact semantic role assertions:

```ts
expect(stylesheet).toContain("--font-size-display: 2.5rem");
expect(stylesheet).toContain("--line-height-display: 2.75rem");
expect(stylesheet).toContain("--font-size-section: 2rem");
expect(stylesheet).toContain("--line-height-section: 2.375rem");
expect(stylesheet).toContain("--font-size-supporting: 1.125rem");
expect(stylesheet).toContain("--line-height-supporting: 1.6875rem");
expect(stylesheet).toContain("--font-size-body: 1rem");
expect(stylesheet).toContain("--font-size-navigation: 0.9375rem");
expect(stylesheet).toContain("--font-size-ui: 0.875rem");
expect(stylesheet).toContain("--font-size-caption: 0.8125rem");
expect(stylesheet).toContain("--font-sans: var(--font-geist-sans)");
expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-display:\s*2rem/);
expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-section:\s*1\.75rem/);
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
cd web/platform
npm exec vitest -- run src/app/layout.test.tsx src/app/globals.theme.test.ts --reporter=dot
```

Expected: failures for the missing font class and missing semantic tokens.

- [ ] **Step 3: Load Geist and define the complete scale**

Add to `layout.tsx`:

```tsx
import { Geist } from "next/font/google";

const geistSans = Geist({
  display: "swap",
  subsets: ["cyrillic", "latin", "latin-ext"],
  variable: "--font-geist-sans",
});
```

Apply `className={geistSans.variable}` to `<html>`.

Define the role tokens in `globals.css`:

```css
--font-size-display: 2.5rem;
--line-height-display: 2.75rem;
--font-size-section: 2rem;
--line-height-section: 2.375rem;
--font-size-supporting: 1.125rem;
--line-height-supporting: 1.6875rem;
--font-size-body: 1rem;
--line-height-body: 1.5rem;
--font-size-navigation: 0.9375rem;
--line-height-navigation: 1.375rem;
--font-size-ui: 0.875rem;
--line-height-ui: 1.25rem;
--font-size-caption: 0.8125rem;
--line-height-caption: 1.125rem;
--font-weight-regular: 400;
--font-weight-medium: 500;
--font-weight-semibold: 600;
--letter-spacing-display: -0.03em;
--letter-spacing-section: -0.025em;
--font-sans: var(--font-geist-sans), ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
```

Keep `--font-size-label` and `--font-size-body-lg` as aliases, use the body tokens on `body`, and override only display and section size/line-height tokens inside `@media (width < 48rem)`.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the Step 2 command. Expected: both files pass.

- [ ] **Step 5: Commit the font foundation**

```bash
git add web/platform/src/app/layout.tsx web/platform/src/app/layout.test.tsx web/platform/src/app/globals.css web/platform/src/app/globals.theme.test.ts
git commit -m "feat(platform): add unified typography tokens"
```

---

### Task 2: Primary page and section hierarchy

**Files:**
- Create: `web/platform/src/app/typography.contract.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.module.css`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.module.css`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.module.css`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.module.css`
- Modify: `web/platform/src/components/public/SectionHeading/SectionHeading.module.css`

**Interfaces:**
- Consumes: Task 1 display, section, supporting, and caption tokens.
- Produces: consistent page H1, section H2, supporting copy, and kicker styling across authenticated and public surfaces.

- [ ] **Step 1: Write a failing stylesheet contract test**

Create `typography.contract.test.ts` with a helper that reads CSS Modules and asserts representative selectors:

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

function css(path: string) {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

describe("primary interface typography", () => {
  it.each([
    ["src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css", ".heroCopy h1"],
    ["src/features/workspace/WorkspaceHome/WorkspaceHome.module.css", ".content h1"],
    ["src/features/models/ModelsCatalog/ModelsCatalog.module.css", ".header h1"],
    ["src/features/files/FilesWorkspace/FilesWorkspace.module.css", ".header h1"],
    ["src/features/inspiration/InspirationGallery/InspirationGallery.module.css", ".heading h1"],
  ])("uses the display role in %s", (path, selector) => {
    const rule = css(path).match(new RegExp(`${selector.replace(/[.*+?^${}()|[\\]\\]/g, "\\$&")}\\s*\\{[^}]*\\}`, "s"))?.[0] ?? "";
    expect(rule).toContain("font-size: var(--font-size-display)");
    expect(rule).toContain("line-height: var(--line-height-display)");
    expect(rule).toContain("font-weight: var(--font-weight-semibold)");
  });
});
```

Also assert that workspace/public section headings use `--font-size-section` and supporting paragraphs use `--font-size-supporting` plus `--line-height-supporting`.

- [ ] **Step 2: Run the contract tests and verify RED**

```bash
cd web/platform
npm exec vitest -- run src/app/typography.contract.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts --reporter=dot
```

Expected: failures showing current `clamp(...)`, `1.0625rem`, and `1.125rem` declarations.

- [ ] **Step 3: Migrate page and section roles**

Use the same declarations for every page H1:

```css
font-size: var(--font-size-display);
font-weight: var(--font-weight-semibold);
letter-spacing: var(--letter-spacing-display);
line-height: var(--line-height-display);
```

Use the section role for product H2s and the supporting role for hero/section descriptions. Use the caption role for uppercase kickers. Remove component-local mobile H1 sizes because the root token override now owns responsive sizing; preserve wrapping rules and every non-typographic declaration.

Update `WorkspaceLanding.styles.test.ts` to expect the display token instead of the old `clamp(...)` value.

- [ ] **Step 4: Run Task 2 tests and verify GREEN**

Run the Step 2 command. Expected: all tests pass.

- [ ] **Step 5: Commit primary hierarchy migration**

```bash
git add web/platform/src/app/typography.contract.test.ts web/platform/src/features/workspace/WorkspaceLanding web/platform/src/features/workspace/WorkspaceHome web/platform/src/features/models/ModelsCatalog web/platform/src/features/files/FilesWorkspace web/platform/src/features/inspiration/InspirationGallery web/platform/src/components/public/SectionHeading
git commit -m "feat(platform): unify page typography hierarchy"
```

---

### Task 3: Navigation, selectors, composer, and account identity

**Files:**
- Modify: `web/platform/src/app/typography.contract.test.ts`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.module.css`
- Modify: `web/platform/src/features/conversations/NewConversationButton/NewConversationButton.module.css`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.module.css`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`

**Interfaces:**
- Consumes: Task 1 brand, navigation, UI, body, and caption role tokens.
- Produces: compact and consistent chrome around every workspace screen.

- [ ] **Step 1: Extend the stylesheet contract with failing UI-role assertions**

Add assertions for these exact mappings:

```ts
const expectedRoles = [
  ["src/components/layout/Sidebar/Sidebar.module.css", ".brand", "--font-size-supporting", "--line-height-body"],
  ["src/components/layout/Sidebar/Sidebar.module.css", ".navigationList a", "--font-size-navigation", "--line-height-navigation"],
  ["src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css", ".title", "--font-size-navigation", "--line-height-navigation"],
  ["src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css", ".trigger", "--font-size-navigation", "--line-height-navigation"],
  ["src/components/chat/ChatTextInput/ChatTextInput.module.css", ".input", "--font-size-body", "--line-height-body"],
] as const;
```

Assert balance, account identity, chat names, attachment labels, and button labels use `--font-size-ui`/`--line-height-ui`; sidebar headings and service labels use caption tokens.

- [ ] **Step 2: Run the contract test and verify RED**

```bash
cd web/platform
npm exec vitest -- run src/app/typography.contract.test.ts --reporter=dot
```

Expected: failures for missing role tokens in workspace chrome CSS.

- [ ] **Step 3: Apply the compact UI roles**

For navigation and the model selector use:

```css
font-size: var(--font-size-navigation);
font-weight: var(--font-weight-medium);
line-height: var(--line-height-navigation);
```

For standard UI labels use:

```css
font-size: var(--font-size-ui);
font-weight: var(--font-weight-medium);
line-height: var(--line-height-ui);
```

For sidebar section labels use caption size/line-height and semibold weight. For textareas and placeholders use body size/line-height and regular weight. The brand uses supporting size, body line-height, and semibold weight. Preserve every geometry and visibility declaration.

- [ ] **Step 4: Run Task 3 tests and verify GREEN**

Run the Step 2 command plus existing Sidebar, WorkspaceHeader, WorkspaceModelSelector, and ChatComposer tests. Expected: all pass.

- [ ] **Step 5: Commit workspace chrome migration**

```bash
git add web/platform/src/app/typography.contract.test.ts web/platform/src/components/layout/Sidebar web/platform/src/components/layout/WorkspaceHeader web/platform/src/features/conversations web/platform/src/features/models/WorkspaceModelSelector web/platform/src/components/chat/ChatComposer web/platform/src/components/chat/ChatTextInput web/platform/src/features/account/AccountMenu
git commit -m "feat(platform): unify workspace control typography"
```

---

### Task 4: Model cards and secondary product surfaces

**Files:**
- Modify: `web/platform/src/app/typography.contract.test.ts`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.module.css`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Modify: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`
- Modify: `web/platform/src/features/files/FilesEmptyState/FilesEmptyState.module.css`
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css`
- Modify: `web/platform/src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.module.css`
- Modify: `web/platform/src/features/account/ProfileWorkspace/ProfileWorkspace.module.css`
- Modify: `web/platform/src/features/account/ProfileLoginMethods/ProfileLoginMethods.module.css`
- Modify: `web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css`
- Modify: `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css`
- Modify: `web/platform/src/components/public/ModelPreviewCard/ModelPreviewCard.module.css`
- Modify: `web/platform/src/components/public/EmptyState/EmptyState.module.css`

**Interfaces:**
- Consumes: all Task 1 role tokens.
- Produces: consistent model names, card metadata, empty-state headings, image workflow guidance, and profile section headings.

- [ ] **Step 1: Add failing representative card and secondary-heading assertions**

Assert model names and card labels use UI tokens, card descriptions use body tokens, empty-state and workflow modal headings use section tokens, and profile subsection headings use supporting size/body line-height at semibold weight. Keep generated assistant Markdown out of these assertions.

```ts
expect(css("src/features/models/ModelCard/ModelCard.module.css")).toMatch(
  /\.heading h3\s*\{[^}]*font-size:\s*var\(--font-size-ui\)[^}]*line-height:\s*var\(--line-height-caption\)/s,
);
expect(css("src/features/workspace/FeaturedModels/FeaturedModels.module.css")).toMatch(
  /\.copy strong\s*\{[^}]*font-size:\s*var\(--font-size-ui\)[^}]*line-height:\s*var\(--line-height-caption\)/s,
);
```

- [ ] **Step 2: Run the contract and related component tests and verify RED**

```bash
cd web/platform
npm exec vitest -- run src/app/typography.contract.test.ts src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/features/models/ModelCard/ModelCard.test.tsx --reporter=dot
```

Expected: contract failures on the current raw card sizes.

- [ ] **Step 3: Migrate card and secondary roles**

Use UI size and caption line-height for compact model/file names and metadata, body tokens for descriptions, supporting size/body line-height for subsection headings, and section tokens only for genuine standalone empty-state or modal headings. Keep prices, generated output values, icon glyphs, and media controls unchanged.

- [ ] **Step 4: Run Task 4 tests and verify GREEN**

Run the Step 2 command. Expected: all tests pass.

- [ ] **Step 5: Commit secondary surface migration**

```bash
git add web/platform/src/app/typography.contract.test.ts web/platform/src/features/models/ModelCard web/platform/src/features/workspace/FeaturedModels web/platform/src/features/workspace/FeaturedModelShortcuts web/platform/src/features/files web/platform/src/features/image-generation/ImageTemplatePicker web/platform/src/features/image-generation/ImageGenerationGuide web/platform/src/features/account web/platform/src/components/public/ModelPreviewCard web/platform/src/components/public/EmptyState
git commit -m "feat(platform): unify card and section typography"
```

---

### Task 5: Regression and production verification

**Files:**
- Modify only if a verification failure reveals a typography-related regression.

**Interfaces:**
- Consumes: completed Tasks 1–4.
- Produces: verified, deployable typography change with no unrelated working-tree edits.

- [ ] **Step 1: Scan the diff for forbidden design changes**

```bash
git diff origin/dev-deploy -- web/platform/src ':!**/*.test.*'
git diff --check
```

Expected: only font loading and typography declarations changed; no spacing, size geometry, borders, radii, colors, or copy changed.

- [ ] **Step 2: Run the complete automated suite**

```bash
cd web/platform
npm test -- --maxWorkers=4
npm run lint
npm run typecheck
npm run build
npm run test:packaging
```

Expected: every command exits 0; Vitest and asset tests report zero failures.

- [ ] **Step 3: Verify rendered font assets and representative pages locally**

Start the built platform and inspect `/app`, `/app/models`, `/app/files`, and a narrow viewport. Confirm computed font family begins with Geist, H1 is 40/44 desktop and 32/38 narrow, section H2 is 32/38 desktop and 28/34 narrow, and sidebar/control sizes match their assigned roles. If local backend data is unavailable, use deterministic component tests for data-bound content and still verify the global font asset is served by the production build.

- [ ] **Step 4: Record final repository state**

```bash
git status --short --branch
git log -5 --oneline
```

Expected: clean worktree and local branch ahead of `origin/dev-deploy`; do not push unless the user asks separately.
