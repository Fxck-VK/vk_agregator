# Workspace Scrollbar Visual Alignment

## Goal

Match the workspace scrollbar shown in the approved reference without changing
the page's initial scroll position, content spacing, panel geometry, or scroll
behavior.

## Design

- Keep the native `.workspaceScroller` as the only vertical scroll container.
- Keep `scrollTop` at `0` on initial render; do not add JavaScript scrolling.
- In Chromium/WebKit, inset the transparent scrollbar track from the top so the
  thumb begins lower inside the rounded workspace surface.
- Use the existing dark border token for the thumb in idle, hover, and
  focus-within states. Keep the track transparent and the thumb narrow and
  rounded.
- In Firefox, keep the same dark thumb color in every state. Firefox does not
  expose an equivalent native track-start inset, so no layout workaround will
  be introduced.

## Scope

Only `AppShell.module.css` and its scrollbar style test may change. No content
padding, panel dimensions, radii, application state, or other design tokens are
in scope.

## Verification

- A style regression test must fail before the CSS change and pass afterward.
- Run the focused AppShell test, frontend lint, typecheck, test suite, and build.
- Verify the workspace visually at the reference desktop viewport.

