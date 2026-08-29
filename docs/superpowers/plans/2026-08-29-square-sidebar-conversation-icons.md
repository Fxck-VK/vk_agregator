# Square Sidebar Conversation Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render collapsed-sidebar conversation icons as equal rounded squares while keeping the account avatar circular.

**Architecture:** Preserve the existing `ConversationRailIcon` markup and collapsed-sidebar layout. Enforce the shape at the component stylesheet boundary using the existing `--radius-sm` token and protect it with a focused stylesheet contract test.

**Tech Stack:** Next.js, React, CSS Modules, Vitest

## Global Constraints

- Change only the purple conversation rail icon shown in the collapsed desktop sidebar.
- Preserve the existing `2rem` dimensions, color, glyph, spacing, hover state, and active-row background.
- Keep the account avatar circular.
- Do not change expanded sidebar rows or mobile drawer geometry.

---

### Task 1: Square conversation rail icon

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css`
- Test: `web/platform/src/features/conversations/ConversationRow/ConversationRow.styles.test.ts`

**Interfaces:**
- Consumes: `.railIcon` rendered by `ConversationRailIcon` with `data-sidebar-conversation-icon="true"`.
- Produces: a fixed `2rem × 2rem` icon with `border-radius: var(--radius-sm)`.

- [ ] **Step 1: Write the failing stylesheet contract test**

Add this assertion inside the existing style test suite:

```ts
expect(stylesheet).toMatch(
  /\.railIcon\s*\{[^}]*inline-size:\s*2rem;[^}]*block-size:\s*2rem;[^}]*border-radius:\s*var\(--radius-sm\);/s,
);
```

- [ ] **Step 2: Run the focused test and verify the expected failure**

Run:

```powershell
npm exec -- vitest run src/features/conversations/ConversationRow/ConversationRow.styles.test.ts
```

Expected: FAIL because `.railIcon` still uses `border-radius: 50%`.

- [ ] **Step 3: Apply the minimal CSS change**

Change the `.railIcon` rule to:

```css
.railIcon {
  display: none;
  inline-size: 2rem;
  block-size: 2rem;
  place-items: center;
  border-radius: var(--radius-sm);
  background: var(--color-accent);
  color: #fff;
}
```

- [ ] **Step 4: Verify focused and related tests**

Run:

```powershell
npm exec -- vitest run src/features/conversations/ConversationRow/ConversationRow.styles.test.ts src/components/layout/Sidebar/Sidebar.test.tsx
npm run lint
npm run typecheck
```

Expected: all commands exit with code `0`.

- [ ] **Step 5: Verify the local page visually**

Reload `http://localhost:7158/app`, keep the desktop sidebar collapsed, and confirm:

- every purple conversation icon is a rounded square;
- active and inactive icons have identical geometry;
- the account avatar remains circular.

- [ ] **Step 6: Commit the implementation**

```powershell
git add web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css web/platform/src/features/conversations/ConversationRow/ConversationRow.styles.test.ts
git commit -m "style(platform): square sidebar conversation icons"
```
