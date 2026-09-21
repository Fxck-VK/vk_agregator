import "server-only";
import { webServerFetch } from "@/lib/web-api/server";
import { getWebApiInternalOrigin } from "@/lib/web-api/internal-origin";
import { parseModelCatalog } from "./model-catalog-contract";
import { projectGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";
import { isLocalWorkspacePreviewEnabled, localWorkspacePreviewModels } from "@/features/session/local-workspace-preview";

// Only normalized, non-personal catalog data. Call after verifying the session;
// never cache /me, cookies, balances, conversations or authorization responses.
let cached: { origin: string; expires: number; catalog: GenerationModelCatalog } | null = null;
let inFlight: { origin: string; promise: Promise<GenerationModelCatalog | null> } | null = null;
export async function loadServerModelCatalog(): Promise<GenerationModelCatalog | null> {
  if (isLocalWorkspacePreviewEnabled()) return projectGenerationModelCatalog(localWorkspacePreviewModels);
  let origin: string;
  try { origin = getWebApiInternalOrigin(); } catch { return null; }
  if (cached?.origin === origin && cached.expires > Date.now()) return cached.catalog;
  if (inFlight?.origin === origin) return inFlight.promise;
  const promise = (async () => {
    try {
      const response = await webServerFetch("/web/v1/models", { signal: AbortSignal.timeout(8000) });
      if (!response.ok) return null;
      const catalog = projectGenerationModelCatalog(parseModelCatalog(await response.json()));
      cached = { origin, expires: Date.now() + 60_000, catalog };
      return catalog;
    } catch { return null; }
    finally { if (inFlight?.origin === origin) inFlight = null; }
  })();
  inFlight = { origin, promise };
  return promise;
}

export async function initialModelCatalog(): Promise<GenerationModelCatalog | null> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try { return await Promise.race([loadServerModelCatalog(), new Promise<null>(resolve => { timer = setTimeout(() => resolve(null), 200); })]); }
  finally { clearTimeout(timer); }
}
