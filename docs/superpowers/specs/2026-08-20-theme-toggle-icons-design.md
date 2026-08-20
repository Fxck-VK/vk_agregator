# Theme Toggle Icons Design

## Goal

Replace the three inline theme-toggle glyphs in the account menu with the approved monitor, sun, and moon SVG artwork without changing theme-selection behavior.

## Design

- Store the approved source files under `public/assets/icons/theme/`.
- Render them through the existing `AssetIcon` CSS-mask component so their visible color continues to inherit `currentColor` from each theme button.
- Expose one focused React wrapper per icon: `MonitorIcon`, `SunIcon`, and `MoonIcon`.
- Keep the existing theme buttons, labels, `aria-pressed` state, persistence, and click handlers unchanged.
- Size the shared asset icons exactly like the previous inline SVGs.

## Verification

- Component tests assert that each theme button renders the correct approved shared icon.
- Shared-icon and asset-path tests cover the three new assets.
- Asset validation, lint, typecheck, tests, and the production build must pass.
