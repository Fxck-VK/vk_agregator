import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));

import { webBrowserFetch } from "@/lib/web-api/browser";

import { publicModelCatalog } from "./model-catalog-test-fixtures";
import { resetModelCatalogCacheForTests } from "./model-catalog-cache";
import { loadVideoModelCatalog } from "./video-model-catalog";

describe("video model catalogue", () => {
  afterEach(() => {
    resetModelCatalogCacheForTests();
    vi.resetAllMocks();
  });

  it("projects video controls and variants from the unified catalogue", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(publicModelCatalog()));

    await expect(loadVideoModelCatalog()).resolves.toEqual({
      items: [
        expect.objectContaining({
          id: "video",
          description: "Server video description",
          categories: ["popular", "video-audio"],
          variants: [
            {
              resolution: "720p",
              duration_sec: 8,
              aspect_ratio: "16:9",
              fps: 24,
              audio: false,
            },
          ],
        }),
      ],
    });
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/models");
  });

  it("rejects invalid unified catalogues", async () => {
    const payload = publicModelCatalog();
    payload.items[2].operations[0].video.variants[0].resolution = "not-offered";
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(payload));

    await expect(loadVideoModelCatalog()).rejects.toThrow();
  });
});
