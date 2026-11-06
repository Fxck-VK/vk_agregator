import { webBrowserFetch } from "@/lib/web-api/browser";
import { readJson } from "@/lib/web-api/read-error";
import { measureWorkspaceRead, recordWorkspaceRead } from "@/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics";

import { parseModelCatalog, type ModelCatalogSnapshot, type ModelCatalogDelivery, type PublicCatalog } from "./model-catalog-contract";
import { clearModelCatalogSession, writeModelCatalogSession } from "./model-catalog-session-cache";
import { filterReleaseCatalog, type ReleaseView } from "@/features/release/policy";

const modelCatalogTtlMs = 60_000;

export type ModelCatalogLoadOptions = {
  fetcher?: typeof webBrowserFetch;
  now?: () => number;
};

let cached: ModelCatalogSnapshot | null = null;
let inFlight: { promise: Promise<PublicCatalog>; startedAt: number; source?: Promise<ModelCatalogDelivery> } | null = null;
let revision = 0;
let accountScope: string | null = null;
let releaseView: ReleaseView = "prod";

// Public readers need the current view without inventing an account.
export function synchronizeModelCatalogView(view: ReleaseView): void {
  if (releaseView !== view) { clearModelCatalogCache(); releaseView = view; }
}

/** In-memory epoch only; account identifiers never enter catalogue storage. */
export function bindModelCatalogSession(accountId: string, view: ReleaseView = "prod"): void {
  if (accountScope !== null && (accountScope !== accountId || releaseView !== view)) clearModelCatalogCache();
  accountScope = accountId;
  releaseView = view;
}

// The server already validated this non-personal snapshot. Preserve its age;
// a later route seed must not replace a more recent browser result.
export function seedModelCatalog(snapshot: ModelCatalogSnapshot): void {
  if (snapshot.view && snapshot.view !== releaseView) return;
  const updatedAt = Math.min(snapshot.updatedAt, Date.now());
  if (cached !== null && cached.updatedAt >= updatedAt) return;
  cached = { value: filterReleaseCatalog(snapshot.value, releaseView), updatedAt, view: releaseView };
  writeModelCatalogSession(cached);
  if (inFlight === null || updatedAt > inFlight.startedAt) revision++;
}

export function loadModelCatalog(options: ModelCatalogLoadOptions = {}): Promise<PublicCatalog> {
  const now = options.now ?? Date.now;
  if (inFlight?.source) return inFlight.promise;
  if (cached !== null && now() < cached.updatedAt + modelCatalogTtlMs) {
    recordWorkspaceRead({ target: "models", source: "cache", outcome: "success", durationMs: 0 });
    return Promise.resolve(cached.value);
  }
  if (inFlight !== null) {
    return inFlight.promise;
  }

  const fetcher = options.fetcher ?? webBrowserFetch;
  const startedAt = now();
  const requestRevision = revision;
  const request = requestCatalog(fetcher, now, requestRevision);
  inFlight = { promise: request, startedAt };
  const settled = () => { if (inFlight?.promise === request) inFlight = null; };
  void request.then(settled, settled);
  return request;
}

function currentValue(): PublicCatalog {
  if (cached) return cached.value;
  throw new DOMException("Catalogue read invalidated", "AbortError");
}

async function requestCatalog(fetcher: typeof webBrowserFetch, now: () => number, requestRevision: number): Promise<PublicCatalog> {
  const raw = await measureWorkspaceRead("models", "network", async () => {
    const response = await fetcher("/web/v1/models");
    return readJson(response, parseModelCatalog);
  });
  if (requestRevision !== revision) return currentValue();
  const value = filterReleaseCatalog(raw, releaseView);
  cached = { updatedAt: now(), value, view: releaseView };
  writeModelCatalogSession(cached);
  return value;
}

/** Install before child passive effects so every adapter shares server work. */
export function adoptServerModelCatalog(source: Promise<ModelCatalogDelivery>): void {
  if (inFlight) return;
  const startedAt = Date.now(), requestRevision = revision;
  const startedReadingAt = performance.now();
  const request = (async () => {
    let delivery: ModelCatalogDelivery | null;
    try { delivery = await source; } catch { delivery = null; }
    if (requestRevision !== revision) return currentValue();
    if (delivery?.snapshot?.view && delivery.snapshot.view !== releaseView) delivery = null;
    recordWorkspaceRead({ target: "models", source: "server", outcome: delivery?.snapshot ? "success" : "failure",
      durationMs: delivery?.durationMs ?? performance.now() - startedReadingAt });
    if (delivery?.snapshot) {
      const snapshot = { value: filterReleaseCatalog(delivery.snapshot.value, releaseView), updatedAt: Math.min(delivery.snapshot.updatedAt, Date.now()), view: releaseView };
      if (!cached || snapshot.updatedAt >= cached.updatedAt) {
        cached = snapshot;
        writeModelCatalogSession(snapshot);
      }
      return cached.value;
    }
    // The server read failed; use the existing authenticated browser transport
    // once, not a recursive load which would await its own in-flight promise.
    return requestCatalog(webBrowserFetch, Date.now, requestRevision);
  })();
  inFlight = { promise: request, startedAt, source };
  const settled = () => { if (inFlight?.promise === request) inFlight = null; };
  void request.then(settled, settled);
}

export function clearModelCatalogCache(): void {
  revision++;
  cached = null;
  inFlight = null;
  accountScope = null;
  releaseView = "prod";
  clearModelCatalogSession();
}

export const resetModelCatalogCacheForTests = clearModelCatalogCache;
