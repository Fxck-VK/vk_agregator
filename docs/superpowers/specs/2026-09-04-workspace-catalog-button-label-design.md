# Workspace Catalog Button Label Design

## Goal

Make the label on the glossy pink-blue catalogue button feel consistent with the dark NeiroHub interface.

## Approved appearance

- Both `Показать ещё` and `Все нейросети` use the same light label because they share `catalogAction`.
- Label color is `#f5f5f7`, matching the platform's primary dark-theme text.
- A subtle `0 0.0625rem 0.2rem rgb(0 0 0 / 45%)` text shadow keeps the label readable over both bright halves of the PNG background.
- The background image, dimensions, typography weight, hover movement, and navigation behavior remain unchanged.

## Verification

Extend the existing `FeaturedModels` CSS contract test, run it RED then GREEN, and verify the complete platform test, typecheck, lint, and build commands.
