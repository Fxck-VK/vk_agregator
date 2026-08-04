# NeiroHub Theme Switching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing system/light/dark account-menu control switch the complete NeiroHub palette immediately and without startup flashing.

**Architecture:** A focused browser theme module validates and applies a local preference to the root `data-theme` attribute. The root layout bootstraps that attribute before paint, while global CSS maps dark, light and system appearance to the same semantic tokens.

**Tech Stack:** TypeScript, React 19, Next.js 16, CSS Modules, Vitest, Testing Library

## Global Constraints

- Theme preference is one of `system`, `light`, or `dark`.
- Preference storage is local, non-sensitive and must never block rendering.
- System appearance follows `prefers-color-scheme` without an API request.
- The early script uses the existing per-request CSP nonce; CSP is never weakened.
- Backend, auth, billing, jobs and account contracts remain unchanged.
- All implementation steps follow red-green-refactor.

---

### Task 1: Browser theme preference contract

**Files:**
- Create: `web/platform/src/features/theme/theme-preference.ts`
- Test: `web/platform/src/features/theme/theme-preference.test.ts`

**Interfaces:**
- Produces: `ThemePreference`, `themeStorageKey`, `themeBootstrapScript`, `readThemePreference()`, and `applyThemePreference()`.

- [ ] **Step 1: Write the failing test**

```ts
import { afterEach, describe, expect, it } from "vitest";

import {
  applyThemePreference,
  readThemePreference,
  themeStorageKey,
} from "./theme-preference";

describe("theme preference", () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("falls back to system for absent or malformed storage", () => {
    expect(readThemePreference()).toBe("system");
    localStorage.setItem(themeStorageKey, "unknown");
    expect(readThemePreference()).toBe("system");
  });

  it.each(["system", "light", "dark"] as const)("applies and persists %s", (preference) => {
    applyThemePreference(preference);
    expect(document.documentElement).toHaveAttribute("data-theme", preference);
    expect(localStorage.getItem(themeStorageKey)).toBe(preference);
    expect(readThemePreference()).toBe(preference);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/features/theme/theme-preference.test.ts`

Expected: FAIL because `./theme-preference` does not exist.

- [ ] **Step 3: Write the minimal implementation**

```ts
export const themeStorageKey = "neirohub.theme";
export const themePreferences = ["system", "light", "dark"] as const;
export type ThemePreference = (typeof themePreferences)[number];

function isThemePreference(value: string | null): value is ThemePreference {
  return themePreferences.some((preference) => preference === value);
}

export function readThemePreference(): ThemePreference {
  try {
    const value = window.localStorage.getItem(themeStorageKey);
    return isThemePreference(value) ? value : "system";
  } catch {
    return "system";
  }
}

export function applyThemePreference(preference: ThemePreference): void {
  document.documentElement.dataset.theme = preference;
  try {
    window.localStorage.setItem(themeStorageKey, preference);
  } catch {
    // The root attribute still provides the selected theme for this page.
  }
}

export const themeBootstrapScript =
  `(()=>{try{const value=window.localStorage.getItem("${themeStorageKey}");` +
  `document.documentElement.dataset.theme=value==="system"||value==="light"||value==="dark"?value:"system";` +
  `}catch{document.documentElement.dataset.theme="system";}})();`;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- src/features/theme/theme-preference.test.ts`

Expected: PASS.

### Task 2: Interactive account-menu selector

