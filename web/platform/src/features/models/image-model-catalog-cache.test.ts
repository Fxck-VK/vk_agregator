import { afterEach, describe, expect, it, vi } from "vitest";

import {
  loadImageModelCatalog,
  resetImageModelCatalogCacheForTests,
} from "./image-model-catalog-cache";

const validCatalogue = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana",
      quality_options: ["1K", "2K"],
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
  ],
};

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
    expect(fetcher).toHaveBeenCalledWith("/web/v1/image-models");
    response.resolve(Response.json(validCatalogue));
    await expect(Promise.all([first, second])).resolves.toEqual([
      expect.objectContaining({ items: expect.any(Array) }),
      expect.objectContaining({ items: expect.any(Array) }),
    ]);
  });

  it("reuses a fresh successful catalogue then refetches after 60 seconds", async () => {
    let now = 1_000;
    const fetcher = vi.fn().mockResolvedValue(Response.json(validCatalogue));
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
