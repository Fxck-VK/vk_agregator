"use client";
import { createContext, useCallback, useContext, useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { createReadResource } from "@/lib/read-resource";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";

function createCatalogResource(initial: GenerationModelCatalog | null) {
  return createReadResource(async () => {
    const result = await loadGenerationModelCatalog();
    if (!result.items.length && Object.keys(result.categoryErrors).length) throw new Error("Catalog unavailable");
    return result;
  }, initial);
}
const CatalogContext = createContext<ReturnType<typeof createCatalogResource> | null>(null);

export function GenerationCatalogProvider({ initial = null, children }: { initial?: GenerationModelCatalog | null; children: ReactNode }) {
  const [resource] = useState(() => createCatalogResource(initial));
  useEffect(() => {
    void resource.load();
    const refresh = () => { if (document.visibilityState !== "hidden") void resource.load(resource.getSnapshot().failed); };
    window.addEventListener("online", refresh); window.addEventListener("focus", refresh);
    const timer = window.setInterval(refresh, 60_000);
    return () => { window.clearInterval(timer); window.removeEventListener("online", refresh); window.removeEventListener("focus", refresh); };
  }, [resource]);
  return <CatalogContext.Provider value={resource}>{children}</CatalogContext.Provider>;
}

export function useGenerationCatalog() {
  const shared = useContext(CatalogContext);
  const [fallback] = useState(() => createCatalogResource(null));
  const resource = shared ?? fallback;
  const snapshot = useSyncExternalStore(resource.subscribe, resource.getSnapshot, resource.getServerSnapshot);
  useEffect(() => { if (!shared) void resource.load(); }, [resource, shared]);
  const retry = useCallback(() => { void resource.load(true); }, [resource]);
  return { catalog: snapshot.data, pending: snapshot.pending || snapshot.data === null && !snapshot.failed,
    failed: snapshot.failed, status: snapshot.data ? "ready" as const : snapshot.failed ? "failure" as const : "loading" as const,
    retry };
}
