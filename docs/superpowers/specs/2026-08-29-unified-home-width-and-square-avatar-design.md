# Unified home width and square avatar

## Goal

Make the workspace home page read as one narrow, centered composition and apply the sidebar's rounded-square geometry to the account avatar.

## Home page width

- The existing `50rem` content frame used by the hero and the four featured-model cards is the source of truth.
- Every following home-page section must use the same centered `50rem` frame: how it works, capabilities, plan, prompt library, FAQ, and community.
- The footer's inner content must use the same `50rem` maximum width.
- Existing section spacing, card dimensions within the available frame, responsive stacking, typography, and sidebar geometry remain unchanged.
- The full-width page background and footer background remain full width; only their inner content is constrained.

## Account avatar

- Keep the existing `2.5rem × 2.5rem` size, initials, color, and account-trigger layout.
- Replace the circular `50%` radius with the shared `--radius-sm` token (`0.5rem`).
- This supersedes the earlier circular-avatar exception: all primary sidebar icons now use rounded-square geometry.

## Verification

- Add stylesheet/source contract tests before implementation.
- Confirm all home sections resolve to the same content width in the local browser.
- Confirm the account avatar is `40×40 px` with an `8 px` radius.
- Run relevant tests, the full test suite, lint, and typecheck.
