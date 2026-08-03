# Fast Private Workspace Navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make fixed private workspace navigation responsive by prefetching only five known routes and reusing bounded account-tab data caches without restoring a global loading screen.

**Architecture:** `WorkspaceFrame` owns a tab-lifetime cache provider that is remounted for each authenticated account. The fixed sidebar opts into full route prefetch while chat links remain on-demand. Files and chat history read a ready value synchronously from this provider, then quietly revalidate through the existing same-origin BFF. DEV-only metrics are kept in `window` memory and do not contain account, prompt, query, or conversation-ID data.

**Tech Stack:** TypeScript, React 19, Next.js App Router, Vitest, Testing Library, CSS Modules.

## Global Constraints

- Keep `/app/**` authenticated, dynamic, no-store, and noindex; do not restore `app/app/loading.tsx`.
- Force prefetch for exactly `/app`, `/app/chats`, `/app/files`, `/app/models`, and `/app/inspiration`; do not prefetch recent chats or all chat histories.
- Cache only in the React tree for one account and one tab. Do not use `localStorage`, `sessionStorage`, cookies, module-global account state, CDN, or a backend cache for the new cache.
- Cache at most eight ready first history pages and one ready 12-item image-job page. Never cache history failure/not-found state, files/artifact bytes, credentials, or private URLs. Safe message text and safe image-job DTO fields already rendered for the current account may remain only in this React-tree cache; they never enter persistent storage or metrics.
- Browser traffic remains same-origin `/web/v1/*`; frontend never calls providers, payments, storage, databases, or VK directly.
- DEV metrics never leave the browser and must retain only bounded anonymous categories plus duration.
- Keep components in their own folders; use CSS Modules only for component-specific visual styles.

---

## File Structure

- `src/features/workspace/WorkspaceDataCache/workspace-data-cache.ts` — pure, bounded LRU data cache interface.
- `src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.tsx` — account-remounted React context provider and hook.
- `src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx` — cache isolation, LRU, and ready-only tests.
- `src/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics.ts` — DEV hostname gate, route categorisation, bounded in-memory timing record functions.
- `src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.tsx` — click/pathname observer that emits navigation measurements.
- `src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx` — anonymous bounded metric behaviour and DEV-only gate tests.
- `src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx` — installs the provider and metrics observer under the persistent app layout.
- `src/components/layout/Sidebar/Sidebar.tsx` — explicit full prefetch only for five fixed links.
- `src/components/layout/Sidebar/Sidebar.test.tsx` — route-budget assertion for the fixed navigation configuration.
- `src/features/files/FilesWorkspace/FilesWorkspace.tsx` — stale-while-revalidate first-page reuse.
- `src/features/files/FilesWorkspace/FilesWorkspace.test.tsx` — cached page is visible before delayed background refresh.
- `src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx` — client initial-history loader with cache-first rendering.
- `src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.test.tsx` — cache hit, cold ready, failure, and replacement behavior.
- `src/features/conversations/conversation-history-contract.ts` — explicit `loading` history state shared by the loader and view.
- `src/app/app/chat/[conversationId]/page.tsx` — thin route that composes the client loader instead of blocking on a server history fetch.

### Task 1: Account-tab cache boundary

**Files:**
- Create: `web/platform/src/features/workspace/WorkspaceDataCache/workspace-data-cache.ts`
- Create: `web/platform/src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.tsx`
- Create: `web/platform/src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx`
- Test: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`

**Interfaces:**

```ts
export const maxCachedConversationHistoryPages = 8;

export type WorkspaceDataCache = {
  getConversationHistory: (conversationId: string) => ReadyConversationHistory | undefined;
  setConversationHistory: (history: ConversationHistoryData) => void;
  getImageFilesFirstPage: () => ImageJobList | undefined;
  setImageFilesFirstPage: (page: ImageJobList) => void;
};

