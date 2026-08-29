# Square sidebar conversation icons

## Goal

Keep the collapsed desktop sidebar visually consistent by rendering every conversation icon as a square with softly rounded corners instead of a circle.

## Scope

- Change only the purple conversation rail icon shown in the collapsed desktop sidebar.
- Preserve its existing `2rem` inline and block size, color, glyph, spacing, hover state, and active-row background.
- Use the existing `--radius-sm` design token (`0.5rem`) for the rounded corners.
- Keep the account avatar circular because it represents a person rather than a navigation item.
- Do not change expanded sidebar rows or mobile drawer geometry.

## Implementation

Update `.railIcon` in `ConversationRow.module.css` from `border-radius: 50%` to `border-radius: var(--radius-sm)`. Add a stylesheet contract test that requires the fixed square dimensions and the shared radius token.

## Verification

- Run the focused stylesheet test first in a failing state and then after the CSS change.
- Run the relevant conversation/sidebar tests, lint, and typecheck.
- Reload `http://localhost:7158/app` with the desktop sidebar collapsed and visually confirm that all conversation icons are rounded squares while the account avatar remains circular.
