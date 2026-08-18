# Content and localization design

## Objective

Create the second public-platform foundation chapter: a typed, validated,
server-only content layer for models, tools, and articles, plus an exact Russian
and English dictionary contract. This chapter prepares future public route
templates without adding those templates or changing the authenticated `/app`
experience.

## Scope

This chapter includes:

- locale and public-dictionary contracts for `ru` and `en`;
- domain types and Zod schemas for model, tool, and article content;
- schema-validated repository-owned seed content;
- a server-only repository API for localized list and slug lookup operations;
- publication, relationship, slug, and translation validation;
- automated contract tests and documentation.

This chapter excludes:

- public model, tool, guide, article, comparison, pricing, or catalog pages;
- locale routing, redirects, canonical URLs, `hreflang`, JSON-LD, sitemaps, or
  other route-level SEO output;
- a CMS, editorial UI, database, search index, or browser-side content API;
- changes to the authenticated `/app` UI or its existing Russian workspace
  dictionary;
- runtime model availability, prices, balances, limits, provider details, or
  other backend-owned facts.

## Data ownership boundaries

Three data classes remain separate:

1. **UI dictionaries** contain interface labels, actions, errors, and
   accessibility text.
2. **Editorial content** contains original NeiroHub descriptions, use cases,
   limitations, instructions, and related-content relationships.
3. **Runtime product facts** contain availability, star prices, capabilities,
   balances, limits, and job state. The Go backend remains the source of truth
   for these facts.

Editorial model records may hold an optional stable runtime catalog key solely
for joining server-fetched facts later. They must not copy volatile prices,
availability, provider metadata, or account state into repository content.

## Locale strategy

The supported locale contract is initially:

```ts
export const locales = ["ru", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "ru";
```

Future routes will use unprefixed Russian URLs and `/en/...` English URLs. Route
implementation is deferred, but content records support locale-specific current
and legacy slugs now so future route work does not require a data migration.

UI dictionaries and editorial content use separate localization contracts:

- every public UI dictionary must have exactly the same keys in both locales;
- draft editorial records may have incomplete translations;
- reviewed or published records must contain valid Russian and English
  translations;
- repository reads never silently combine languages in one localized result;
- unsupported locales are rejected instead of falling back implicitly.

The current `src/i18n/ru.ts` workspace dictionary remains in place during this
chapter. A new public dictionary namespace is introduced without changing its
imports, preventing a broad `/app` regression. A later, separate migration may
split the workspace dictionary by feature.

## Domain model

All content entities share:

- immutable stable `id` used for internal relationships;
- `kind` discriminator;
- lifecycle status: `draft`, `review`, `published`, or `archived`;
- indexing policy: `index` or `noindex`;
- locale-keyed translations;
- `createdAt`, `updatedAt`, and optional `publishedAt` timestamps;
- author and optional reviewer ownership;
- current localized slug and localized legacy slugs.

Localized metadata includes:

- title;
- short description;
- SEO title and description;
- current slug and legacy slugs.

Model content additionally includes:

- category and original NeiroHub editorial positioning;
- optional stable runtime catalog key;
- localized capabilities, use cases, limitations, and workflow notes;
- related tool and article IDs.

Tool content additionally includes:

- task category;
- related model IDs;
- localized input, output, use-case, limitation, and workflow descriptions;
- an internal authenticated workspace path when the tool is available;
- related article IDs.

Article content additionally includes:

- article category;
- localized, structured text blocks;
- related model and tool IDs;
- publication and last-reviewed dates.

Relationships always use immutable entity IDs, never slugs. This allows slugs
and titles to change without breaking internal references.

## Content representation

The initial adapter stores typed TypeScript records inside the repository. This
keeps the first implementation dependency-light, reviewable in Git, and checked
by both TypeScript and Zod. Long-form authoring or a headless CMS can be added
later behind the same repository interface.

The initial dataset contains one small, original NeiroHub example of each entity
kind. Seed text must not copy StudyAI/Study24 or other competitor content. Seed
records exist to prove validation, localization, relationships, and repository
behavior; they are not rendered publicly in this chapter.

## Validation rules

Zod validation runs over the complete content graph and fails tests/build-time
verification for invalid content. It enforces:

- stable, non-empty, unique IDs;
- lowercase URL-safe slugs and legacy slugs;
- current and legacy slug uniqueness within each route family and locale;
- valid ISO timestamps and chronological lifecycle dates;
- non-empty titles, descriptions, SEO metadata, and body blocks;
- existence and correct target kind for every relationship;
- no duplicate relationship IDs;
- both `ru` and `en` translations for `review` and `published` records;
- author and reviewer ownership for `published` records;
- `publishedAt` and `lastReviewedAt` where publication requires them;
- `index` only for complete `published` records;
- `noindex` for `draft`, `review`, and `archived` records;
- safe internal workspace paths only, never arbitrary external or script URLs;
- no price, balance, availability, secret, provider credential, or account
  fields in editorial schemas.

Validation errors include the entity kind, entity ID, locale, and failing field
where possible so CI failures remain actionable.

## Server-only repository

The repository entry point imports `server-only`. Route components will consume
localized immutable view models rather than raw multilingual records.

The first repository contract provides:

```ts
listModels(locale, options?)
getModelBySlug(locale, slug)
listTools(locale, options?)
getToolBySlug(locale, slug)
listArticles(locale, options?)
getArticleBySlug(locale, slug)
```

Default list operations return published content only. Explicit preview options
may include non-published records on the server, but browser input cannot enable
preview by itself. Lookup returns `undefined` for a syntactically valid missing
slug and rejects an unsupported locale or invalid lookup input.

Returned values contain only the requested locale, stable relationship IDs, and
safe editorial fields. Raw translations, authorship internals not required by a
page, and other locales are not serialized to the browser.

The repository interface is storage-neutral. A future CMS adapter must return
the same domain results and pass the same graph validation before publication.

## Public dictionaries

The public UI dictionary structure is defined once and implemented by both
locale files:

```text
src/i18n/
  locales.ts
  public/
    dictionary.ts
    ru.ts
    en.ts
    get-public-dictionary.ts
```

The English dictionary must satisfy the exact shape of the Russian source
dictionary without widening values to arbitrary strings. A missing or extra key
is a TypeScript and test failure. Dictionary loading is server-oriented and
returns only the selected locale bundle.

The initial public dictionary contains shared navigation, content labels,
publication states, empty states, and accessibility text needed by the future
public templates. It does not translate or move the existing workspace UI in
this chapter.

## Error behavior

- Invalid repository content fails validation before it can be treated as
  published content.
- Missing valid slugs resolve to `undefined`, enabling future routes to return a
  real 404.
- Unsupported locale values fail explicitly.
- Published content never silently falls back to another language.
- No partially validated content is returned when one record fails graph
  validation.

## Security and privacy

- Repository content contains no secrets, credentials, private prompts,
  account data, artifact URLs, or provider-private metadata.
- Content blocks are plain structured text, not trusted HTML.
- Workspace paths are validated as same-origin `/app/...` paths.
- The server-only entry point prevents the raw multilingual catalog and
  editorial metadata from being imported into client components accidentally.
- Runtime price and availability remain backend-owned and must be joined on the
  server in a later page-template chapter.

## Testing strategy

Tests cover:

- exact Russian and English dictionary key parity;
- supported-locale parsing;
- valid seed graph acceptance;
- rejection of duplicate IDs and localized slugs;
- rejection of missing published translations and review metadata;
- rejection of broken and wrong-kind relationships;
- rejection of indexable non-published content;
- rejection of unsafe workspace paths;
- localized list and slug lookup behavior;
- published-only default filtering and explicit server preview behavior;
- missing slug and unsupported locale behavior;
- no regression in the existing `/app` test suite, lint, typecheck, packaging,
  and production build.

## Proposed file layout

```text
web/platform/src/
  content/
    domain/
      types.ts
      schemas.ts
    data/
      models.ts
      tools.ts
      articles.ts
    repository/
      content-repository.ts
      content-validation.ts
    tests/
      content-validation.test.ts
      content-repository.test.ts
  i18n/
    locales.ts
    public/
      dictionary.ts
      ru.ts
      en.ts
      get-public-dictionary.ts
      public-dictionary.test.ts
```

Exact test colocation may be adjusted to match the existing Vitest conventions,
but domain, data, repository, and public dictionaries remain separate.

## Definition of done

The chapter is complete when:

1. model, tool, and article records are strongly typed and graph-validated;
2. Russian and English public dictionaries share an exact contract;
3. localized, server-only list and slug lookup APIs are available;
4. publication rules prevent incomplete or unsafe content from becoming
   indexable;
5. volatile backend-owned facts are absent from editorial records;
6. representative original seed content proves both locales and relationships;
7. focused tests, the full suite, lint, typecheck, packaging, and production
   build pass;
8. existing `/app` behavior and routes are unchanged.