export function createWorkspaceDataCache(): WorkspaceDataCache;
export function WorkspaceDataCacheProvider({ children }: { children: ReactNode }): ReactNode;
export function useWorkspaceDataCache(): WorkspaceDataCache;
```

- [ ] **Step 1: Write the failing cache test**

```tsx
it("keeps only eight most-recent ready histories and ignores unavailable data", () => {
  const cache = createWorkspaceDataCache();
  for (const history of readyHistories(9)) cache.setConversationHistory(history);
  expect(cache.getConversationHistory(readyHistories(1)[0].conversationId)).toBeUndefined();
  cache.setConversationHistory({ kind: "unavailable" });
  expect(cache.getConversationHistory(readyHistories(9).at(-1)!.conversationId)).toBeDefined();
});
```

- [ ] **Step 2: Run the targeted test and verify it fails because the cache module does not exist**

Run: `npm.cmd test -- src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx`

Expected: FAIL with an unresolved module or missing `createWorkspaceDataCache` export.

- [ ] **Step 3: Implement the minimal cache and provider**

```ts
const histories = new Map<string, ReadyConversationHistory>();

function getConversationHistory(id: string) {
  const history = histories.get(id);
  if (history === undefined) return undefined;
  histories.delete(id);
  histories.set(id, history);
  return history;
}

function setConversationHistory(history: ConversationHistoryData) {
  if (history.kind !== "ready") return;
  histories.delete(history.conversationId);
  histories.set(history.conversationId, history);
  while (histories.size > maxCachedConversationHistoryPages) {
    histories.delete(histories.keys().next().value as string);
  }
}
```

Create a provider with `useRef(createWorkspaceDataCache())`; mount it under
the already `key={accountId}` `WorkspaceConversationListProvider` so it is
discarded for another account. Do not put `accountId` into cache records.

- [ ] **Step 4: Run targeted cache and frame tests**

Run: `npm.cmd test -- src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit the cache boundary**

```powershell
git add web/platform/src/features/workspace/WorkspaceDataCache web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx
git commit -m "feat: add account-tab workspace cache"
```

### Task 2: Bounded route prefetch and DEV navigation metrics

**Files:**
- Create: `web/platform/src/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics.ts`
- Create: `web/platform/src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.tsx`
- Create: `web/platform/src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`

**Interfaces:**

```ts
export type WorkspaceMetric =
  | { type: "navigation"; target: WorkspaceMetricTarget; durationMs: number }
  | { type: "data"; target: "files" | "conversation"; source: "cache" | "network"; durationMs: number };

export function recordWorkspaceDataLoad(metric: Extract<WorkspaceMetric, { type: "data" }>): void;
export function beginWorkspaceNavigation(pathname: string): void;
export function completeWorkspaceNavigation(pathname: string): void;
```

- [ ] **Step 1: Write the failing metric and fixed-link configuration tests**

```tsx
it("records no metric on a production hostname", () => {
  setHostname("app.neiirohub.ru");
  recordWorkspaceDataLoad({ type: "data", target: "files", source: "network", durationMs: 12 });
  expect(readWorkspaceMetrics()).toEqual([]);
});

it("marks exactly the five fixed workspace links for full prefetch", () => {
  expect(workspaceNavigationItems.map(({ href, prefetch }) => [href, prefetch])).toEqual([
    ["/app", true], ["/app/chats", true], ["/app/files", true], ["/app/models", true], ["/app/inspiration", true],
  ]);
});
```

- [ ] **Step 2: Run targeted tests and verify expected failure**

Run: `npm.cmd test -- src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx`

Expected: FAIL because the metric module/configuration is absent.

- [ ] **Step 3: Implement privacy-safe metrics and explicit fixed-link prefetch**

```ts
const devHosts = new Set(["localhost", "127.0.0.1", "dev-web.neiirohub.ru"]);
const maxWorkspaceMetrics = 50;

function pushMetric(metric: WorkspaceMetric) {
  if (!devHosts.has(window.location.hostname)) return;
  const metrics = window.__NEIROHUB_WORKSPACE_METRICS__ ?? [];
  metrics.push(metric);
  window.__NEIROHUB_WORKSPACE_METRICS__ = metrics.slice(-maxWorkspaceMetrics);
}
```

