import { afterEach, describe, expect, it, vi } from "vitest";
import { loadModelCatalog, resetModelCatalogCacheForTests, seedModelCatalog } from "./model-catalog-cache";
import { parseModelCatalog } from "./model-catalog-contract";
import { publicModelCatalog } from "./model-catalog-test-fixtures";

const catalog = parseModelCatalog(publicModelCatalog());
afterEach(() => { resetModelCatalogCacheForTests(); vi.useRealTimers(); });
describe("shared raw catalog cache", () => {
  it("honors the original seed age and coalesces the first expired read", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    seedModelCatalog({ value: catalog, updatedAt: 40_001 });
    const fetcher = vi.fn(async () => Response.json(catalog));
    expect(await loadModelCatalog({ fetcher })).toBe(catalog);
    expect(fetcher).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    await Promise.all([loadModelCatalog({ fetcher }), loadModelCatalog({ fetcher })]);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("does not replace a newer browser result with an older server seed", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    const newer = structuredClone(catalog);
    newer.items[0].name = "Newer image";
    const fetcher = vi.fn(async () => Response.json(newer));
    const loaded = await loadModelCatalog({ fetcher });
    seedModelCatalog({ value: catalog, updatedAt: 99_000 });
    expect(await loadModelCatalog({ fetcher })).toBe(loaded);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("does not let a seed older than the pending read discard its fresh response", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    let finish!: (response: Response) => void;
    const fetcher = vi.fn(() => new Promise<Response>(resolve => { finish = resolve; }));
    const pending = loadModelCatalog({ fetcher });
    seedModelCatalog({ value: catalog, updatedAt: 30_000 });
    const fresh = structuredClone(catalog);
    fresh.items[0].name = "Fresh browser response";
    finish(Response.json(fresh));
    await expect(pending).resolves.toEqual(fresh);
    expect(await loadModelCatalog({ fetcher })).toEqual(fresh);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it.each(["success", "failure"])("a late %s does not overwrite a newer installed seed", async outcome => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    let finish!: (response: Response) => void;
    const fetcher = vi.fn(() => new Promise<Response>(resolve => { finish = resolve; }));
    const pending = loadModelCatalog({ fetcher });
    const newer = structuredClone(catalog);
    newer.items[0].name = "Seed from a newer route";
    vi.advanceTimersByTime(1000);
    seedModelCatalog({ value: newer, updatedAt: Date.now() });
    finish(outcome === "success" ? Response.json(catalog) : new Response(null, { status: 503 }));
    if (outcome === "success") await expect(pending).resolves.toBe(newer);
    else await expect(pending).rejects.toThrow();
    expect(await loadModelCatalog({ fetcher })).toBe(newer);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("retries once for concurrent readers after a failed expired refresh", async () => {
    seedModelCatalog({ value: catalog, updatedAt: Date.now() - 60_001 });
    const fetcher = vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockImplementation(async () => Response.json(catalog));
    const failed = await Promise.allSettled([loadModelCatalog({ fetcher }), loadModelCatalog({ fetcher })]);
    expect(failed.every(result => result.status === "rejected")).toBe(true);
    expect(fetcher).toHaveBeenCalledTimes(1);
    const [first, second] = await Promise.all([loadModelCatalog({ fetcher }), loadModelCatalog({ fetcher })]);
    expect(first).toBe(second);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
