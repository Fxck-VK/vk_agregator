import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("@/lib/web-api/server", () => ({ webServerFetch: vi.fn() }));
vi.mock("@/lib/web-api/internal-origin", () => ({ getWebApiInternalOrigin: () => "http://catalog.test" }));
vi.mock("@/features/session/local-workspace-preview", () => ({ isLocalWorkspacePreviewEnabled: () => false }));
import rawCatalog from "@/features/session/model-catalog.preview.json";
import { webServerFetch } from "@/lib/web-api/server";

beforeEach(() => { vi.resetModules(); vi.mocked(webServerFetch).mockReset(); });
afterEach(() => vi.useRealTimers());
describe("server model seed", () => {
  it("caches only a validated successful catalog and coalesces reads", async () => {
    const { loadServerModelCatalog } = await import("./model-catalog-server");
    vi.mocked(webServerFetch).mockResolvedValueOnce(Response.json(rawCatalog));
    const [first, second] = await Promise.all([loadServerModelCatalog(), loadServerModelCatalog()]);
    expect(first?.value.items.length).toBeGreaterThan(0);
    expect(first?.value.items.some(model => model.id === "gpt_4o_mini_tts")).toBe(true);
    expect(first?.updatedAt).toBeGreaterThan(0);
    expect(second).toBe(first);
    expect(await loadServerModelCatalog()).toBe(first);
    expect(webServerFetch).toHaveBeenCalledTimes(1);
  });
  it("does not cache authorization failures and bounds the initial screen wait", async () => {
    const { loadServerModelCatalog, initialModelCatalog } = await import("./model-catalog-server");
    vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 401 }));
    expect(await loadServerModelCatalog()).toBeNull();
    let finish!: (response: Response) => void;
    vi.mocked(webServerFetch).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
    vi.useFakeTimers();
    const pending = initialModelCatalog();
    await vi.advanceTimersByTimeAsync(200);
    expect(await pending).toBeNull();
    expect(webServerFetch).toHaveBeenCalledTimes(2);
    finish(Response.json(rawCatalog));
    expect((await loadServerModelCatalog())?.value.items.length).toBeGreaterThan(0);
  });
  it("preserves the seed age on cache hits and reloads after 60 seconds", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    const { loadServerModelCatalog } = await import("./model-catalog-server");
    vi.mocked(webServerFetch).mockImplementation(async () => Response.json(rawCatalog));
    const seed = await loadServerModelCatalog();
    expect(seed?.updatedAt).toBe(100_000);
    vi.advanceTimersByTime(59_999);
    expect(await loadServerModelCatalog()).toBe(seed);
    vi.advanceTimersByTime(1);
    expect((await loadServerModelCatalog())?.updatedAt).toBe(160_000);
    expect(webServerFetch).toHaveBeenCalledTimes(2);
  });
});
