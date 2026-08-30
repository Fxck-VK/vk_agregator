# Workspace Video Poster Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display the supplied NeiroHub PNG as the custom start screen for the existing workspace how-to video.

**Architecture:** Store the PNG in the platform public asset tree, expose its URL through `assetPaths`, and pass it from `WorkspaceLanding` to `VideoPlayer`. `VideoPlayer` applies the poster URL to its existing start overlay through a scoped CSS custom property while retaining the same URL on the native video `poster` attribute.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Use the supplied `1680 × 941` PNG without visual editing.
- Keep the existing centered circular Play button and playback behavior.
- Add no text or tint over the poster.
- Preserve the current `16 / 9` video frame dimensions.
- Keep the neutral gradient when no poster is supplied.
- Do not modify or stage `web/platform/next-env.d.ts`.

---

### Task 1: Register the poster asset

**Files:**
- Create: `web/platform/public/assets/images/workspace/neirohub-how-it-works-poster.png`
- Modify: `web/platform/src/assets/asset-paths.ts`
- Test: `web/platform/src/assets/asset-paths.test.ts`

**Interfaces:**
- Consumes: user-supplied `D:/Downloads/ChatGPT Image 30 авг. 2026 г., 09_48_29.png`
- Produces: `assetPaths.images.workspace.howItWorksPoster: string`

- [ ] **Step 1: Write the failing asset-path test**

```ts
it("exposes the workspace how-to poster URL", () => {
  expect(assetPaths.images.workspace.howItWorksPoster).toBe(
    "/assets/images/workspace/neirohub-how-it-works-poster.png",
  );
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/assets/asset-paths.test.ts`

Expected: FAIL because `howItWorksPoster` does not exist.

- [ ] **Step 3: Add the path and copy the exact PNG**

Add to `assetPaths.images.workspace`:

```ts
howItWorksPoster: "/assets/images/workspace/neirohub-how-it-works-poster.png",
```

Copy the supplied PNG byte-for-byte to the public asset path.

- [ ] **Step 4: Run focused asset checks and verify GREEN**

Run: `npm test -- src/assets/asset-paths.test.ts`

Expected: PASS.

Run: `npm run test:assets`

Expected: PASS and accept the kebab-case PNG.

### Task 2: Render the poster in the start overlay

**Files:**
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.tsx`
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.module.css`
- Test: `web/platform/src/components/media/VideoPlayer/VideoPlayer.test.tsx`
- Test: `web/platform/src/components/media/VideoPlayer/VideoPlayer.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Test: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

**Interfaces:**
- Consumes: optional `poster?: string` and `assetPaths.images.workspace.howItWorksPoster`
- Produces: `--video-player-poster` on the start overlay while playback has not started

- [ ] **Step 1: Write failing component and integration assertions**

Add assertions that the overlay receives:

```ts
expect(screen.getByTestId("video-poster-overlay")).toHaveStyle({
  "--video-player-poster": 'url("/assets/images/video/how-it-works-poster.webp")',
});
```

Add stylesheet assertions for `var(--video-player-poster)`, `center`, `cover`, and a Play-button z-index. Update the workspace markup test to require:

```ts
expect(markup).toContain('poster="/assets/images/workspace/neirohub-how-it-works-poster.png"');
```

- [ ] **Step 2: Run focused tests and verify RED**

Run: `npm test -- src/components/media/VideoPlayer/VideoPlayer.test.tsx src/components/media/VideoPlayer/VideoPlayer.styles.test.ts src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL because the overlay has no poster variable and `WorkspaceLanding` does not pass the poster.

- [ ] **Step 3: Apply the poster to the existing overlay**

In `VideoPlayer.tsx`, create the typed overlay style and attach it to the overlay:

```ts
const posterOverlayStyle = {
  "--video-player-poster": poster ? `url("${poster}")` : "none",
} as CSSProperties;
```

```tsx
<div
  className={styles.posterOverlay}
  data-testid="video-poster-overlay"
  style={posterOverlayStyle}
>
```

In the stylesheet, layer the poster above the fallback gradients without adding a tint:

```css
background:
  var(--video-player-poster) center / cover no-repeat,
  radial-gradient(circle at 72% 34%, rgb(36 150 255 / 0.14), transparent 28%),
  radial-gradient(circle at 24% 74%, rgb(116 87 255 / 0.12), transparent 32%),
  var(--color-surface);
```

Give `.playButton` `position: relative` and `z-index: 1`. Pass `poster={assetPaths.images.workspace.howItWorksPoster}` from `WorkspaceLanding`.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the Task 2 focused command again.

Expected: all focused tests PASS.

### Task 3: Verify and commit the implementation

**Files:**
- Verify all files from Tasks 1 and 2

**Interfaces:**
- Consumes: completed poster asset and player integration
- Produces: a verified local commit without `next-env.d.ts`

- [ ] **Step 1: Run the full verification suite**

Run from `web/platform`:

```text
npm test
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Expected: every command exits with code `0`.

- [ ] **Step 2: Verify repository hygiene**

Run: `git diff --check`

Expected: no whitespace errors.

Confirm the staged file list contains only the poster asset, path registry, player integration, landing integration, and their tests.

- [ ] **Step 3: Commit**

```text
git commit -m "feat(platform): add workspace video poster"
```

Expected: one implementation commit; no push.
