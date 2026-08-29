import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

vi.mock("../../../../lib/web-api/internal-origin", () => ({
  getWebApiInternalOrigin: vi.fn(() => "http://backend.internal:8080"),
}));

vi.mock("../../../../lib/web-api/proxy", () => ({
  proxyWebApiRequest: vi.fn(async () => new Response(null, { status: 204 })),
}));

import { getWebApiInternalOrigin } from "../../../../lib/web-api/internal-origin";
import { proxyWebApiRequest } from "../../../../lib/web-api/proxy";
import { GET } from "./route";

describe("web API route local workspace preview", () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  it("serves a fixed image model catalogue without the backend in local development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");

    const response = await GET(new Request("http://localhost:7158/web/v1/image-models"));

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      items: [
        { id: "nano-banana-2", name: "Nano Banana 2" },
        { id: "nano-banana-pro", name: "Nano Banana Pro" },
        { id: "gpt-image-2", name: "GPT Image 2" },
        { id: "seedream-4-5", name: "Seedream 4.5" },
      ],
    });
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(getWebApiInternalOrigin).not.toHaveBeenCalled();
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });

  it("keeps the existing backend proxy path in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const request = new Request("https://platform.example/web/v1/image-models");

    const response = await GET(request);

    expect(response.status).toBe(204);
    expect(getWebApiInternalOrigin).toHaveBeenCalledTimes(1);
    expect(proxyWebApiRequest).toHaveBeenCalledWith(
      request,
      "/web/v1/image-models",
      "http://backend.internal:8080",
    );
  });
});
