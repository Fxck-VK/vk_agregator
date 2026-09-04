# Workspace Floating Scrollbar Design

## Goal

Show exactly one scrollbar in the workspace: a rounded thumb that visually floats over the main page without a visible track or contrasting gutter.

## Approved behavior

- When `AppShell` is mounted, the root document must not scroll, including in local Next.js development mode.
- The existing `workspaceScroller` remains the only scroll container and preserves native wheel, keyboard, touch, and drag behavior.
- The scrollbar track is transparent in Chromium/WebKit and Firefox.
- The scroller no longer reserves the extra right-side margin that currently looks like a dark strip.
- The scroller background matches the workspace landing-page background so the native scrollbar gutter is visually indistinguishable from the page.
- Public pages outside `AppShell` keep their normal document scrolling.

## Implementation

Add a stable `data-app-shell` marker to `AppShell`. A scoped global selector locks `html` and `body` only while that marker exists. Update the existing native scrollbar CSS instead of introducing JavaScript or a custom scrollbar dependency.

## Verification

- Static contract tests cover the scoped root lock, the single internal scroll owner, transparent tracks, removal of the extra inset, and matching background.
- The full platform test suite, typecheck, lint, and production build must pass.
- Runtime browser metrics must show `documentElement.scrollHeight === documentElement.clientHeight` while `workspaceScroller.scrollHeight > workspaceScroller.clientHeight`.
