/** Fallback for browsers without Web Locks: an atomic IndexedDB lease, no credentials. */
export async function withBrowserSessionLock<T>(name: string, signal: AbortSignal, run: () => Promise<T>): Promise<T> {
  if (typeof navigator !== "undefined" && navigator.locks) return await navigator.locks.request(name, { signal }, run);
  if (typeof indexedDB === "undefined") {
    // Server/test runtimes have no tabs. Browser runtimes without either locking
    // primitive must not race rotating credentials.
    if (typeof window === "undefined") return run();
    throw new DOMException("Session coordination unavailable.", "NotSupportedError");
  }
  const db = await new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open("neirohub.session-coordination", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("leases");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(new Error("Session coordination unavailable."));
  });
  const owner = crypto.randomUUID();
  const lease = (release: boolean) => new Promise<boolean>((resolve, reject) => {
    let acquired = false;
    const transaction = db.transaction("leases", "readwrite");
    const store = transaction.objectStore("leases");
    const request = store.get(name);
    request.onsuccess = () => {
      const current = request.result;
      if (release) { if (current?.owner === owner) store.delete(name); }
      else if (!current || current.expiresAt <= Date.now()) {
        // All callers use a <=15s deadline, so the lease outlives their request.
        store.put({ owner, expiresAt: Date.now() + 20_000 }, name); acquired = true;
      }
    };
    transaction.oncomplete = () => resolve(acquired);
    transaction.onerror = () => reject(new Error("Session coordination unavailable."));
    transaction.onabort = () => reject(new Error("Session coordination unavailable."));
  });
  try {
    while (true) {
      signal.throwIfAborted();
      if (await lease(false)) break;
      await new Promise<void>((resolve, reject) => {
        const abort = () => { clearTimeout(timer); reject(signal.reason); };
        const timer = setTimeout(() => { signal.removeEventListener("abort", abort); resolve(); }, 100);
        signal.addEventListener("abort", abort, { once: true });
      });
    }
    signal.throwIfAborted();
    return await run();
  } finally { await lease(true).catch(() => undefined); db.close(); }
}
