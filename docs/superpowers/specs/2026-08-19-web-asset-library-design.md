# NeiroHub Web Asset Library Design

Date: 2026-08-19

Status: approved design, pending implementation plan

Scope: `web/platform/**`

## Goal

Create a predictable asset library for the NeiroHub web platform without
changing the current UI. Shared brand media, icons, illustrations and content
images must have stable locations and naming rules. Feature-specific media may
remain colocated with the feature that owns it.

The library covers repository-owned static media only. User uploads, generated
artifacts and private storage URLs remain backend-owned data and must never be
copied into this library.

## Approaches Considered

### One global `public/assets` directory

Simple to discover, but it eventually mixes global media with files used by a
single component and becomes difficult to maintain.

### Colocate every asset with its component

Makes component ownership clear, but duplicates shared icons and brand files
and makes editorial images harder to find.

### Hybrid library

Use a shared library for reusable and editorial assets, colocate assets that
belong to exactly one feature, and represent interactive SVG icons as typed
React components. This is the selected approach.

## Directory Structure

```text
web/platform/
  public/
    assets/
      brand/
        logos/
        marks/
      icons/
        models/
      illustrations/
        empty-states/
        onboarding/
      images/
        inspiration/
        models/
        tools/
        articles/
  src/
    assets/
      asset-paths.ts
    components/
      icons/
        <IconName>/
          <IconName>.tsx
          <IconName>.test.tsx
          index.ts
    features/
      <feature>/
        <Component>/
          <Component>.tsx
          <Component>.module.css
          assets/
```

`public/assets` contains files that need stable public URLs or are reused by
multiple surfaces. A feature-local `assets` directory is allowed only when the
file has one clear owner and no reuse is expected.

## Asset Rules

- Use SVG for trusted repository-owned icons, logos and simple illustrations.
- Interactive or theme-aware SVG icons are React components using
  `currentColor`; they must expose accessible labels through their consumer.
- Use AVIF or WebP for photographic content where practical. PNG is reserved
  for assets that require its transparency or compatibility characteristics.
- Use lowercase kebab-case names that describe purpose, for example
  `copy-message.svg` and `empty-files.webp`. Names such as `icon-12.svg` are
  forbidden.
- Do not add separate light and dark files when CSS, `currentColor` or an SVG
  mask can provide the theme variation. Theme-specific files are allowed only
  when the artwork itself is different.
- Do not create a single barrel that imports every raster asset into the client
  bundle. Consumers use direct imports or stable public paths.
- Render large raster media through `next/image` with explicit dimensions,
  responsive sizes and lazy loading outside the initial viewport.
- Decorative images use empty alternative text. Meaningful images receive
  localized alternative text at the usage site; accessibility text is not
  encoded in the file name.

## Typed Path Catalog

Stable shared public URLs are exposed through a small server/client-safe
catalog in `src/assets/asset-paths.ts`. Components must not repeat raw paths
such as `/assets/images/inspiration/...` throughout the codebase.

The catalog is grouped by domain and declared `as const`. It stores paths only;
it does not eagerly import images or contain private URLs, user data, pricing or
backend-owned artifact metadata.

Feature-local media may use direct static imports because its ownership and
bundle boundary are already explicit.

## Security Boundary

- Only trusted, reviewed SVG files may be committed. SVGs containing scripts,
  event handlers, embedded remote content or unapproved external references are
  rejected.
- Runtime user uploads and provider outputs are untrusted artifacts. They are
  served through owner-checked backend routes or approved short-lived URLs and
  never imported as application assets.
- The asset catalog contains no secrets, tokens, PII or raw storage/provider
  URLs.
- Components do not render arbitrary SVG or HTML supplied by users.

## Initial Migration

The existing file
`public/inspiration/paper-crane-cloud.png` moves to
`public/assets/images/inspiration/paper-crane-cloud.png`. Its current consumers
are switched to the typed path catalog in one atomic change so no broken URL is
deployed.

No other UI or visual styling changes are part of this migration. Future assets
adopt the structure when they are introduced; unrelated files are not moved
merely for cleanup.

## Validation and Tests

- Unit-test the typed catalog paths used by current components.
- Keep rendering tests for the inspiration gallery and workspace landing page
  so the moved image URL remains valid.
- Add a repository validation script that checks duplicate paths, naming rules,
  allowed extensions and unsafe SVG constructs.
- Run validation in the frontend test/pre-push path without downloading or
  decoding every large image.
- Verify frontend lint, typecheck, focused tests and production build before
  deployment.

## Non-goals

- Building a media CMS or DAM.
- Moving user uploads or generated artifacts into the frontend repository.
- Redesigning existing components.
- Adding an icon package dependency.
- Re-encoding all historical images in the first migration.

## Completion Criteria

- The shared directory structure and typed path catalog exist.
- The existing inspiration image is migrated without a broken URL.
- At least one theme-aware SVG icon follows the component convention when a
  suitable current icon is migrated; otherwise no artificial example asset is
  added.
- Asset validation is automated and documented.
- Existing `/app` behavior and appearance remain unchanged.
