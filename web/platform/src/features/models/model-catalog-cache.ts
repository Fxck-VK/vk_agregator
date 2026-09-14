import { webBrowserFetch } from "@/lib/web-api/browser";

import { parseModelCatalog, type PublicCatalog } from "./model-catalog-contract";

const modelCatalogTtlMs = 60_000;

export type ModelCatalogLoadOptions = {
  fetcher?: typeof webBrowserFetch;
  now?: () => number;
};

let cached: { expiresAt: number; value: PublicCatalog } | null = null;
let inFlight: Promise<PublicCatalog> | null = null;

export function loadModelCatalog(options: ModelCatalogLoadOptions = {}): Promise<PublicCatalog> {
  const now = options.now ?? Date.now;
  if (cached !== null && now() < cached.expiresAt) {
    return Promise.resolve(cached.value);
  }
  if (inFlight !== null) {
    return inFlight;
  }

  const fetcher = options.fetcher ?? webBrowserFetch;
  inFlight = (async () => {
    try {
      const response = await fetcher("/web/v1/models");
      if (response.status !== 200) {
        throw new Error("Unable to load model catalog.");
      }
      const value = parseModelCatalog(await response.json());
      cached = { expiresAt: now() + modelCatalogTtlMs, value };
      return value;
    } catch (error) {
      cached = null;
      throw error;
    } finally {
      inFlight = null;
    }
  })();

  return inFlight;
}

export function resetModelCatalogCacheForTests(): void {
  cached = null;
  inFlight = null;
}
