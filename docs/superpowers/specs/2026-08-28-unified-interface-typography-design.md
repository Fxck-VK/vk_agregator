# Unified interface typography design

## Objective

Replace the current scattered typography values with one compact interface scale based on Geist Sans. The change must strengthen hierarchy and consistency without changing component geometry, spacing, colors, borders, radii, or application behavior.

## Chosen approach

Use semantic typography tokens in `globals.css`, load Geist Sans once at the root layout, and migrate interface roles to those tokens through their existing CSS Modules.

This is preferred over two alternatives:

- global element selectors would be fast but would incorrectly restyle content headings, nested controls, and third-party markup;
- rewriting every CSS rule in one undifferentiated sweep would create unnecessary visual risk and make later exceptions difficult to understand.

The selected token-based approach keeps typography centralized while preserving component ownership.

## Font delivery

- Geist Sans is loaded through the framework font loader and exposed as the global sans-serif CSS variable.
- The application keeps system sans fonts as a fallback chain.
- Geist Mono is not introduced. Existing code and preformatted content keep their current monospace fallback.
- The font applies to public and authenticated NeiroHub UI because both inherit from the root layout.

## Type scale

The desktop scale contains seven roles:

| Role | Size / line height | Weight | Tracking |
| --- | --- | --- | --- |
| Display / H1 | 40px / 44px | 600 | -0.03em |
| Section / H2 | 32px / 38px | 600 | -0.025em |
| Supporting | 18px / 27px | 400 | normal |
| Body and inputs | 16px / 24px | 400–500 | normal to -0.01em |
| Navigation and model selector | 15px / 22px | 500 | normal |
| UI labels, model names, balance and email | 14px / 20px | 500–600 | normal |
| Service labels | 13px / 18px | 600 | normal |

Brand text uses 18px / 24px at weight 600. Buttons use the UI or navigation role according to their existing visual prominence.

At narrow widths, display headings reduce to 32px / 38px and section headings reduce to 28px / 34px. Body and control text do not shrink below their assigned role.

## Scope and mapping

The migration covers:

- global body font and reusable typography tokens;
- workspace hero and section headings;
- models catalogue headings and model cards;
- sidebar brand, navigation, chat labels, account identity, and balance;
- workspace model selector, composer controls, inputs, and placeholders;
- files, inspiration, image-generation, profile, and public interface headings that represent the same semantic roles.

User-generated assistant content remains a content typography surface. Its paragraph text aligns to the 16px / 24px body role, but its internal Markdown heading hierarchy remains local so generated documents are still readable. Media overlays, icon glyphs, numeric display values, and code text remain explicit exceptions.

## Token contract

`globals.css` defines semantic variables for every role's size, line height, weight, and heading tracking. Existing public token names remain available as aliases during migration so no component is silently broken.

Components consume role tokens instead of copying raw pixel or rem values. A component may keep an explicit value only when it is a documented exception such as an icon glyph or generated-content heading.

## Non-goals

- No spacing, panel size, card size, border, radius, color, or layout changes.
- No copy changes.
- No new mono font.
- No redesign of Markdown heading hierarchy or media controls.
- No unrelated CSS cleanup.

## Verification

Tests must prove that:

- the root layout exposes Geist Sans through the global font variable;
- the complete semantic token scale and responsive heading values exist;
- representative workspace, sidebar, catalogue, composer, and public components consume the correct role tokens;
- legacy oversized values are removed from the migrated primary headings;
- the full Vitest suite, asset checks, lint, typecheck, packaging checks, and production build pass.

Local browser verification covers the workspace landing page, models catalogue, chat composer, sidebar, and a narrow viewport when the local backend data is available. Where backend data is unavailable, component tests provide the deterministic rendered state.

## Definition of done

The platform uses Geist Sans and the approved compact hierarchy consistently, primary interface roles no longer rely on scattered one-off sizes, responsive headings retain hierarchy without overflow, all non-typographic design remains unchanged, and the complete verification suite passes.
