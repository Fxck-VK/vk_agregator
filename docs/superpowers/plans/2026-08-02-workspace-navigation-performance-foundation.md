# Workspace Navigation Performance Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the authenticated workspace show immediate route feedback and avoid repeated safe model-catalogue requests without touching private-data caching or backend infrastructure.

**Architecture:** Keep the shared `/app` layout server-authenticated. Add a route-segment loading fallback so the existing shell persists while a dynamic child RSC response is pending. Introduce a tab-memory, TTL-bound, single-flight loader for only the validated image-model DTO; both existing model consumers use that loader. Preserve Next client navigation for model-to-generator links.

**Tech Stack:** TypeScript, React, Next.js App Router, CSS Modules, Vitest, Testing Library, Zod, existing `webBrowserFetch`.

## Global Constraints

- Frontend only: do not change Go, database, Redis, API contracts, session/auth, deployment, Cloudflare or CDN configuration.
- Keep the `/app` server layout and its authorization check intact.
- Do not cache profile, conversations, chat messages, jobs, payment data, results or artifacts in this slice.
- Cache only `ImageModelList` in JavaScript memory for one tab; use a 60,000 ms TTL and no persistent browser storage.
- A failed, non-200 or invalid catalogue response must never be cached.
- Preserve existing query encoding, generator explicit-open behaviour, chat mechanics and mobile drawer behaviour.
- Do not add `prefetch={true}` to every private conversation link.
- Use TDD: each behavioural test must be observed failing before its implementation is written.

---

### Task 1: Add a route-level immediate workspace fallback

**Files:**

- Create: `web/platform/src/app/app/loading.tsx`
- Create: `web/platform/src/app/app/loading.module.css`
- Create: `web/platform/src/app/app/loading.test.tsx`

**Interfaces:**

- Consumes: the existing `/app` layout, which already owns `WorkspaceFrame` and the right `<main>` region.
- Produces: a segment fallback rendered by the App Router while a child route RSC payload is pending.

- [ ] **Step 1: Write the failing fallback test**

Create `loading.test.tsx` using `renderToStaticMarkup` and assert the fallback has exactly one status region and no account/chat/API content:

```tsx
const markup = renderToStaticMarkup(<WorkspaceLoading />);

expect(markup).toContain('role="status"');
expect(markup).toContain(ru.workspace.navigationLoading);
expect(markup).not.toContain("workspace-navigation");
expect(markup).not.toContain("/web/v1/");
```

- [ ] **Step 2: Verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/app/app/loading.test.tsx
```

Expected: FAIL because `loading.tsx` and `ru.workspace.navigationLoading` do not exist.

- [ ] **Step 3: Add the smallest route fallback**

Add `navigationLoading: "Открываем раздел…"` to the existing `ru.workspace` object. Implement the route-special component:

```tsx
import { ru } from "@/i18n/ru";
import styles from "./loading.module.css";

export default function WorkspaceLoading() {
  return (
    <section aria-live="polite" className={styles.loading} role="status">
      <div className={styles.indicator} />
      <p>{ru.workspace.navigationLoading}</p>
    </section>
  );
}
```

Style `.loading` as a neutral centered right-panel surface using only existing
`--color-surface`, `--color-border`, `--color-text-muted`, spacing and radius
tokens. The indicator may use a CSS animation but must respect
`prefers-reduced-motion: reduce`.

- [ ] **Step 4: Verify GREEN**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/app/app/loading.test.tsx src/app/app/layout.test.tsx
```

Expected: both test files pass; layout keeps its existing dynamic/auth
contract.

- [ ] **Step 5: Commit**

```powershell
git add web/platform/src/app/app/loading.tsx web/platform/src/app/app/loading.module.css web/platform/src/app/app/loading.test.tsx web/platform/src/i18n/ru.ts
git commit -m "feat: add workspace route loading feedback"
```

### Task 2: Add a bounded single-flight image-model catalogue loader

**Files:**

- Create: `web/platform/src/features/models/image-model-catalog-cache.ts`
- Create: `web/platform/src/features/models/image-model-catalog-cache.test.ts`

**Interfaces:**

- Consumes: `webBrowserFetch`, `parseImageModelList`, and `ImageModelList`.
- Produces: `loadImageModelCatalog(options?)` plus
  `resetImageModelCatalogCacheForTests()`.

- [ ] **Step 1: Write failing loader tests**

Create a valid DTO fixture and tests for each independent cache rule:

```tsx
it("shares one request between concurrent consumers", async () => {
  const response = deferred<Response>();
  const fetcher = vi.fn(() => response.promise);
  const first = loadImageModelCatalog({ fetcher });
  const second = loadImageModelCatalog({ fetcher });

  expect(fetcher).toHaveBeenCalledOnce();
  expect(fetcher).toHaveBeenCalledWith("/web/v1/image-models");
  response.resolve(Response.json(validCatalogue));
  await expect(Promise.all([first, second])).resolves.toEqual([
    expect.objectContaining({ items: expect.any(Array) }),
    expect.objectContaining({ items: expect.any(Array) }),
  ]);
});

it("reuses a fresh successful catalogue then refetches after 60 seconds", async () => {
  let now = 1_000;
  const fetcher = vi.fn().mockResolvedValue(Response.json(validCatalogue));
  await loadImageModelCatalog({ fetcher, now: () => now });
  await loadImageModelCatalog({ fetcher, now: () => now + 59_999 });
  now += 60_000;
  await loadImageModelCatalog({ fetcher, now: () => now });

  expect(fetcher).toHaveBeenCalledTimes(2);
});

it.each([non200Response, invalidDtoResponse, rejectedRequest])(
  "does not retain a failed catalogue load",
  async (request) => {
    const fetcher = vi.fn().mockImplementationOnce(request).mockResolvedValue(Response.json(validCatalogue));
    await expect(loadImageModelCatalog({ fetcher })).rejects.toThrow();
    await expect(loadImageModelCatalog({ fetcher })).resolves.toEqual(expect.objectContaining({ items: expect.any(Array) }));
    expect(fetcher).toHaveBeenCalledTimes(2);
  },
);
```

