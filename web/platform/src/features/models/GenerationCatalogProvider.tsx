"use client";
import { createContext, useCallback, useContext, useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { createReadResource } from "@/lib/read-resource";
import { ReadError } from "@/lib/web-api/read-error";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";

function createCatalogResource(initial: GenerationModelCatalog | null) {
  return createReadResource(async () => {
    const result = await loadGenerationModelCatalog({ throwOnError: true });
    if (!result.items.length && Object.keys(result.categoryErrors).length) throw new ReadError("invalid_payload");
    return result;
  }, initial);
}
const CatalogContext = createContext<ReturnType<typeof createCatalogResource> | null>(null);

export function GenerationCatalogProvider({ initial = null, children }: { initial?: GenerationModelCatalog | null; children: ReactNode }) {
  const [resource] = useState(() => createCatalogResource(initial));
  useEffect(() => {
    void resource.resume();
    const refresh = () => { void resource.resume(); };
    window.addEventListener("online", refresh); window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    const timer = window.setInterval(refresh, 60_000);
    return () => { resource.dispose(); window.clearInterval(timer); window.removeEventListener("online", refresh); window.removeEventListener("focus", refresh); document.removeEventListener("visibilitychange", refresh); };
  }, [resource]);
  return <CatalogContext.Provider value={resource}>{children}</CatalogContext.Provider>;
}

export function useGenerationCatalog() {
  const shared = useContext(CatalogContext);
  const [fallback] = useState(() => createCatalogResource(null));
  const resource = shared ?? fallback;
  const snapshot = useSyncExternalStore(resource.subscribe, resource.getSnapshot, resource.getServerSnapshot);
  useEffect(() => { if (!shared) { void resource.resume(); return () => resource.dispose(); } }, [resource, shared]);
  const retry = useCallback(() => { void resource.load(true); }, [resource]);
  return { catalog: snapshot.data, pending: snapshot.pending || snapshot.data === null && !snapshot.failed,
    failed: snapshot.failed, status: snapshot.data ? "ready" as const : snapshot.failed ? "failure" as const : "loading" as const,
    retry };
}
