"use client";
import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { createReadResource } from "@/lib/read-resource";
import { ReadError } from "@/lib/web-api/read-error";
import { recordWorkspaceRead } from "@/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics";
import { loadGenerationModelCatalog, projectGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";
import { adoptServerModelCatalog, bindModelCatalogSession, seedModelCatalog } from "./model-catalog-cache";
import { readModelCatalogSession } from "./model-catalog-session-cache";
import type { ModelCatalogSnapshot, ModelCatalogDelivery } from "./model-catalog-contract";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { filterReleaseCatalog } from "@/features/release/policy";

function createCatalogResource(initial: GenerationModelCatalog | null, source: ModelCatalogSnapshot | null = null) {
  return createReadResource(async () => {
    const result = await loadGenerationModelCatalog({ throwOnError: true });
    if (!result.items.length && Object.keys(result.categoryErrors).length) throw new ReadError("invalid_payload");
    return result;
  }, source ? projectGenerationModelCatalog(source.value) : initial, 60_000, source?.updatedAt);
}
const CatalogContext = createContext<ReturnType<typeof createCatalogResource> | null>(null);

export function GenerationCatalogProvider({ accountId, initial = null, initialSource = null, pendingSource = null, children }: { accountId?: string; initial?: GenerationModelCatalog | null; initialSource?: ModelCatalogSnapshot | null; pendingSource?: Promise<ModelCatalogDelivery> | null; children: ReactNode }) {
  const release = useReleaseView();
  const sourceForView = initialSource && (!initialSource.view || initialSource.view === release.view) ? initialSource : null;
  const [resource] = useState(() => createCatalogResource(initial, sourceForView ? { ...sourceForView, value: filterReleaseCatalog(sourceForView.value, release.view) } : null));
  const seedMetricRecorded = useRef(false);
  const previousView = useRef(release.view);
  const previousSource = useRef(initialSource);
  // Seed once committed in the browser, before child passive effects load raw
  // catalogs. Do not mutate a module cache during server rendering.
  useLayoutEffect(() => {
    const startedAt = performance.now();
    let restoredSeed = false;
    if (accountId) bindModelCatalogSession(accountId, release.view);
    const viewChanged = previousView.current !== release.view;
    const sourceChanged = previousSource.current !== initialSource;
    previousSource.current = initialSource;
    if (viewChanged || sourceChanged && sourceForView && sourceForView.updatedAt >= resource.getSnapshot().updatedAt) {
      previousView.current = release.view; seedMetricRecorded.current = false;
      resource.replaceSeed(sourceForView ? projectGenerationModelCatalog(filterReleaseCatalog(sourceForView.value, release.view)) : null, sourceForView?.updatedAt);
    }
    if (sourceForView) { seedModelCatalog(sourceForView); restoredSeed = true; }
    else {
      const restored = readModelCatalogSession();
      if (restored && (!restored.view || restored.view === release.view)) { const source = { ...restored, value: filterReleaseCatalog(restored.value, release.view) }; seedModelCatalog(source); resource.seed(projectGenerationModelCatalog(source.value), source.updatedAt); restoredSeed = true; }
    }
    if (restoredSeed && !seedMetricRecorded.current) {
      seedMetricRecorded.current = true;
      recordWorkspaceRead({ target: "models", source: "cache", outcome: "success", durationMs: performance.now() - startedAt });
    }
    if (pendingSource) adoptServerModelCatalog(pendingSource);
  }, [accountId, initialSource, sourceForView, pendingSource, resource, release.view]);
  useEffect(() => {
    void resource.resume();
    const refresh = () => { void resource.resume(); };
    window.addEventListener("online", refresh); window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    const timer = window.setInterval(refresh, 60_000);
    return () => { resource.dispose(); window.clearInterval(timer); window.removeEventListener("online", refresh); window.removeEventListener("focus", refresh); document.removeEventListener("visibilitychange", refresh); };
  }, [resource]);
  useEffect(() => {
    if (initialSource?.view && initialSource.view !== release.view) return;
    if (pendingSource) void resource.load(true); else void resource.resume();
  }, [initialSource, pendingSource, resource, release.view]);
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
