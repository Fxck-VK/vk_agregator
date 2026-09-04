# Workspace capability links design

## Goal

Restyle the six secondary capability links beneath the workspace capability cards to match the supplied reference: a centered divider title followed by two compact rows of outlined pills with an icon before every label.

## Design

- Add a centered `И многое другое` label above the links, with flexible horizontal rules on both sides.
- Keep the existing six labels, destinations, and ordering.
- Render each link as an outlined transparent pill with a compact horizontal layout, rounded ends, and the existing accent-border hover treatment.
- Use the existing theme-aware model fallback artwork as the temporary icon for every link. The icon remains decorative and hidden from assistive technology.
- Allow the pills to wrap responsively; the desktop composition forms two centered rows of three, while narrow screens can use fewer items per row without horizontal overflow.

## Alternatives considered

1. Create six temporary SVG icons. This resembles the reference more closely but creates throwaway assets that would later need replacement.
2. Add one new generic SVG placeholder. This is simple but duplicates the placeholder artwork already present in the product.
3. Reuse the existing theme-aware base placeholder. This keeps one source of truth and is the selected approach.

## Accessibility and behavior

- Navigation remains semantic and all current `href` values stay unchanged.
- Placeholder artwork is decorative (`aria-hidden`) so screen readers announce only the link label.
- Keyboard focus remains visible through the same accent-border treatment as hover.

## Verification

- A component regression test verifies the divider label, all six links, unchanged destinations, and six fallback icons.
- A stylesheet regression test verifies the divider structure, three-column desktop layout, outlined transparent pills, icon sizing, and responsive collapse.
- Run focused tests, full tests, typecheck, lint, production build, packaging checks, and inspect the rendered localhost page.
