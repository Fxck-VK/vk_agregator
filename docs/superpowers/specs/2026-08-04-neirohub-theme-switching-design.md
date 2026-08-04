# NeiroHub Theme Switching Design

Status: approved by the user on 2026-08-04.

## Goal

Turn the existing account-menu theme control into an immediate, accessible
three-way switch for system, light and dark appearance without involving the
backend or delaying workspace navigation.

## User experience

- The monitor option selects the operating-system appearance.
- The sun option selects the NeiroHub light palette.
- The moon option selects the existing NeiroHub dark palette.
- The selected option exposes `aria-pressed="true"` and uses the existing
  selected-pill treatment.
- A selection applies immediately across the whole document and survives a
  reload in the same browser.
- System mode follows `prefers-color-scheme` automatically when the operating
  system appearance changes.
- The page must not render a dark frame and then repaint light, or the reverse,
  during startup.

## Architecture

The browser preference is the non-sensitive local value `system`, `light` or
`dark`, stored under one namespaced local-storage key. A small theme module owns
validation, reading and applying that value so account UI does not duplicate
storage rules. The root layout emits `data-theme="system"` and a synchronous,
fail-safe bootstrap script in `<head>` updates the attribute from local storage
before the body is painted.

CSS variables remain the only application-wide color contract. Dark values
stay the default and light mode overrides the same tokens. In system mode a
`prefers-color-scheme: light` media query activates the light overrides, so no
JavaScript media-query subscription is required.

## Light palette

- page background: `#f6f7f9`
- surfaces: `#ffffff`
- raised and hover surfaces: `#eef1f5`
- border: `#e2e6ec`
- primary text: `#171a21`
- muted text: `#667085`
- primary accent: `#1688f8`
- strong accent: `#0877e6`
- focus: `#0f6fdb`
- danger: `#d92d20`

## Failure and security behavior

Unavailable or malformed storage falls back to system mode. Storage access is
wrapped because browser privacy settings can reject it. Theme selection never
contains identity, session, balance or other trusted state, never calls the
server, and never changes authentication, billing or job behavior.

## Verification

Automated tests cover preference validation and persistence, live account-menu
selection, the early root-layout bootstrap contract, and the light/dark CSS
token definitions. The complete frontend test, lint, typecheck and build
commands run before deployment.