The observer listens only for unmodified primary in-app anchor clicks, converts
the path to one fixed route category or `conversation`, and completes a
measurement when `usePathname()` changes. It must never store the clicked URL,
conversation ID, search string, account ID, prompt, or send a request. Install
the observer inside `WorkspaceFrame`. Export `workspaceNavigationItems` from
`Sidebar.tsx` and render fixed `<Link prefetch={item.prefetch}>`; recent chat
links remain untouched.

- [ ] **Step 4: Run targeted tests**

Run: `npm.cmd test -- src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit prefetch and metrics**

```powershell
git add web/platform/src/features/workspace/WorkspaceNavigationMetrics web/platform/src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx web/platform/src/components/layout/Sidebar/Sidebar.tsx web/platform/src/components/layout/Sidebar/Sidebar.test.tsx
git commit -m "feat: prefetch fixed workspace routes"
```

### Task 3: Cache-first Files workspace

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

**Interfaces:**

```ts
const cache = useWorkspaceDataCache();
const cachedFirstPage = cache.getImageFilesFirstPage();
// The existing fetchImageFilesPage(undefined) remains the source of truth.
```

- [ ] **Step 1: Write the failing stale-while-revalidate test**

```tsx
it("renders a cached first file page before delayed revalidation completes", async () => {
  const deferred = createDeferred<Response>();
  vi.mocked(webBrowserFetch).mockReturnValueOnce(deferred.promise);
  render(<CacheProviderWithFiles page={pageWith(firstSucceededJob)}><FilesWorkspace /></CacheProviderWithFiles>);
  expect(screen.getByText(firstSucceededJob.prompt)).toBeVisible();
  expect(screen.queryByText(ru.files.loading)).toBeNull();
  deferred.resolve(Response.json({ items: [secondSucceededJob], has_more: false, next_cursor: null }));
  expect(await screen.findByText(secondSucceededJob.prompt)).toBeVisible();
});
```

- [ ] **Step 2: Run the targeted test and verify it fails**

Run: `npm.cmd test -- src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: FAIL because cached data is not rendered before the first request resolves.

- [ ] **Step 3: Implement the minimal cache-first state initialiser**

```tsx
const cache = useWorkspaceDataCache();
const cachedFirstPage = cache.getImageFilesFirstPage();
const [jobs, setJobs] = useState(() => cachedFirstPage?.items ?? []);
const [nextCursor, setNextCursor] = useState(() => cachedFirstPage?.next_cursor ?? null);
const [hasLoaded, setHasLoaded] = useState(() => cachedFirstPage !== undefined);
```

On successful `loadPage()` with no cursor, call
`cache.setImageFilesFirstPage(page)` before replacing visible first-page jobs.
Keep existing cursor load-more and preview behavior. First-page network errors
must preserve a visible cached page and only set the inline failure state.
Record cache/network timing through `recordWorkspaceDataLoad` without prompt,
job, artifact, or account information.

- [ ] **Step 4: Run scoped tests**

Run: `npm.cmd test -- src/features/files/FilesWorkspace/FilesWorkspace.test.tsx src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit cache-first files**

```powershell
git add web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx
git commit -m "feat: reuse cached files page"
```

### Task 4: Cache-first conversation route

**Files:**
- Create: `web/platform/src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx`
- Create: `web/platform/src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.test.tsx`
- Modify: `web/platform/src/app/app/chat/[conversationId]/page.tsx`
- Modify: `web/platform/src/features/conversations/conversation-history-contract.ts`
- Modify: `web/platform/src/features/conversations/conversation-history-data.ts` only if a shared validator/helper is required; do not duplicate DTO parsing.

**Interfaces:**

```ts
export function ConversationHistoryLoader({
  conversationId,
  initialRefresh = false,
}: {
  conversationId: string;
  initialRefresh?: boolean;
}): ReactNode;
```

- [ ] **Step 1: Write failing loader tests**

```tsx
it("shows a cached ready history before a delayed revalidation resolves", async () => {
  const deferred = createDeferred<Response>();
  vi.mocked(webBrowserFetch).mockReturnValueOnce(deferred.promise);
  render(<CacheProviderWithHistory history={readyHistory}><ConversationHistoryLoader conversationId={conversationId} /></CacheProviderWithHistory>);
  expect(screen.getByText("cached message")).toBeVisible();
  deferred.resolve(Response.json({ items: [freshMessage], has_more_before: false }));
  expect(await screen.findByText("fresh message")).toBeVisible();
});

