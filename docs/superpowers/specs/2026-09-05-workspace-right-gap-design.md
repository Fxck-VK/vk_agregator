# Workspace right gap design

## Goal

Keep the desktop workspace panel at one fixed distance from the browser's right edge on every authenticated application route.

## Design

Define one global design token, `--app-workspace-edge-gap`, with the currently approved value of `0.125rem`. `AppShell` consumes that token for its outer panel geometry, so every route rendered through `WorkspaceFrame` inherits the same right gap without page-specific rules.

The existing block-axis gap and sidebar inset continue to use the same value so the shell remains visually balanced. At widths below `48rem`, the workspace keeps its existing edge-to-edge mobile layout with zero margin and no rounding.

## Verification

A stylesheet contract test must prove that the token is defined globally, consumed by `AppShell`, applied to `margin-inline-end`, and neutralized by the existing mobile rule. Existing AppShell tests, TypeScript, ESLint, and a browser inspection cover regression risk.
