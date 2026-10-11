import "server-only";
import { webServerFetch } from "@/lib/web-api/server";
import { getWebApiInternalOrigin } from "@/lib/web-api/internal-origin";
import { parseModelCatalog, type ModelCatalogSnapshot } from "./model-catalog-contract";
import { isLocalWorkspacePreviewEnabled, localWorkspacePreviewModels } from "@/features/session/local-workspace-preview";

// Only normalized, non-personal catalog data. Call after verifying the session;
// never cache /me, cookies, balances, conversations or authorization responses.
let cached: { origin: string; expires: number; catalog: ModelCatalogSnapshot } | null = null;
let inFlight: { origin: string; promise: Promise<ModelCatalogSnapshot | null> } | null = null;
export async function loadServerModelCatalog(): Promise<ModelCatalogSnapshot | null> {
  if (isLocalWorkspacePreviewEnabled()) return { value: localWorkspacePreviewModels, updatedAt: Date.now() };
  let origin: string;
  try { origin = getWebApiInternalOrigin(); } catch { return null; }
  if (cached?.origin === origin && cached.expires > Date.now()) return cached.catalog;
  if (inFlight?.origin === origin) return inFlight.promise;
  const promise = (async () => {
    try {
      const response = await webServerFetch("/web/v1/models", { signal: AbortSignal.timeout(8000) });
      if (!response.ok) return null;
      const catalog = { value: parseModelCatalog(await response.json()), updatedAt: Date.now() };
      cached = { origin, expires: catalog.updatedAt + 60_000, catalog };
      return catalog;
    } catch { return null; }
    finally { if (inFlight?.origin === origin) inFlight = null; }
  })();
  inFlight = { origin, promise };
  return promise;
}

export async function initialModelCatalog(): Promise<ModelCatalogSnapshot | null> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try { return await Promise.race([loadServerModelCatalog(), new Promise<null>(resolve => { timer = setTimeout(() => resolve(null), 200); })]); }
  finally { clearTimeout(timer); }
}
