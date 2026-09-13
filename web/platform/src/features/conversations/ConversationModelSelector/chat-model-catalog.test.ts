import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));

import { webBrowserFetch } from "@/lib/web-api/browser";
import { loadChatModelCatalog } from "./chat-model-catalog";

const catalog = { default_model_id: "chatgpt", items: [{ id: "chatgpt", name: "NeiroHub Chat" }] };

describe("chat model catalogue", () => {
  afterEach(() => vi.resetAllMocks());

  it("loads and validates the authenticated web chat catalogue", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
    await expect(loadChatModelCatalog()).resolves.toEqual(catalog);
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/chat-models");
  });

  it.each([
    { status: 503, body: catalog },
    { status: 200, body: { ...catalog, default_model_id: "missing" } },
    { status: 200, body: { ...catalog, items: [...catalog.items, ...catalog.items] } },
    { status: 200, body: { ...catalog, items: [{ ...catalog.items[0], provider: "private" }] } },
  ])("rejects unavailable or invalid catalogues %#", async ({ status, body }) => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(body, { status }));
    await expect(loadChatModelCatalog()).rejects.toThrow();
  });
});
