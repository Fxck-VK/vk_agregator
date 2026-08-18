# Content And Localization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a schema-validated, server-only, Russian/English content foundation for public model, tool, and article pages without changing `/app`.

**Architecture:** Keep public UI dictionaries, editorial content, and backend-owned runtime facts as separate contracts. Store initial editorial records in typed repository files, validate the complete relationship graph with Zod, and expose only one requested locale through a `server-only` repository interface that can later be backed by a CMS.

**Tech Stack:** TypeScript 5.9, Next.js 16 App Router, `server-only`, Zod 4, Vitest 4.

## Global Constraints

- Do not change the behavior, routes, or existing Russian dictionary imports of `/app`.
- Do not put prices, balances, availability, limits, provider-private metadata, account state, secrets, private prompts, or artifact URLs in editorial content.
- Runtime model facts remain backend-owned and may be joined later only by a stable optional runtime catalog key.
- Reviewed and published content requires both `ru` and `en`; no mixed-language fallback is allowed.
- Raw multilingual records and preview controls remain server-only.
- Content blocks are structured plain text, never trusted HTML.
- Do not commit or push unless the user explicitly requests it.

---

### Task 1: Public locale and exact dictionary contracts

**Files:**
- Create: `web/platform/src/i18n/locales.ts`
- Create: `web/platform/src/i18n/public/dictionary.ts`
- Create: `web/platform/src/i18n/public/ru.ts`
- Create: `web/platform/src/i18n/public/en.ts`
- Create: `web/platform/src/i18n/public/get-public-dictionary.ts`
- Test: `web/platform/src/i18n/public/public-dictionary.test.ts`

**Interfaces:**
- Produces: `locales`, `Locale`, `defaultLocale`, `parseLocale(input)`.
- Produces: `PublicDictionary`, `publicDictionaryRu`, `publicDictionaryEn`.
- Produces: `getPublicDictionary(input): Promise<PublicDictionary>` from a `server-only` module.

- [x] **Step 1: Write the failing locale and dictionary tests**

```ts
vi.mock("server-only", () => ({}));

it("accepts only the supported locales", () => {
  expect(parseLocale("ru")).toBe("ru");
  expect(parseLocale("en")).toBe("en");
  expect(() => parseLocale("de")).toThrow("Unsupported locale");
});

it("keeps Russian and English public dictionary keys identical", () => {
  expect(flattenKeys(publicDictionaryEn)).toEqual(flattenKeys(publicDictionaryRu));
});

it("loads only a supported public dictionary", async () => {
  await expect(getPublicDictionary("en")).resolves.toEqual(publicDictionaryEn);
  await expect(getPublicDictionary("de")).rejects.toThrow("Unsupported locale");
});
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/i18n/public/public-dictionary.test.ts`

Expected: FAIL because locale and public-dictionary modules do not exist.

- [x] **Step 3: Add the minimal locale and exact dictionary implementation**

```ts
export const locales = ["ru", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "ru";

export function parseLocale(input: unknown): Locale {
  if (input === "ru" || input === "en") return input;
  throw new Error("Unsupported locale");
}
```

```ts
type DictionaryShape<T> = T extends string
  ? string
  : T extends readonly unknown[]
    ? { readonly [K in keyof T]: DictionaryShape<T[K]> }
    : T extends object
      ? { readonly [K in keyof T]: DictionaryShape<T[K]> }
      : never;

export type PublicDictionary = DictionaryShape<typeof publicDictionaryRu>;
```

Define original public navigation, content-type, publication-state, empty-state,
and accessibility labels in Russian and English. Make the English object
`satisfies PublicDictionary`. Keep the existing `src/i18n/ru.ts` unchanged.

Mark `get-public-dictionary.ts` with `import "server-only"`, parse the input,
and dynamically import only the requested locale module.

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `npm test -- src/i18n/public/public-dictionary.test.ts`

Expected: PASS with locale rejection, dictionary parity, and locale loading verified.

---

### Task 2: Editorial domain schemas and lifecycle rules

**Files:**
- Create: `web/platform/src/content/domain/schemas.ts`
- Create: `web/platform/src/content/domain/types.ts`
- Test: `web/platform/src/content/domain/schemas.test.ts`

