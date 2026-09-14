import { afterEach, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));

import { webBrowserFetch } from "@/lib/web-api/browser";

import { loadGenerationModelCatalog } from "./generation-model-catalog";
import { resetModelCatalogCacheForTests } from "./model-catalog-cache";
import type { PublicCatalog } from "./model-catalog-contract";
import { publicModelCatalog } from "./model-catalog-test-fixtures";

afterEach(() => {
  resetModelCatalogCacheForTests();
  vi.resetAllMocks();
});

it("projects image, text and video models from one unified request", async () => {
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(publicModelCatalog()));
 const catalog = await loadGenerationModelCatalog();
 expect(catalog.items.map(model => [model.id, model.category])).toEqual([["image", "images"], ["chatgpt", "text"], ["video", "video"]]);
 expect(catalog.default_model_id).toBe("chatgpt");
 expect(catalog.items[0]).toMatchObject({ description: "Server image description", operations: [expect.objectContaining({ kind: "image" })] });
 expect(catalog.items[1]).toMatchObject({ isFree: true, description: "Server chat description", operations: [expect.objectContaining({ kind: "text" })] });
 expect(catalog.items[2]).toMatchObject({ description: "Server video description", operations: [expect.objectContaining({ kind: "video" })] });
 expect(catalog.categoryErrors).toEqual({});
 expect(webBrowserFetch).toHaveBeenCalledTimes(1);
});

it("uses the model primary kind when a model exposes multiple operations", async () => {
 const base = publicModelCatalog();
 const payload = {
  ...base,
  default_model_id: "multi",
  items: [{
   ...base.items[1],
   id: "multi",
   name: "Multimodal",
   kind: "text",
   operations: [base.items[1].operations[0], base.items[0].operations[0]],
  }],
 } as unknown as PublicCatalog;
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(payload));

 const catalog = await loadGenerationModelCatalog();

 expect(catalog.items).toHaveLength(1);
 expect(catalog.items[0]).toMatchObject({ id: "multi", category: "text" });
 const operations = catalog.items[0].operations as PublicCatalog["items"][number]["operations"];
 expect(operations.map((operation) => operation.kind)).toEqual(["text", "image"]);
});

it("returns failures for every selector category without rejecting when the unified catalog is unavailable", async () => {
 vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 503 }));
 const catalog = await loadGenerationModelCatalog();
 expect(catalog.items).toEqual([]);
 expect(catalog.default_model_id).toBe("");
 expect(catalog.categoryErrors).toEqual({
  popular: expect.any(String),
  images: expect.any(String),
  text: expect.any(String),
  "video-audio": expect.any(String),
  free: expect.any(String),
  "study-work": expect.any(String),
 });
 expect(webBrowserFetch).toHaveBeenCalledTimes(1);
});
