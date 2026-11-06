import { parseModelCatalog, type ModelCatalogSnapshot } from "./model-catalog-contract";

const key = "epifera.model-catalog.v1";
const maxAgeMs = 15 * 60_000;
const maxBytes = 1_500_000;
const byteLength = (raw: string) => new TextEncoder().encode(raw).byteLength;

export function clearModelCatalogSession(): void {
  try { window.sessionStorage.removeItem(key); } catch { /* Optional presentation cache. */ }
}

/** Restore only inside the server-verified workspace, after hydration. */
export function readModelCatalogSession(): ModelCatalogSnapshot | null {
  try {
    const raw = window.sessionStorage.getItem(key);
    if (!raw) return null;
    if (raw.length > maxBytes || byteLength(raw) > maxBytes) { clearModelCatalogSession(); return null; }
    const stored = JSON.parse(raw);
    if (!stored || stored.version !== 1 || Object.keys(stored).some(field => !["version", "updatedAt", "value", "view"].includes(field))
      || stored.view !== undefined && stored.view !== "dev" && stored.view !== "prod"
      || !Number.isFinite(stored.updatedAt) || stored.updatedAt < 0 || stored.updatedAt > Date.now()
      || Date.now() - stored.updatedAt > maxAgeMs || !Array.isArray(stored.value?.items) || stored.value.items.length > 256) {
      clearModelCatalogSession(); return null;
    }
    return { value: parseModelCatalog(stored.value), updatedAt: stored.updatedAt, ...(stored.view ? { view: stored.view } : {}) };
  } catch { clearModelCatalogSession(); return null; }
}

/** Only the public catalogue DTO: no profile, balance, identity or credentials. */
export function writeModelCatalogSession(snapshot: ModelCatalogSnapshot): void {
  if (typeof window === "undefined") return;
  try {
    if (!Number.isFinite(snapshot.updatedAt) || snapshot.updatedAt < 0 || snapshot.updatedAt > Date.now()
      || Date.now() - snapshot.updatedAt > maxAgeMs || snapshot.value.items.length > 256) return;
    const raw = JSON.stringify({ version: 1, updatedAt: snapshot.updatedAt, value: parseModelCatalog(snapshot.value), ...(snapshot.view ? { view: snapshot.view } : {}) });
    if (raw.length <= maxBytes && byteLength(raw) <= maxBytes) window.sessionStorage.setItem(key, raw);
  } catch { /* Quota/disabled storage must not prevent catalogue reads. */ }
}
