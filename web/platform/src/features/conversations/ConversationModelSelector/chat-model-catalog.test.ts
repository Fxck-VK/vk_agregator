import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));

import { webBrowserFetch } from "@/lib/web-api/browser";
import { loadChatModelCatalog } from "./chat-model-catalog";
import { resetModelCatalogCacheForTests } from "@/features/models/model-catalog-cache";
import { publicModelCatalog } from "@/features/models/model-catalog-test-fixtures";

const catalog = publicModelCatalog();

describe("chat model catalogue", () => {
  afterEach(() => {
    resetModelCatalogCacheForTests();
    vi.resetAllMocks();
  });

  it("loads and validates the authenticated web chat catalogue", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
    await expect(loadChatModelCatalog()).resolves.toEqual({
      default_model_id: "chatgpt",
      items: [
        expect.objectContaining({
          id: "chatgpt",
          name: "Chat",
          description: "Server chat description",
          estimate_credits: 0,
          max_prompt_bytes: 4096,
          max_output_tokens: 1024,
          categories: ["popular", "text", "free", "study-work"],
        }),
      ],
    });
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/models");
  });

  it.each([
    { status: 503, body: catalog },
    { status: 200, body: { ...publicModelCatalog(), default_model_id: "missing" } },
    { status: 200, body: { ...publicModelCatalog(), items: [...publicModelCatalog().items, publicModelCatalog().items[0]] } },
    { status: 200, body: { ...publicModelCatalog(), items: [{ ...publicModelCatalog().items[0], provider: "private" }] } },
  ])("rejects unavailable or invalid catalogues %#", async ({ status, body }) => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(body, { status }));
    await expect(loadChatModelCatalog()).rejects.toThrow();
  });
});
