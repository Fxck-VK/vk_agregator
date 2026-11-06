"use client";
import { createContext, useContext, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useRouter } from "@/i18n/navigation";
import { allowsReleasePath, featureEnabled, type ReleaseState, type ReleaseView } from "./policy";
import { clearModelCatalogCache, synchronizeModelCatalogView } from "@/features/models/model-catalog-cache";

const safeState: ReleaseState = { view: "prod", canSwitch: false, localSimulation: false };
type ReleaseContextValue = ReleaseState & { enabled(id: string): boolean; allowsPath(href: string): boolean; switching: boolean; error: boolean; changeView(view: ReleaseView): Promise<boolean> };
const safeContext: ReleaseContextValue = { ...safeState, enabled: id => featureEnabled(id, "prod"), allowsPath: href => allowsReleasePath(href, "prod"), switching: false, error: false, changeView: async () => false };
const ReleaseContext = createContext<ReleaseContextValue>(safeContext);

export function ReleaseViewProvider({ initial, children }: { initial: ReleaseState; children: ReactNode }) {
  const router = useRouter();
  const [switching, setSwitching] = useState(false), [error, setError] = useState(false);
  const [confirmed, setConfirmed] = useState<{ source: ReleaseState; view: ReleaseView } | null>(null);
  const pending = useRef(false);
  const view = confirmed?.source === initial ? confirmed.view : initial.view;
  useLayoutEffect(() => { synchronizeModelCatalogView(view); }, [view]);
  const value = useMemo<ReleaseContextValue>(() => ({ ...initial, view, switching, error,
    enabled: id => featureEnabled(id, view), allowsPath: href => allowsReleasePath(href, view),
    changeView: async nextView => {
      if (!initial.canSwitch || pending.current) return false;
      pending.current = true;
      setSwitching(true); setError(false);
      try {
        const response = await fetch("/web/release-view", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ view: nextView }), cache: "no-store" });
        if (!response.ok) throw new Error("Release view unavailable");
        const result: unknown = await response.json();
        if (!result || typeof result !== "object" || !("view" in result) || result.view !== nextView) throw new Error("Invalid release view");
        clearModelCatalogCache(); setConfirmed({ source: initial, view: nextView });
        router.refresh(); return true;
      } catch { setError(true); return false; }
      finally { pending.current = false; setSwitching(false); }
    },
  }), [initial, view, switching, error, router]);
  return <ReleaseContext.Provider value={value}>{children}</ReleaseContext.Provider>;
}
export function useReleaseView() { return useContext(ReleaseContext); }
