import { afterEach, describe, expect, it, vi } from "vitest";
import { parseModelCatalog } from "./model-catalog-contract";
import { publicModelCatalog } from "./model-catalog-test-fixtures";
import { clearModelCatalogSession, readModelCatalogSession, writeModelCatalogSession } from "./model-catalog-session-cache";

const key = "epifera.model-catalog.v1";
const value = parseModelCatalog(publicModelCatalog());
afterEach(() => { sessionStorage.clear(); vi.useRealTimers(); vi.restoreAllMocks(); });
describe("non-personal catalogue session snapshot", () => {
  it("bounds UTF-8 bytes, including multibyte catalogue names", () => {
    const large = structuredClone(value); large.items[0].description = "я".repeat(760_000);
    const raw = JSON.stringify({ version: 1, value: large, updatedAt: Date.now() });
    expect(raw.length).toBeLessThan(1_500_000);
    sessionStorage.setItem(key, raw);
    expect(readModelCatalogSession()).toBeNull();
    writeModelCatalogSession({ value: large, updatedAt: Date.now() });
    expect(sessionStorage.getItem(key)).toBeNull();
  });
  it("restores a validated snapshot without extending its original age", () => {
    vi.useFakeTimers(); vi.setSystemTime(100_000);
    writeModelCatalogSession({ value, updatedAt: 99_000 });
    expect(readModelCatalogSession()).toEqual({ value, updatedAt: 99_000 });
    vi.advanceTimersByTime(900_001);
    expect(readModelCatalogSession()).toBeNull();
    expect(sessionStorage.getItem(key)).toBeNull();
  });
  it.each(["{broken", JSON.stringify({ version: 1, value, updatedAt: 1e20 }), JSON.stringify({ version: 1, value: { ...value, account_id: "private" }, updatedAt: 1 }), "x".repeat(1_500_001)])("drops corrupt, future, private or oversized data", raw => {
    sessionStorage.setItem(key, raw);
    expect(readModelCatalogSession()).toBeNull();
    expect(sessionStorage.getItem(key)).toBeNull();
  });
  it("clears the saved data when the workspace session ends", () => {
    writeModelCatalogSession({ value, updatedAt: Date.now() });
    expect(sessionStorage.getItem(key)).not.toBeNull();
    clearModelCatalogSession();
    expect(readModelCatalogSession()).toBeNull();
  });
  it("treats blocked storage as optional", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
    expect(() => writeModelCatalogSession({ value, updatedAt: Date.now() })).not.toThrow();
    expect(readModelCatalogSession()).toBeNull();
  });
});
