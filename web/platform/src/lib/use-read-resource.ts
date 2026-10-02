"use client";
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { createReadResource } from "./read-resource";

/** Mount this within an account-keyed provider for private data. */
export function useReadResource<T>(loader: (signal: AbortSignal) => Promise<T>, initial: T | null, enabled = true, revisionKey?: unknown) {
  const [resource] = useState(() => createReadResource(loader, initial));
  const snapshot = useSyncExternalStore(resource.subscribe, resource.getSnapshot, resource.getServerSnapshot);
  const lastRevision = useRef(revisionKey);
  useEffect(() => {
    if (lastRevision.current === revisionKey) return;
    lastRevision.current = revisionKey;
    if (enabled) void resource.invalidate();
  }, [enabled, resource, revisionKey]);
  useEffect(() => {
    if (!enabled) return;
    void resource.resume();
    const refresh = () => { void resource.resume(); };
    window.addEventListener("online", refresh);
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    const timer = window.setInterval(refresh, 60_000);
    return () => {
      window.removeEventListener("online", refresh);
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
      window.clearInterval(timer);
      resource.dispose();
    };
  }, [enabled, resource]);
  const retry = useCallback(() => { void resource.load(true); }, [resource]);
  return { ...snapshot, pending: snapshot.pending || enabled && snapshot.data === null && !snapshot.failed, retry, seed: resource.seed };
}
