# Platform Typography Scale Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the approved NeiroHub H1/H2/H3 scale and remove overly compressed Cyrillic tracking.

**Architecture:** Shared typography values remain centralized in `globals.css`. Existing display and section consumers inherit the updated tracking automatically, while genuine content H3 headings opt into a new subsection role; compact cards and controls keep their existing semantic roles.

**Tech Stack:** CSS custom properties, CSS Modules, Vitest contract tests.

## Global Constraints

- H1/display: `40px`, weight `600`, letter spacing `-0.01em`.
- H2/section: `32px`, weight `600`, letter spacing `-0.01em`.
- H3/subsection: `24px`, weight `600`, letter spacing `0`.
- Interface/body text remains `13px`–`18px` with zero letter spacing.
- Preserve current mobile H1/H2 sizes; reduce subsection size to `22px` below `48rem`.
- Do not enlarge compact card names, navigation labels, or controls.

---

### Task 1: Update global typography tokens

**Files:**
- Modify: `web/platform/src/app/globals.theme.test.ts`
- Modify: `web/platform/src/app/globals.css`

**Interfaces:**
- Produces: `--font-size-subsection`, `--line-height-subsection`, `--letter-spacing-subsection`, and `--letter-spacing-interface` tokens.
- Existing consumers continue using `--letter-spacing-display` and `--letter-spacing-section` with new approved values.

- [ ] **Step 1: Write the failing token contract test**

Add these assertions to the shared token contract:

```ts
expect(stylesheet).toContain("--font-size-subsection: 1.5rem");
expect(stylesheet).toContain("--line-height-subsection: 2rem");
expect(stylesheet).toContain("--letter-spacing-display: -0.01em");
expect(stylesheet).toContain("--letter-spacing-section: -0.01em");
expect(stylesheet).toContain("--letter-spacing-subsection: 0");
expect(stylesheet).toContain("--letter-spacing-interface: 0");
expect(stylesheet).not.toContain("--letter-spacing-display: -0.03em");
expect(stylesheet).not.toContain("--letter-spacing-section: -0.025em");
expect(stylesheet).toMatch(/body\s*\{[^}]*letter-spacing:\s*var\(--letter-spacing-interface\)/s);
expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-subsection:\s*1\.375rem/);
```

- [ ] **Step 2: Run the token test and verify RED**

Run: `npx vitest run src/app/globals.theme.test.ts`

Expected: FAIL because the subsection/interface tokens are absent and old compressed tracking is still present.

- [ ] **Step 3: Implement the approved tokens**

Add or update these declarations in `:root`:

```css
--font-size-subsection: 1.5rem;
--line-height-subsection: 2rem;
--letter-spacing-display: -0.01em;
--letter-spacing-section: -0.01em;
--letter-spacing-subsection: 0;
--letter-spacing-interface: 0;
```

Apply `letter-spacing: var(--letter-spacing-interface);` to `body` and add `--font-size-subsection: 1.375rem;` to the existing mobile root override.

- [ ] **Step 4: Run the token test and verify GREEN**

Run: `npx vitest run src/app/globals.theme.test.ts`

Expected: PASS.

### Task 2: Apply the H3 role to assistant content headings

**Files:**
- Modify: `web/platform/src/app/typography.contract.test.ts`
- Modify: `web/platform/src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css`

**Interfaces:**
- Consumes: subsection tokens created in Task 1.
- Produces: semantic assistant-message H3 headings at `24px/600/0` on desktop and `22px` through the mobile token override.

- [ ] **Step 1: Write the failing H3 contract test**

```ts
it("uses the subsection role for assistant-message H3 headings", () => {
  const headingRule = rule(
    "src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css",
    ".content h3",
  );

  expect(headingRule).toContain("font-size: var(--font-size-subsection)");
  expect(headingRule).toContain("line-height: var(--line-height-subsection)");
  expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
  expect(headingRule).toContain("letter-spacing: var(--letter-spacing-subsection)");
});
```

- [ ] **Step 2: Run the typography contract and verify RED**

Run: `npx vitest run src/app/typography.contract.test.ts`

Expected: FAIL because `.content h3` still uses a literal `1.25rem` size and inherits compressed heading tracking.

- [ ] **Step 3: Implement the subsection role**

Replace the current H3 rule with:

```css
.content h3 {
  font-size: var(--font-size-subsection);
  font-weight: var(--font-weight-semibold);
  letter-spacing: var(--letter-spacing-subsection);
  line-height: var(--line-height-subsection);
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npx vitest run src/app/globals.theme.test.ts src/app/typography.contract.test.ts`

Expected: both files PASS.

- [ ] **Step 5: Run full verification**

Run from `web/platform`:

```bash
npm test
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Expected: every command exits `0`.

- [ ] **Step 6: Commit the implementation**

```bash
git add web/platform/src/app/globals.css web/platform/src/app/globals.theme.test.ts web/platform/src/app/typography.contract.test.ts web/platform/src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css
git commit -m "style(platform): relax heading typography"
```
