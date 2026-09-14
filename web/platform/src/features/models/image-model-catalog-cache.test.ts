import { afterEach, describe, expect, it, vi } from "vitest";

import {
  loadImageModelCatalog,
  resetImageModelCatalogCacheForTests,
} from "./image-model-catalog-cache";
import { publicModelCatalog } from "./model-catalog-test-fixtures";

const validCatalogue = publicModelCatalog();

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((promiseResolve) => {
    resolve = promiseResolve;
  });

  return { promise, resolve };
}

afterEach(() => {
  resetImageModelCatalogCacheForTests();
});

describe("loadImageModelCatalog", () => {
  it("shares one request between concurrent consumers", async () => {
    const response = deferred<Response>();
    const fetcher = vi.fn(() => response.promise);
    const first = loadImageModelCatalog({ fetcher });
    const second = loadImageModelCatalog({ fetcher });

    expect(fetcher).toHaveBeenCalledOnce();
    expect(fetcher).toHaveBeenCalledWith("/web/v1/models");
    response.resolve(Response.json(validCatalogue));
    await expect(Promise.all([first, second])).resolves.toEqual([
      expect.objectContaining({ items: expect.any(Array) }),
      expect.objectContaining({ items: expect.any(Array) }),
    ]);
  });

  it("consumes the catalogue response without cloning its body", async () => {
    const response = Response.json(validCatalogue);
    const clone = vi.spyOn(response, "clone");
    const fetcher = vi.fn().mockResolvedValue(response);

    await expect(loadImageModelCatalog({ fetcher })).resolves.toEqual(
      expect.objectContaining({ items: expect.any(Array) }),
    );

    expect(clone).not.toHaveBeenCalled();
  });

  it("projects image controls and server metadata from the unified catalogue", async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json(validCatalogue)));

    await expect(loadImageModelCatalog({ fetcher })).resolves.toEqual({
      items: [
        expect.objectContaining({
          id: "image",
          description: "Server image description",
          categories: ["popular", "images"],
          default_aspect_ratio: "16:9",
          price_by_variant: { "1K:1:1": 10, "1K:16:9": 12, "2K:1:1": 18, "2K:16:9": 20 },
          quality_label: "Режим",
          show_output_count: false,
          max_prompt_bytes: 2048,
        }),
      ],
    });
  });

  it("reuses a fresh successful catalogue then refetches after 60 seconds", async () => {
    let now = 1_000;
    const fetcher = vi.fn(() => Promise.resolve(Response.json(validCatalogue)));
    await loadImageModelCatalog({ fetcher, now: () => now });
    await loadImageModelCatalog({ fetcher, now: () => now + 59_999 });
    now += 60_000;
    await loadImageModelCatalog({ fetcher, now: () => now });

    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it.each([
    () => Promise.resolve(new Response(null, { status: 500 })),
    () => Promise.resolve(Response.json({ items: [{ id: "missing required fields" }] })),
    () => Promise.reject(new Error("request rejected")),
  ])("does not retain a failed catalogue load", async (request) => {
    const fetcher = vi.fn().mockImplementationOnce(request).mockResolvedValue(Response.json(validCatalogue));

    await expect(loadImageModelCatalog({ fetcher })).rejects.toThrow();
    await expect(loadImageModelCatalog({ fetcher })).resolves.toEqual(
      expect.objectContaining({ items: expect.any(Array) }),
    );
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