Reset the module cache in `afterEach` so each test starts isolated.

- [ ] **Step 2: Verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/models/image-model-catalog-cache.test.ts
```

Expected: FAIL because the loader module does not exist.

- [ ] **Step 3: Implement the minimal cache**

Implement a module-scoped record:

```tsx
const imageModelCatalogueTtlMs = 60_000;
let cached: { expiresAt: number; value: ImageModelList } | null = null;
let inFlight: Promise<ImageModelList> | null = null;
```

`loadImageModelCatalog` must return `cached.value` only while
`now() < expiresAt`; otherwise reuse `inFlight` if present, or request
`/web/v1/image-models`, require status 200, validate with
`parseImageModelList`, then write the cache after validation succeeds. Clear
`inFlight` in `finally`; leave `cached` unset on every failure. The options
object is test-only dependency injection with these exact optional fields:

```tsx
type ImageModelCatalogueLoadOptions = {
  fetcher?: typeof webBrowserFetch;
  now?: () => number;
};
```

`resetImageModelCatalogCacheForTests` clears both module variables. Do not
export mutable cache state.

- [ ] **Step 4: Verify GREEN**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/models/image-model-catalog-cache.test.ts src/lib/web-api/contracts.test.ts
```

Expected: all tests pass and invalid DTO validation remains Zod-backed.

- [ ] **Step 5: Commit**

```powershell
git add web/platform/src/features/models/image-model-catalog-cache.ts web/platform/src/features/models/image-model-catalog-cache.test.ts
git commit -m "feat: cache image model catalogue in memory"
```

### Task 3: Connect existing model consumers to the shared loader

**Files:**

- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx`

**Interfaces:**

- Consumes: `loadImageModelCatalog()` from Task 2.
- Produces: model catalogue and generator both use the same validated, scoped
  cache; catalogue cards navigate via Next `Link`.

- [ ] **Step 1: Write failing integration tests**

Mock `loadImageModelCatalog` rather than `webBrowserFetch` in each component
test. In the catalogue suite, mock `next/link` with a visible marker and
prove the selected model uses it:

```tsx
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => (
    <a data-next-link="true" href={href} {...props}>{children}</a>
  ),
}));

expect(await screen.findByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` }))
  .toHaveAttribute("data-next-link", "true");
```

In the generator test, click the existing explicit open button and expect the
loader to be called once, preserving known-model selection and first-model
fallback tests. In the catalogue test, mount, unmount, remount and assert
the component asks the shared loader both times; the loader unit test owns
the deduplication proof.

- [ ] **Step 2: Verify RED**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/models/ModelsCatalog/ModelsCatalog.test.tsx src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx
```

Expected: FAIL because components still call `webBrowserFetch` directly and
cards are native anchors.

- [ ] **Step 3: Make the minimal component changes**

- In `ModelsCatalog`, import `Link` from `next/link`, remove direct
  `parseImageModelList` and `webBrowserFetch` imports, and call
  `loadImageModelCatalog()` in the existing effect.
- Render the existing card CTA as:

```tsx
<Link
  aria-label={`${ru.modelsCatalog.openGeneratorLabel}: ${model.name}`}
  href={`/app/image?model=${encodeURIComponent(model.id)}`}
>
  {ru.modelsCatalog.openGeneratorLabel}
</Link>
```

- In `ImageGenerationPanel`, remove direct image-model request parsing and
  call `loadImageModelCatalog()` only inside the existing explicit
  `openGenerator` action. Keep its closed-on-first-render, known query id
  lookup and unknown-id fallback exactly as they are.

- [ ] **Step 4: Verify GREEN and regression coverage**

Run:

```powershell
npm.cmd --prefix web/platform test -- --run src/features/models/image-model-catalog-cache.test.ts src/features/models/ModelsCatalog/ModelsCatalog.test.tsx src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx
```

Expected: cache rules, catalogue facts/filtering/encoded link and generator
selection flows all pass.

- [ ] **Step 5: Commit**

```powershell
git add web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx
git commit -m "feat: reuse cached image models across workspace views"
```

## Final verification

- [ ] Run `npm.cmd --prefix web/platform test -- --run`.
- [ ] Run `npm.cmd --prefix web/platform run typecheck`, `lint`, `build`, and `test:packaging`.
- [ ] Run `git diff --check` from the pre-slice base through HEAD.
- [ ] Run an independent review for each task and a whole-delivery review; resolve every Critical or Important finding.
- [ ] Push `dev-deploy`, wait for CI and signed images, run the existing DEV deployment and smoke-check the protected DEV address.
