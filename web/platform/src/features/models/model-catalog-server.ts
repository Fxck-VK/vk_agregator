import "server-only";
import { webServerFetch } from "@/lib/web-api/server";
import { getWebApiInternalOrigin } from "@/lib/web-api/internal-origin";
import { parseModelCatalog, type ModelCatalogSnapshot, type ModelCatalogBootstrap } from "./model-catalog-contract";
import { isLocalWorkspacePreviewEnabled, localWorkspacePreviewModels } from "@/features/session/local-workspace-preview";
import { filterReleaseCatalog, type ReleaseView } from "@/features/release/policy";

// Only normalized, non-personal catalog data. Call after verifying the session;
// never cache /me, cookies, balances, conversations or authorization responses.
const cachedByView = new Map<ReleaseView, { origin: string; expires: number; catalog: ModelCatalogSnapshot }>();
const inFlightByView = new Map<ReleaseView, { origin: string; promise: Promise<ModelCatalogSnapshot | null> }>();
export async function loadServerModelCatalog(view: ReleaseView = "prod"): Promise<ModelCatalogSnapshot | null> {
  if (isLocalWorkspacePreviewEnabled()) return { value: filterReleaseCatalog(localWorkspacePreviewModels, view), updatedAt: Date.now(), view };
  let origin: string;
  try { origin = getWebApiInternalOrigin(); } catch { return null; }
  const cached = cachedByView.get(view), inFlight = inFlightByView.get(view);
  if (cached?.origin === origin && cached.expires > Date.now()) return cached.catalog;
  if (inFlight?.origin === origin) return inFlight.promise;
  const promise = (async () => {
    try {
      const response = await webServerFetch("/web/v1/models", { signal: AbortSignal.timeout(8000) });
      if (!response.ok) return null;
      const catalog = { value: filterReleaseCatalog(parseModelCatalog(await response.json()), view), updatedAt: Date.now(), view };
      cachedByView.set(view, { origin, expires: catalog.updatedAt + 60_000, catalog });
      return catalog;
    } catch { return null; }
  })();
  inFlightByView.set(view, { origin, promise });
  void promise.then(() => { if (inFlightByView.get(view)?.promise === promise) inFlightByView.delete(view); });
  return promise;
}

export function initialModelCatalog(view: ReleaseView = "prod"): ModelCatalogBootstrap {
  if (isLocalWorkspacePreviewEnabled()) return { source: { value: filterReleaseCatalog(localWorkspacePreviewModels, view), updatedAt: Date.now(), view }, pending: null };
  let origin: string;
  try { origin = getWebApiInternalOrigin(); } catch { return { source: null, pending: null }; }
  const cached = cachedByView.get(view);
  const source = cached?.origin === origin && Date.now() - cached.catalog.updatedAt <= 15 * 60_000 ? cached.catalog : null;
  if (source && cached!.expires > Date.now()) return { source, pending: null };
  const startedAt = performance.now();
  // React streams the pending prop. A slow read no longer becomes a second
  // browser request; the shell and a last good snapshot can render immediately.
  return { source, pending: loadServerModelCatalog(view).then(snapshot => ({ snapshot, durationMs: Math.max(0, performance.now() - startedAt) })) };
}