**Interfaces:**
- Produces: `modelContentSchema`, `toolContentSchema`, `articleContentSchema`, and `contentGraphSchema`.
- Produces inferred `ModelContent`, `ToolContent`, `ArticleContent`, `ContentGraph`, and structured block types.

- [x] **Step 1: Write failing schema tests for lifecycle and safe fields**

```ts
it("accepts a complete bilingual published model", () => {
  expect(modelContentSchema.safeParse(validPublishedModel).success).toBe(true);
});

it("rejects published content without both translations and reviewer metadata", () => {
  expect(modelContentSchema.safeParse(incompletePublishedModel).success).toBe(false);
});

it("rejects indexable non-published content", () => {
  expect(modelContentSchema.safeParse({ ...validDraftModel, indexing: "index" }).success).toBe(false);
});

it("rejects unsafe workspace paths", () => {
  expect(toolContentSchema.safeParse({ ...validTool, workspacePath: "https://example.com" }).success).toBe(false);
});
```

Also assert lowercase URL-safe current/legacy slugs, timestamp ordering, unique
relationship arrays, structured plain-text article blocks, and strict rejection
of unrecognized fields such as `price` and `balance`.

- [x] **Step 2: Run the schema tests and verify RED**

Run: `npm test -- src/content/domain/schemas.test.ts`

Expected: FAIL because the schemas do not exist.

- [x] **Step 3: Implement strict discriminated content schemas**

Use Zod strict objects, a `kind` discriminator, `z.string().datetime({ offset:
true })`, URL-safe slug regexes, safe `/app/...` path validation, and structured
article block discriminated unions. Add record-level `superRefine` rules:

```ts
function validateLifecycle(record: LifecycleRecord, context: z.RefinementCtx) {
  if (record.status !== "published" && record.indexing !== "noindex") {
    context.addIssue({ code: "custom", path: ["indexing"], message: "Only published content may be indexable" });
  }

  if ((record.status === "review" || record.status === "published")
      && (!record.translations.ru || !record.translations.en)) {
    context.addIssue({ code: "custom", path: ["translations"], message: "Review and published content requires ru and en" });
  }
}
```

Published records additionally require reviewer, `publishedAt`, and
`lastReviewedAt`. Infer exported TypeScript types from the schemas so runtime
validation and compile-time contracts cannot drift.

- [x] **Step 4: Run the schema tests and verify GREEN**

Run: `npm test -- src/content/domain/schemas.test.ts`

Expected: PASS for valid records and all rejection cases.

---

### Task 3: Seed content and whole-graph validation

**Files:**
- Create: `web/platform/src/content/data/models.ts`
- Create: `web/platform/src/content/data/tools.ts`
- Create: `web/platform/src/content/data/articles.ts`
- Create: `web/platform/src/content/data/content.ts`
- Create: `web/platform/src/content/repository/content-validation.ts`
- Test: `web/platform/src/content/repository/content-validation.test.ts`

**Interfaces:**
- Consumes: `ContentGraph` and entity schemas from Task 2.
- Produces: `rawContentGraph` and `validateContentGraph(input): ContentGraph`.

- [x] **Step 1: Write failing graph-validation tests**

```ts
it("accepts the repository seed graph", () => {
  expect(validateContentGraph(rawContentGraph)).toEqual(rawContentGraph);
});

it("rejects duplicate IDs and localized current or legacy slugs", () => {
  expect(() => validateContentGraph(graphWithDuplicateSlug)).toThrow(/duplicate.*slug/i);
});

it("rejects missing and wrong-kind relationships", () => {
  expect(() => validateContentGraph(graphWithBrokenRelationship)).toThrow(/relatedToolIds/i);
});
```

- [x] **Step 2: Run the graph tests and verify RED**

Run: `npm test -- src/content/repository/content-validation.test.ts`

Expected: FAIL because graph data and validation do not exist.

- [x] **Step 3: Implement original bilingual seed content**

Add at least one model, one tool, and one article with original NeiroHub copy,
stable semantic IDs, bilingual localized slugs, SEO metadata, relationships by
ID, and no volatile runtime facts. Use an optional `runtimeCatalogKey` only
where a real stable backend key is known; otherwise omit it.

- [x] **Step 4: Implement graph validation**

Parse the graph through `contentGraphSchema`, then validate in deterministic
passes:

