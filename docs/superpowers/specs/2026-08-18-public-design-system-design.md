# Public design system design

## Objective

Formalize the existing NeiroHub visual language as reusable public UI without
redesigning the authenticated workspace. The public surface must inherit the
current dark and light palettes, typography, spacing, radii, motion, focus
behavior, and responsive conventions while remaining structurally independent
from `/app`.

## Scope

This chapter includes:

- semantic design tokens for containers, type scale, radii, shadows, headers,
  interaction states, and status colors;
- reusable public header, footer, container, heading, action, card, FAQ, and
  empty-state components;
- a `PublicShell` used by the existing public route group;
- a compact public theme switcher backed by the existing theme preference API;
- migration of the current `/` page to the shared public components;
- component, style-contract, route-boundary, type, lint, and build verification.

This chapter excludes:

- changes to `/app`, its sidebar, workspace header, chat, account menu, or data
  flows;
- model, tool, article, prompt, comparison, pricing, or catalog route templates;
- new editorial copy, runtime prices, account state, SEO metadata, JSON-LD,
  sitemap, robots, canonical, or hreflang work;
- copying a competitor's branding, assets, or proprietary content.

## Existing visual language

The implementation preserves the current NeiroHub foundation:

- Inter/system sans typography;
- graphite dark surfaces and a restrained blue accent;
- the approved light palette;
- rounded bordered surfaces, pill actions, and compact motion;
- visible keyboard focus and reduced-motion support;
- responsive layouts centered on the existing 48rem application breakpoint.

The public layer extends these tokens rather than replacing their current
values. Existing token names remain valid so authenticated workspace CSS is not
forced through a migration.

## Token contract

Global tokens add semantic values for:

- narrow, content, and wide containers;
- body, label, section-title, and display typography;
- tight, normal, and relaxed line heights;
- extra-large and pill radii;
- card, floating-menu, and mobile-overlay shadows;
- desktop and mobile header heights;
- success, warning, and information foreground/surface colors;
- shared hover, active, disabled, and loading opacity behavior.

Dark, light, and system-light themes define the complete semantic color set.
No public component may introduce a second competing palette.

## Component boundaries

Every public component lives in its own folder with its React source and CSS
Module. Components expose semantic content props and do not read account data.

- `PageContainer` controls horizontal padding and maximum width.
- `SectionHeading` renders an optional eyebrow, heading, description, and action.
- `PrimaryButton` and `SecondaryButton` are link actions with shared dimensions.
- `ContentCard` is the neutral bordered content surface.
- `ModelPreviewCard` renders editorial model content only; runtime facts remain
  backend-owned.
- `FAQ` uses native `details` and `summary` semantics.
- `EmptyState` renders an accessible non-error absence state.
- `PublicThemeSwitcher` reuses the existing system/light/dark persistence.
- `PublicHeader` and `PublicFooter` own public navigation and product identity.
- `PublicShell` composes the public header, main content, and footer.

## Public shell

The `(public)` route-group layout wraps public pages in `PublicShell`. It must
not import `AppShell`, authenticated session loaders, balance APIs, sidebar
components, or workspace dictionaries. The header is sticky, uses a compact
brand/navigation/action composition, and keeps navigation optional until the
corresponding public routes exist. The footer contains only live links.

The current `/` page keeps its intentionally small content scope but is rebuilt
from shared primitives. Full homepage sections remain a later, separately
approved UI chapter.

## Accessibility and responsive behavior

- Header, navigation, main, footer, headings, links, and FAQ use native semantic
  elements.
- Theme controls expose names and `aria-pressed` state.
- Focus uses the existing global focus ring.
- Motion respects the existing reduced-motion rule.
- At narrow widths the public header wraps safely, navigation can scroll, and
  section actions stack without horizontal overflow.

## Testing

Tests must prove:

- the complete semantic token contract exists in all theme palettes;
- public primitives render their expected semantics and links;
- the theme switcher persists and applies the selected preference;
- `PublicShell` renders header, main content, and footer without workspace UI;
- the `(public)` layout uses `PublicShell` and remains indexable;
- the existing `/app` boundary and tests remain unchanged;
- lint, typecheck, the full Vitest suite, packaging, and production build pass.

## Definition of done

The chapter is complete when the approved existing visual language is expressed
through shared tokens and public components, the current public page uses the
new shell, no authenticated workspace behavior changes, and all verification
commands pass.
