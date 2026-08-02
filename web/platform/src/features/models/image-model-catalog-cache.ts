import { webBrowserFetch } from "@/lib/web-api/browser";
import { parseImageModelList, type ImageModelList } from "@/lib/web-api/contracts";

const imageModelCatalogueTtlMs = 60_000;

type ImageModelCatalogueLoadOptions = {
  fetcher?: typeof webBrowserFetch;
  now?: () => number;
};

let cached: { expiresAt: number; value: ImageModelList } | null = null;
let inFlight: Promise<ImageModelList> | null = null;

export function loadImageModelCatalog(options: ImageModelCatalogueLoadOptions = {}): Promise<ImageModelList> {
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
      const response = await fetcher("/web/v1/image-models");
      if (response.status !== 200) {
        throw new Error("Unable to load image models.");
      }

      const value = parseImageModelList(await response.clone().json());
      cached = { expiresAt: now() + imageModelCatalogueTtlMs, value };
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

export function resetImageModelCatalogCacheForTests(): void {
  cached = null;
  inFlight = null;
}