```ts
const indexes = buildEntityIndexes(graph);
assertUniqueIds(indexes);
assertUniqueLocalizedSlugs(graph);
assertRelationshipsExistWithExpectedKind(graph, indexes);
return graph;
```

Errors identify entity kind, ID, locale, relation field, and conflicting slug.
Do not return a partial graph when any record is invalid.

- [x] **Step 5: Run the graph tests and verify GREEN**

Run: `npm test -- src/content/repository/content-validation.test.ts`

Expected: PASS for the seed graph and all duplicate/broken relationship cases.

---

### Task 4: Server-only localized repository

**Files:**
- Create: `web/platform/src/content/repository/content-repository.ts`
- Test: `web/platform/src/content/repository/content-repository.test.ts`

**Interfaces:**
- Consumes: validated repository-owned content from Task 3 and `Locale` from Task 1.
- Produces: `listModels`, `getModelBySlug`, `listTools`, `getToolBySlug`, `listArticles`, and `getArticleBySlug`.
- Produces localized immutable view models that contain only one locale.

- [x] **Step 1: Write failing localized repository tests**

```ts
vi.mock("server-only", () => ({}));

it("returns only the requested locale and published records by default", () => {
  const items = listModels("en");
  expect(items[0]).toMatchObject({ locale: "en", title: expect.any(String) });
  expect(items[0]).not.toHaveProperty("translations");
});

it("finds current and legacy localized slugs and exposes the canonical slug", () => {
  const result = getModelBySlug("en", "legacy-image-model");
  expect(result?.slug).toBe("gpt-image-2");
});

it("returns undefined for a valid missing slug and rejects unsupported locales", () => {
  expect(getArticleBySlug("ru", "missing")).toBeUndefined();
  expect(() => listTools("de")).toThrow("Unsupported locale");
});
```

Include tests for deterministic ordering, immutable returned collections, and
explicit server preview inclusion of drafts without changing default behavior.

- [x] **Step 2: Run the repository tests and verify RED**

Run: `npm test -- src/content/repository/content-repository.test.ts`

Expected: FAIL because the repository module does not exist.

- [x] **Step 3: Implement the minimal server-only repository**

At the module top:

```ts
import "server-only";
```

Validate the complete repository graph once at module initialization. Localize
records into new immutable view models, filter to `published` by default, and
support an explicit `{ includeUnpublished: true }` server option. Match current
and legacy slugs but always return the current canonical localized slug. Parse
locale and slug input before lookup.

- [x] **Step 4: Run the repository tests and verify GREEN**

Run: `npm test -- src/content/repository/content-repository.test.ts`

Expected: PASS for localization, publication filtering, slug lookup, and errors.

---

### Task 5: Documentation routing and complete verification

**Files:**
- Modify: `docs/INDEX.md`
- Use: `docs/superpowers/specs/2026-08-17-content-and-localization-design.md`
- Use: `docs/superpowers/plans/2026-08-17-content-and-localization.md`

**Interfaces:**
- Consumes: completed implementation and verification evidence from Tasks 1-4.
- Produces: discoverable documentation and a verified chapter boundary.

- [x] **Step 1: Add task-scoped documentation links**

Add the design and implementation plan under the web-platform public content
scope in `docs/INDEX.md`. Do not re-encode the legacy Windows-1251 architecture
document merely to add a link; its accepted content-operations contract already
defines the server-only schema-validated boundary.

- [x] **Step 2: Run focused content and dictionary tests**

Run:

```text
npm test -- src/i18n/public/public-dictionary.test.ts src/content/domain/schemas.test.ts src/content/repository/content-validation.test.ts src/content/repository/content-repository.test.ts
```

Expected: all focused test files pass with zero failures.

- [x] **Step 3: Run the complete frontend verification suite**

Run from `web/platform`:

```text
npm test
npm run lint
npm run typecheck
npm run test:packaging
npm run build
```

Expected: every command exits `0`; existing `/app` tests and production routes remain unchanged.

- [x] **Step 4: Inspect the final diff and worktree state**

Run:

```text
git diff --check
git diff --stat
git status --short --branch
```

Expected: only the approved public/private boundary, content/localization,
tests, and task-scoped documentation changes are present; no secrets, backend,
billing, provider, storage, or existing `/app` behavior changes appear.