**Files:**
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`

**Interfaces:**
- Consumes: `ThemePreference`, `readThemePreference()`, and `applyThemePreference()` from Task 1.

- [ ] **Step 1: Add the failing interaction test**

```ts
it("switches and persists the selected appearance without closing the menu", async () => {
  render(<AccountControl profile={profile} />);
  fireEvent.click(screen.getByRole("button", { name: ru.account.openMenuLabel }));

  const system = screen.getByRole("button", { name: ru.account.systemThemeLabel });
  const light = screen.getByRole("button", { name: ru.account.lightThemeLabel });
  const dark = screen.getByRole("button", { name: ru.account.darkThemeLabel });

  expect(system).toBeEnabled();
  expect(light).toBeEnabled();
  expect(dark).toBeEnabled();

  fireEvent.click(light);

  expect(light).toHaveAttribute("aria-pressed", "true");
  expect(system).toHaveAttribute("aria-pressed", "false");
  expect(document.documentElement).toHaveAttribute("data-theme", "light");
  expect(localStorage.getItem("neirohub.theme")).toBe("light");
  expect(screen.getByRole("region", { name: ru.account.menuLabel })).toBeInTheDocument();
});
```

Extend the existing cleanup with:

```ts
localStorage.clear();
document.documentElement.removeAttribute("data-theme");
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/features/account/AccountControl/AccountControl.test.tsx`

Expected: FAIL because placeholder theme controls are disabled.

- [ ] **Step 3: Implement the shared selector path**

In `AccountMenu.tsx`, initialize `ThemePreference` as `system`, read the stored
preference in the existing client lifecycle, and use this handler for all
three buttons:

```ts
const selectTheme = (preference: ThemePreference) => {
  applyThemePreference(preference);
  setThemePreference(preference);
};
```

Each button must use:

```tsx
aria-pressed={themePreference === preference}
className={`${styles.themeOption} ${themePreference === preference ? styles.themeSelected : ""}`}
onClick={() => selectTheme(preference)}
```

Remove the placeholder `disabled` state and add a hover treatment to enabled
theme options without changing the menu layout.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm test -- src/features/account/AccountControl/AccountControl.test.tsx src/features/account/AccountMenu/AccountMenu.styles.test.ts`

Expected: PASS.

### Task 3: Before-paint bootstrap and semantic palettes

**Files:**
- Modify: `web/platform/src/app/layout.tsx`
- Modify: `web/platform/src/app/layout.test.tsx`
- Modify: `web/platform/src/app/globals.css`
- Create: `web/platform/src/app/globals.theme.test.ts`

**Interfaces:**
- Consumes: `themeBootstrapScript` from Task 1.
- Produces: the root `data-theme` startup contract and complete dark/light token maps.

- [ ] **Step 1: Add failing layout assertions**

```ts
expect(document.documentElement).toHaveAttribute("data-theme", "system");
expect(document.querySelector("head script")?.textContent).toContain("neirohub.theme");
expect(markup.indexOf("<script")).toBeLessThan(markup.indexOf("<body"));
```

Add `globals.theme.test.ts` that reads `globals.css` and asserts the stylesheet
contains `:root[data-theme="light"]`, `:root[data-theme="dark"]`,
`:root[data-theme="system"]` inside a `prefers-color-scheme: light` query, the
approved `#f6f7f9`, `#ffffff`, `#171a21`, `#667085`, and `color-scheme`.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- src/app/layout.test.tsx src/app/globals.theme.test.ts`

Expected: FAIL because the layout has no theme bootstrap and global CSS has no light mapping.

- [ ] **Step 3: Implement root startup and palette mapping**

Render the root as:

```tsx
const nonce = (await headers()).get("x-nonce") ?? undefined;

<html data-theme="system" lang="ru" suppressHydrationWarning>
  <head>
    <script dangerouslySetInnerHTML={{ __html: themeBootstrapScript }} nonce={nonce} />
  </head>
  <body>{children}</body>
</html>
```

Keep the existing dark semantic values under the dark/default root. Add the
approved light values under `:root[data-theme="light"]`. Repeat those semantic
assignments for `:root[data-theme="system"]` only inside
`@media (prefers-color-scheme: light)`. Set `color-scheme: dark` for the
dark/default path and `color-scheme: light` for both light paths.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `npm test -- src/app/layout.test.tsx src/app/globals.theme.test.ts`

Expected: PASS.

### Task 4: Full verification and DEV publication

**Files:**
- Modify only files required by Tasks 1-3, `docs/INDEX.md`, and the two task documents.

- [ ] **Step 1: Run all frontend checks**

```powershell
npm test
npm run lint
npm run typecheck
npm run build
```

Expected: every command exits `0` without warnings treated as errors.

- [ ] **Step 2: Inspect the final change**

```powershell
git diff --check
git diff --stat
git status --short
```

Expected: only theme feature, tests and routed task documentation are changed.

- [ ] **Step 3: Commit the feature**

```powershell
git add docs/INDEX.md docs/superpowers/specs/2026-08-04-neirohub-theme-switching-design.md docs/superpowers/plans/2026-08-04-neirohub-theme-switching.md web/platform/src/app web/platform/src/features/account web/platform/src/features/theme
git commit -m "feat(web): add theme switching"
```

- [ ] **Step 4: Publish to DEV**

Fetch `origin/dev-deploy`, rebase the isolated feature branch when the remote
base changed, repeat focused verification after any rebase, then run:

```powershell
git push origin HEAD:dev-deploy
```

Expected: the remote `dev-deploy` branch advances to the verified feature commit.
