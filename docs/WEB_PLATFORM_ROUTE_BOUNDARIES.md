# Web Platform Route Boundaries

Status: active architecture contract

## Ownership

`web/platform` is one Next.js application with explicit public, authentication,
private product, and same-origin API boundaries:

```text
web/platform/src/app/(public) -> public/indexable pages; route group is absent from URLs
web/platform/src/app/app      -> authenticated product pages under /app
web/platform/src/app/login    -> public authentication entry; noindex
web/platform/src/app/web/v1   -> same-origin BFF proxy; not a page surface
```

## Invariants

- Route-group folder names organize source ownership and never appear in URLs.
- The public layout permits indexing; individual future public templates own
  their titles, descriptions, canonicals, and structured data.
- The `/app` layout verifies the server session before rendering account data,
  forces dynamic rendering, disables revalidation, and emits
  `noindex, nofollow` as defense in depth.
- Authentication, not `robots.txt` or metadata, is the privacy boundary.
- Prompts, responses, files, balances, payment state, session state, and private
  artifact URLs must not enter public HTML, metadata, sitemaps, or analytics.
- The same-origin `/web/v1` proxy remains a server boundary and is never an
  indexable page family.
- Public model, tool, prompt, guide, comparison, pricing, and editorial pages
  may be added later without restructuring or redesigning the existing `/app`.

## Current URL Contract

The first boundary change preserves these URLs and behaviors:

```text
/      -> existing public home page
/login -> existing authentication page
/app   -> existing authenticated workspace
```

The public home page moved only in the source tree, from `src/app/page.tsx` to
`src/app/(public)/page.tsx`. Next.js route groups are URL-transparent, so no
redirect, rewrite, or client navigation change is introduced.