it("does not cache a not-found response", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 404 }));
  render(<Provider><ConversationHistoryLoader conversationId={conversationId} /></Provider>);
  await screen.findByText(ru.conversations.historyUnavailable);
  expect(cache.getConversationHistory(conversationId)).toBeUndefined();
});
```

- [ ] **Step 2: Run loader tests and verify expected failure**

Run: `npm.cmd test -- src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.test.tsx`

Expected: FAIL because `ConversationHistoryLoader` does not exist.

- [ ] **Step 3: Implement client cache-first fetch with a safe cold state**

```tsx
const cache = useWorkspaceDataCache();
const [history, setHistory] = useState<ConversationHistoryData>(() =>
  cache.getConversationHistory(conversationId) ?? { kind: "loading" },
);

useEffect(() => {
  const controller = new AbortController();
  void loadHistory(conversationId, controller.signal).then((nextHistory) => {
    if (nextHistory.kind === "ready") cache.setConversationHistory(nextHistory);
    setHistory(nextHistory);
  });
  return () => controller.abort();
}, [cache, conversationId]);
```

Validate UUIDs before fetching and parse the existing
`parseConversationMessageList` DTO. Map 404 to `not_found`, other failures to
`unavailable`, and cache only `ready`. Add a `loading` branch to the history
rendering that uses the existing private state styling and leaves the global
workspace shell visible. The server page awaits only route params/search params
and renders this loader; it must no longer await `loadConversationHistory`.
Record cache/network timing by category only.

- [ ] **Step 4: Run conversation and route tests**

Run: `npm.cmd test -- src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.test.tsx src/features/conversations/ConversationHistory/ConversationHistory.test.tsx src/app/app/chat/[conversationId]/page.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit the cache-first conversation route**

```powershell
git add web/platform/src/features/conversations/ConversationHistoryLoader web/platform/src/app/app/chat/[conversationId]/page.tsx web/platform/src/features/conversations/conversation-history-contract.ts web/platform/src/features/conversations/ConversationHistory
git commit -m "feat: cache private chat history"
```

### Task 5: Whole-slice verification and DEV rollout

**Files:**
- Modify if required by review only: files identified by the reviewer.

- [ ] **Step 1: Run the complete platform verification set**

```powershell
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
npm.cmd run test:packaging
```

Expected: every command exits 0.

- [ ] **Step 2: Review the complete branch diff**

Compare the merge base `origin/dev-deploy` through `HEAD` against the global
constraints. Confirm no global loading route was reintroduced, metrics contain
no private values, and chat links do not gain automatic prefetch.

- [ ] **Step 3: Push and deploy the verified branch**

```powershell
git push origin HEAD:dev-deploy
```

Wait for `Docker Images` to pass. Manually dispatch `Deploy DEV` on the
`dev-deploy` ref, wait for the DEV smoke job, and verify it deploys the pushed
commit. Do not deploy `main` or production.

- [ ] **Step 4: Manual browser acceptance check**

At `https://dev-web.neiirohub.ru/app`, verify the shell remains visible when
opening every fixed section, revisiting Files, and revisiting a chat. In the
DEV browser console, inspect `window.__NEIROHUB_WORKSPACE_METRICS__`; its
records must contain only route/resource categories, cache/network source, and
duration.
