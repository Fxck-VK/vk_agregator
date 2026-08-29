# All-models 90+ badge design

## Goal

Place the supplied tilted `90+` PNG above the transparent “Все нейросети” arrow tile so it reads as a small sticker protruding from the tile.

## Approved composition

- Reuse the supplied PNG without adding another rotation.
- Keep the arrow tile transparent by default and outlined with the existing accent color on hover and keyboard focus.
- Anchor the badge to the arrow tile’s upper-left corner with a small negative offset so part of the badge sits outside the 56 × 56 px tile.
- Keep the arrow centered and fully visible.
- Treat the badge as decorative: empty alternative text, no pointer interaction, and no change to the link’s accessible name.
- Keep the existing tile size, radius, label, destination, grid spacing, and hover translation.

## Asset handling

Store the supplied PNG under the platform’s public asset tree and expose it through `assetPaths`. The source file has a transparent background; CSS sizing and positioning will preserve the artwork’s existing tilt.

## Verification

- A source/CSS contract verifies the central asset path, decorative image markup, absolute positioning, and unchanged transparent/hover states.
- The focused test must fail before implementation and pass afterward.
- Verify the default and hover composition in the local browser at `http://localhost:7158/app`.
- Run the platform test suite, lint, and typecheck before committing.
