export type ReadSnapshot<T> = Readonly<{ data: T | null; pending: boolean; failed: boolean; updatedAt: number }>;

// One owner, one in-flight read. A failed refresh keeps the last good snapshot.
// Private resources must be created inside the account-scoped provider.
export function createReadResource<T>(loader: (signal: AbortSignal) => Promise<T>, initial: T | null = null, ttlMs = 60_000) {
  let snapshot: ReadSnapshot<T> = { data: initial, pending: false, failed: false, updatedAt: initial === null ? 0 : Date.now() };
  const serverSnapshot = snapshot;
  const listeners = new Set<() => void>();
  let request: AbortController | null = null;
  let inFlight: Promise<void> | null = null;
  let revision = 0;
  const publish = (next: ReadSnapshot<T>) => { snapshot = next; listeners.forEach(listener => listener()); };
  const resource = {
    getSnapshot: () => snapshot,
    getServerSnapshot: () => serverSnapshot,
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    load(force = false): Promise<void> {
      if (inFlight) return inFlight;
      if (!force && snapshot.data !== null && Date.now() - snapshot.updatedAt < ttlMs) return Promise.resolve();
      const controller = new AbortController();
      const currentRevision = ++revision;
      request = controller;
      publish({ ...snapshot, pending: true, failed: false });
      inFlight = Promise.resolve().then(() => loader(controller.signal)).then(data => {
        if (currentRevision === revision) publish({ data, pending: false, failed: false, updatedAt: Date.now() });
      }).catch(() => {
        if (currentRevision === revision && !controller.signal.aborted) publish({ ...snapshot, pending: false, failed: true });
      }).finally(() => { if (currentRevision === revision) { request = null; inFlight = null; } });
      return inFlight;
    },
    invalidate(): Promise<void> {
      revision++;
      request?.abort();
      request = null;
      inFlight = null;
      return resource.load(true);
    },
    dispose() { revision++; request?.abort(); request = null; inFlight = null; listeners.clear(); },
  };
  return resource;
}
