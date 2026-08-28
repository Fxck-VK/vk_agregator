# Light Theme Model Placeholder SVG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace only the light-theme default model placeholder with the exact SVG geometry supplied by the user.

**Architecture:** Keep `ModelIcon`, its CSS theme switching, and both public asset paths unchanged. Add an asset contract test, then replace the contents of `chip-silhouette-dark.svg`; the dark-theme white placeholder remains untouched.

**Tech Stack:** React, CSS Modules, static SVG assets, Vitest.

## Global Constraints

- The light-theme placeholder fill is exactly `#15161C`.
- The supplied `1024 × 1024` view box, mask, contacts, face cut-outs, and smile geometry are preserved.
- Component markup, icon dimensions, spacing, borders, radii, cards, and the dark-theme asset do not change.
- Push is not performed without a separate user request.

---

### Task 1: Replace the light-theme placeholder asset

**Files:**
- Create: `web/platform/src/features/models/ModelIcon/ModelIcon.asset.test.ts`
- Modify: `web/platform/public/assets/images/models/chip-silhouette-dark.svg`
- Modify: `docs/superpowers/specs/2026-08-28-chip-silhouette-default-model-icon-design.md`

**Interfaces:**
- Consumes: the existing `/assets/images/models/chip-silhouette-dark.svg` path selected by `ModelIcon.module.css` in light mode.
- Produces: the same public URL, now rendering the exact user-supplied dark chip silhouette.

- [ ] **Step 1: Write the failing asset contract test**

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const lightThemePlaceholder = readFileSync(
  resolve(process.cwd(), "public/assets/images/models/chip-silhouette-dark.svg"),
  "utf8",
);

describe("light-theme model placeholder asset", () => {
  it("preserves the supplied masked chip geometry and color", () => {
    expect(lightThemePlaceholder).toContain('<mask id="face-cutout">');
    expect(lightThemePlaceholder).toContain('<g fill="#15161C">');
    expect(lightThemePlaceholder).toMatch(
      /x="180"\s+y="180"\s+width="664"\s+height="664"\s+rx="154"/s,
    );
    expect(lightThemePlaceholder).toMatch(
      /M405 585\s+C435 625 473 645 512 645\s+C551 645 589 625 619 585/s,
    );
    expect(lightThemePlaceholder).toContain('stroke-width="46"');
    expect(lightThemePlaceholder).not.toContain("#0C0C0F");
  });
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm exec -- vitest run src/features/models/ModelIcon/ModelIcon.asset.test.ts`

Expected: FAIL because the old asset has `#0C0C0F` and no `face-cutout` mask.

- [ ] **Step 3: Replace only the light-theme SVG asset**

Write the exact supplied SVG into `public/assets/images/models/chip-silhouette-dark.svg`, preserving `viewBox="0 0 1024 1024"`, mask ID `face-cutout`, fill `#15161C`, and all coordinates.

- [ ] **Step 4: Update the design record**

Document that the dark-theme asset remains a byte-preserved source file while the light-theme asset is the newly supplied inline markup. Remove the obsolete SHA-256 promise for the replaced light asset.

- [ ] **Step 5: Run targeted verification and verify GREEN**

Run: `npm exec -- vitest run src/features/models/ModelIcon/ModelIcon.asset.test.ts src/features/models/ModelIcon/ModelIcon.test.tsx src/features/models/ModelIcon/ModelIcon.styles.test.ts src/assets/asset-paths.test.ts`

Expected: all selected test files pass.

- [ ] **Step 6: Run full verification**

Run: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm run test:packaging`, and `git diff --check`.

Expected: every command exits with code 0.

- [ ] **Step 7: Commit locally**

```bash
git add docs/superpowers/plans/2026-08-29-light-theme-model-placeholder-svg.md docs/superpowers/specs/2026-08-28-chip-silhouette-default-model-icon-design.md web/platform/public/assets/images/models/chip-silhouette-dark.svg web/platform/src/features/models/ModelIcon/ModelIcon.asset.test.ts
git commit -m "fix(platform): update light theme model placeholder"
```
