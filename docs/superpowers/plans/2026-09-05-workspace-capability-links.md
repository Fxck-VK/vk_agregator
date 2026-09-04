# Workspace Capability Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the plain secondary capability chips with the approved divider-and-icon pill layout.

**Architecture:** Extract the block into a focused `CapabilityLinks` component so the new layout and tests remain isolated from the existing landing-page and FAQ edits. Reuse `ModelIcon` without a source to render the existing theme-aware fallback artwork, while preserving the current `capabilityLinks` data and destinations.

**Tech Stack:** React 19, Next.js 16, CSS Modules, Vitest, Testing Library

## Global Constraints

- Show `И многое другое` centered between two flexible horizontal rules.
- Keep all six existing labels, href values, and ordering unchanged.
- Render three centered pills per row on desktop and collapse responsively without horizontal overflow.
- Use the existing theme-aware model fallback as a decorative placeholder icon for every link.
- Do not modify the existing FAQ work already present in the landing-page worktree.

---

### Task 1: Capability links component

**Files:**
- Create: `web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.tsx`
- Create: `web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.test.tsx`
- Create: `web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.module.css`
- Create: `web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`

**Interfaces:**
- Consumes: `capabilityLinks` from `workspace-home-content.ts` and `ModelIcon({ className, src })` with an omitted `src`.
- Produces: `CapabilityLinks(): JSX.Element`, a self-contained divider and navigation block.

- [ ] **Step 1: Write the failing component test**

```tsx
render(<CapabilityLinks />);

expect(screen.getByRole("heading", { level: 3, name: "И многое другое" })).toBeInTheDocument();
const navigation = screen.getByRole("navigation", { name: "Дополнительные возможности" });
const links = within(navigation).getAllByTestId("workspace-capability-link");
expect(links).toHaveLength(6);
expect(links.map((link) => link.textContent)).toEqual(capabilityLinks.map((item) => item.label));
expect(links.map((link) => link.getAttribute("href"))).toEqual(capabilityLinks.map((item) => item.href));
expect(within(navigation).getAllByTestId("model-icon-fallback")).toHaveLength(6);
```

- [ ] **Step 2: Write the failing stylesheet test**

```ts
expect(dividerRule).toContain("grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr)");
expect(stylesheet).toMatch(/\.divider::before,[\s\S]*\.divider::after\s*\{[^}]*content:\s*"";[^}]*background:\s*var\(--color-border\);/s);
expect(listRule).toContain("grid-template-columns: repeat(3, max-content)");
expect(linkRule).toContain("background: transparent");
expect(iconRule).toContain("inline-size: 1.125rem");
expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*\.list\s*\{[^}]*grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/s);
expect(stylesheet).toMatch(/@media \(width < 36rem\)[\s\S]*\.list\s*\{[^}]*grid-template-columns:\s*1fr;/s);
```

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```powershell
npx vitest run src/features/workspace/WorkspaceLanding/CapabilityLinks.test.tsx src/features/workspace/WorkspaceLanding/CapabilityLinks.styles.test.ts
```

Expected: FAIL because `CapabilityLinks` and its stylesheet do not exist yet.

- [ ] **Step 4: Implement the minimal component and styles**

```tsx
export function CapabilityLinks() {
  return (
    <div className={styles.wrapper}>
      <h3 className={styles.divider}><span>И многое другое</span></h3>
      <nav aria-label="Дополнительные возможности" className={styles.list}>
        {capabilityLinks.map((item) => (
          <Link className={styles.link} data-testid="workspace-capability-link" href={item.href} key={item.label}>
            <ModelIcon className={styles.icon} />
            <span>{item.label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
```

Use a three-column `max-content` grid on desktop, a two-column equal grid below `48rem`, and a single column below `36rem`. Give `.link` a transparent background, pill radius, border, and accent hover/focus state. Size `.link .icon` to `1.125rem` square.

Replace the inline `chipList` navigation in `WorkspaceLanding.tsx` with `<CapabilityLinks />` and remove the now-unused `capabilityLinks` import.

- [ ] **Step 5: Run the focused tests and verify GREEN**

Run:

```powershell
npx vitest run src/features/workspace/WorkspaceLanding/CapabilityLinks.test.tsx src/features/workspace/WorkspaceLanding/CapabilityLinks.styles.test.ts
```

Expected: both test files pass with zero failures.

- [ ] **Step 6: Verify the complete platform**

Run sequentially:

```powershell
npm test
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Expected: every command exits `0`.

- [ ] **Step 7: Commit only the capability-link files**

```powershell
git add -- web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.tsx web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.test.tsx web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.module.css web/platform/src/features/workspace/WorkspaceLanding/CapabilityLinks.styles.test.ts web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx
git commit -m "style(platform): add icon capability links"
```
