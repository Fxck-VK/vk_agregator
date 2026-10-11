import { webBrowserFetch } from "@/lib/web-api/browser";
import { readJson } from "@/lib/web-api/read-error";

import { parseModelCatalog, type ModelCatalogSnapshot, type PublicCatalog } from "./model-catalog-contract";

const modelCatalogTtlMs = 60_000;

export type ModelCatalogLoadOptions = {
  fetcher?: typeof webBrowserFetch;
  now?: () => number;
};

let cached: ModelCatalogSnapshot | null = null;
let inFlight: { promise: Promise<PublicCatalog>; startedAt: number } | null = null;
let revision = 0;

// The server already validated this non-personal snapshot. Preserve its age;
// a later route seed must not replace a more recent browser result.
export function seedModelCatalog(snapshot: ModelCatalogSnapshot): void {
  const updatedAt = Math.min(snapshot.updatedAt, Date.now());
  if (cached !== null && cached.updatedAt >= updatedAt) return;
  cached = { ...snapshot, updatedAt };
  if (inFlight === null || updatedAt > inFlight.startedAt) revision++;
}

export function loadModelCatalog(options: ModelCatalogLoadOptions = {}): Promise<PublicCatalog> {
  const now = options.now ?? Date.now;
  if (cached !== null && now() < cached.updatedAt + modelCatalogTtlMs) {
    return Promise.resolve(cached.value);
  }
  if (inFlight !== null) {
    return inFlight.promise;
  }

  const fetcher = options.fetcher ?? webBrowserFetch;
  const startedAt = now();
  const requestRevision = revision;
  const request = (async () => {
    try {
      const response = await fetcher("/web/v1/models");
      const value = await readJson(response, parseModelCatalog);
      if (requestRevision !== revision) return cached?.value ?? value;
      cached = { updatedAt: now(), value };
      return value;
    } catch (error) {
      if (requestRevision === revision) cached = null;
      throw error;
    }
  })();
  inFlight = { promise: request, startedAt };
  const settled = () => { if (inFlight?.promise === request) inFlight = null; };
  void request.then(settled, settled);
  return request;
}

export function resetModelCatalogCacheForTests(): void {
  revision++;
  cached = null;
  inFlight = null;
}
