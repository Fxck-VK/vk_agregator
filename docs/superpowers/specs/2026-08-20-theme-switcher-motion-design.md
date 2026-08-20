# Theme Switcher Motion Design

## Goal

Make the selected theme surface glide between the system, light, and dark icons while the application palette changes smoothly.

## Interaction

- The three icon buttons remain stationary.
- One shared indicator moves to the selected column using `transform`.
- The indicator transition uses the existing normal motion duration (`220ms ease`).
- Theme colors transition at the same time without delaying persistence or `data-theme` updates.
- Hover styling remains available on non-selected options.
- `prefers-reduced-motion: reduce` keeps the existing global near-instant transition behavior.

## Architecture

- `AccountMenu` exposes the current preference to CSS through a stable data attribute.
- `AccountMenu.module.css` owns the sliding indicator and keeps the buttons above it.
- `globals.css` owns global theme-color transitions so all token-driven surfaces change together.
- No new dependency and no JavaScript animation library are introduced.

## Accessibility

- Existing buttons, labels, `aria-pressed`, keyboard behavior, and focus styles remain unchanged.
- The indicator is decorative and does not enter the accessibility tree.
- Reduced-motion preferences are respected.

## Verification

- Component tests assert the selected preference data attribute follows user input.
- Style tests assert one transform-driven indicator and reduced-motion compatibility.
- Existing theme persistence, lint, typecheck, asset validation, tests, and production build must remain green.
