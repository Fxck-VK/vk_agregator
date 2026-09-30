import { classifyReadError, ReadError } from "./web-api/read-error";
export type ReadSnapshot<T> = Readonly<{ data: T | null; pending: boolean; failed: boolean; updatedAt: number; error: ReadError | null }>;

// One owner, one in-flight read. A failed refresh keeps the last good snapshot.
// Private resources must be created inside the account-scoped provider.
export function createReadResource<T>(loader: (signal: AbortSignal) => Promise<T>, initial: T | null = null, ttlMs = 60_000) {
  let snapshot: ReadSnapshot<T> = { data: initial, pending: false, failed: false, updatedAt: initial === null ? 0 : Date.now(), error: null };
  const serverSnapshot = snapshot;
  const listeners = new Set<() => void>();
  let request: AbortController | null = null;
  let inFlight: Promise<void> | null = null;
  let revision = 0;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let failures = 0;
  let nextAllowedAt = 0;
  const active = () => typeof document === "undefined" || document.visibilityState !== "hidden" && navigator.onLine !== false;
  const cancelRetry = () => { if (retryTimer !== null) clearTimeout(retryTimer); retryTimer = null; };
  const scheduleRetry = () => {
    cancelRetry();
    if (!snapshot.error?.retryable) return;
    const delay = [2000, 5000, 15000, 30000, 60000][Math.min(Math.max(failures - 1, 0), 4)];
    retryTimer = setTimeout(() => {
      retryTimer = null;
      if (active()) void resource.load(true); else scheduleRetry();
    }, Math.max(delay + Math.floor(Math.random() * delay * .15), nextAllowedAt - Date.now()));
  };
  const publish = (next: ReadSnapshot<T>) => { snapshot = next; listeners.forEach(listener => listener()); };
  const resource = {
    getSnapshot: () => snapshot,
    getServerSnapshot: () => serverSnapshot,
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    load(force = false): Promise<void> {
      if (inFlight) return inFlight;
      if (Date.now() < nextAllowedAt) return Promise.resolve();
      if (!force && snapshot.data !== null && Date.now() - snapshot.updatedAt < ttlMs) return Promise.resolve();
      const controller = new AbortController();
      cancelRetry();
      const currentRevision = ++revision;
      request = controller;
      publish({ ...snapshot, pending: true, failed: false });
      inFlight = Promise.resolve().then(() => loader(controller.signal)).then(data => {
        if (currentRevision === revision) { failures = 0; nextAllowedAt = 0; publish({ data, pending: false, failed: false, updatedAt: Date.now(), error: null }); }
      }).catch(cause => {
        if (currentRevision === revision && !controller.signal.aborted) {
          const error = classifyReadError(cause);
          nextAllowedAt = Date.now() + (error.retryAfterMs ?? 0);
          publish({ ...snapshot, pending: false, failed: true, error });
          failures++; scheduleRetry();
        }
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
    seed(data: T) { if (snapshot.data === null) publish({ ...snapshot, data }); },
    resume() {
      if (typeof navigator !== "undefined" && navigator.onLine === false && !inFlight && !snapshot.error) {
        publish({ ...snapshot, pending: false, failed: true, error: new ReadError("network") });
        scheduleRetry();
      }
      if (active() && (!snapshot.error || snapshot.error.retryable)) return resource.load(snapshot.failed);
      return Promise.resolve();
    },
    dispose() { revision++; cancelRetry(); request?.abort(); request = null; inFlight = null; listeners.clear(); },
  };
  return resource;
}
