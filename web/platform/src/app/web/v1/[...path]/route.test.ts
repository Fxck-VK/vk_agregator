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
import {
  parseChatModelList,
  parseConversationMessageList,
  parseImageJobList,
  parseImageJobResult,
} from "../../../../lib/web-api/contracts";
import { GET, POST } from "./route";
import { parseModelCatalog, projectChatModelCatalog, projectImageModelCatalog } from "@/features/models/model-catalog-contract";
import { createPreviewImage } from "@/features/local-development/preview-image";
import sharp from "sharp";

describe("web API route local workspace preview", () => {
  it("forwards private thumbnail reads to the backend without fetching or converting the original", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "0");
    const request = new Request("https://web.example.test/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001?preview=1");
    const saved = new Response("saved WebP", { headers: { "Content-Type": "image/webp", "Cache-Control": "private, no-store" } });
    vi.mocked(proxyWebApiRequest).mockResolvedValueOnce(saved);
    const response = await GET(request);
    expect(response).toBe(saved);
    expect(await response.text()).toBe("saved WebP");
    expect(proxyWebApiRequest).toHaveBeenCalledTimes(1);
    expect(proxyWebApiRequest).toHaveBeenCalledWith(request, "/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001?preview=1", "http://backend.internal:8080");
  });

  it.each([401, 403, 404])("does not fetch an original after thumbnail access is denied: %s", async (status) => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "0");
    const denied = new Response(null, { status });
    vi.mocked(proxyWebApiRequest).mockResolvedValueOnce(denied);
    expect(await GET(new Request("https://web.example.test/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001?preview=1"))).toBe(denied);
    expect(proxyWebApiRequest).toHaveBeenCalledTimes(1);
  });

  it("preserves an authenticated image preview during a backend thumbnail outage", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "0");
    const original = await sharp({ create: { width: 800, height: 1600, channels: 4, background: { r: 100, g: 20, b: 40, alpha: 0.5 } } }).png().toBuffer();
    vi.mocked(proxyWebApiRequest)
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(new Response(new Uint8Array(original), { headers: { "Content-Type": "image/png" } }));
    const request = new Request("https://web.example.test/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001?preview=1", { headers: { Cookie: "web_session=opaque", Range: "bytes=0-2", "If-Range": "etag" } });
    const response = await GET(request);
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/webp");
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(await sharp(Buffer.from(await response.arrayBuffer())).metadata()).toMatchObject({ width: 320, height: 640 });
    expect(proxyWebApiRequest).toHaveBeenCalledTimes(2);
    const [fallback, path] = vi.mocked(proxyWebApiRequest).mock.calls[1];
    expect(path).toBe("/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001");
    expect(fallback.headers.get("Cookie")).toBe("web_session=opaque");
    expect(fallback.headers.get("Range")).toBeNull();
    expect(fallback.headers.get("If-Range")).toBeNull();
  });

  it.each(["development", "production"] as const)("keeps empty music history limited to local development: %s", async (environment) => {
    vi.stubEnv("NODE_ENV", environment);
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const request = new Request("http://localhost:7158/web/v1/music-jobs?limit=8");

    const response = await GET(request);

    if (environment === "development") {
      expect(await response.json()).toEqual({ items: [], has_more: false, next_cursor: null });
      expect(proxyWebApiRequest).not.toHaveBeenCalled();
    } else {
      expect(proxyWebApiRequest).toHaveBeenCalledWith(request, "/web/v1/music-jobs?limit=8", "http://backend.internal:8080");
    }
  });

  it("serves bounded image fixtures at the requested ratio only in local preview", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const artifact = createPreviewImage("9:16");
    const request = new Request(`http://localhost:7158/web/v1/image-artifacts/${artifact.id}`);
    const response = await GET(request);
    expect(response.headers.get("Content-Type")).toBe("image/jpeg");
    const metadata = await sharp(Buffer.from(await response.arrayBuffer())).metadata();
    expect(metadata).toMatchObject({ width: artifact.width, height: artifact.height });
    expect((await GET(new Request("http://localhost:7158/web/v1/image-artifacts/f1000000-ffff-4fff-8000-000000000000"))).status).toBe(404);
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
    vi.stubEnv("NODE_ENV", "production");
    await GET(request);
    expect(proxyWebApiRequest).toHaveBeenCalledTimes(1);
  });
  it.each(["input-artifacts", "image-jobs", "conversations", "unknown-preview-path"])("never forwards preview writes to %s to a real backend", async path => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const response = await POST(new Request(`http://localhost:7158/web/v1/${path}`, { method: "POST" }));
    expect(response.status).toBe(503);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });
  it("shows preview packages but never forwards a preview payment", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const response = await GET(new Request("http://localhost:7158/web/v1/payment-products"));
    expect(await response.json()).toMatchObject({ checkout_available: false, items: expect.any(Array) });
    const create = await POST(new Request("http://localhost:7158/web/v1/payments/intents", { method: "POST" }));
    expect(create.status).toBe(503);
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });
  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  it("serves the current public chat model catalogue in local preview", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const response = await GET(new Request("http://localhost:7158/web/v1/chat-models"));
    const unified = await GET(new Request("http://localhost:7158/web/v1/models"));
    const catalog = parseModelCatalog(await unified.json());
    expect(parseChatModelList(await response.json())).toEqual(projectChatModelCatalog(catalog));
    expect(catalog.items.find(model => model.id === "chatgpt")?.categories).toContain("text");
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });

  it("projects image models from the unified generated preview without the backend", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");

    const response = await GET(new Request("http://localhost:7158/web/v1/image-models"));

    expect(response.status).toBe(200);
    const unified = await GET(new Request("http://localhost:7158/web/v1/models"));
    const catalog = parseModelCatalog(await unified.json());
    await expect(response.json()).resolves.toEqual(projectImageModelCatalog(catalog));
    expect(catalog.items.some(model => model.id === "gpt_image_2")).toBe(true);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(getWebApiInternalOrigin).not.toHaveBeenCalled();
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });

  it("serves representative file cards and local previews without the backend in local development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");

    const listResponse = await GET(new Request("http://localhost:7158/web/v1/image-jobs?limit=12"));

    expect(listResponse.status).toBe(200);
    expect(listResponse.headers.get("Cache-Control")).toBe("no-store");
    const page = parseImageJobList(await listResponse.json());
    expect(page).toMatchObject({ has_more: false, next_cursor: null });
    expect(page.items.map(({ status }) => status)).toEqual([
      "succeeded",
      "succeeded",
      "succeeded",
      "expired",
      "awaiting_payment",
      "failed_terminal",
    ]);

    for (const job of page.items.filter(({ status }) => status === "succeeded")) {
      const resultResponse = await GET(
        new Request(`http://localhost:7158/web/v1/image-jobs/${job.id}/result`),
      );
      expect(resultResponse.status).toBe(200);
      expect(resultResponse.headers.get("Cache-Control")).toBe("no-store");
      const result = parseImageJobResult(await resultResponse.json());
      expect(result.job_id).toBe(job.id);

      const artifactResponse = await GET(
        new Request(`http://localhost:7158/web/v1/image-artifacts/${result.artifacts[0].id}`),
      );
      expect(artifactResponse.status).toBe(200);
      expect(artifactResponse.headers.get("Content-Type")).toBe("image/png");
      expect(artifactResponse.headers.get("Cache-Control")).toBe("no-store");
      expect((await artifactResponse.arrayBuffer()).byteLength).toBeGreaterThan(0);
    }

    expect(getWebApiInternalOrigin).not.toHaveBeenCalled();
    expect(proxyWebApiRequest).not.toHaveBeenCalled();
  });

  it("serves representative histories for every local preview conversation", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const conversationIDs = [
      "20000000-0000-4000-8000-000000000004",
      "20000000-0000-4000-8000-000000000001",
      "20000000-0000-4000-8000-000000000002",
      "20000000-0000-4000-8000-000000000003",
    ];

    for (const conversationID of conversationIDs) {
      const response = await GET(new Request(
        `http://localhost:7158/web/v1/conversations/${conversationID}/messages?limit=100`,
      ));

      expect(response.status).toBe(200);
      expect(response.headers.get("Cache-Control")).toBe("no-store");
      const history = parseConversationMessageList(await response.json());
      expect(history.items.length).toBeGreaterThanOrEqual(2);
      expect(history.items.some(({ role }) => role === "user")).toBe(true);
      expect(history.items.some(({ role }) => role === "assistant")).toBe(true);
      expect(history.has_more_before).toBe(false);
      if (conversationID === "20000000-0000-4000-8000-000000000004") {
        const answer = history.items.find(({ role }) => role === "assistant");
        const imagePath = answer?.text.match(/!\[[^\]]+\]\((\/web\/v1\/image-artifacts\/[^)]+)\)/)?.[1];
        expect(imagePath).toBe("/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001");
        expect(answer?.images).toHaveLength(1);
        const attachment = answer!.images![0];
        const filesResponse = await GET(new Request("http://localhost:7158/web/v1/image-jobs?limit=12"));
        const files = parseImageJobList(await filesResponse.json());
        expect(attachment.job).toEqual(files.items.find((job) => job.id === attachment.job.id));
        const resultResponse = await GET(new Request(`http://localhost:7158/web/v1/image-jobs/${attachment.job.id}/result`));
        const result = parseImageJobResult(await resultResponse.json());
        expect(result.artifacts).toContainEqual(attachment.artifact);
        expect(imagePath).toBe(`/web/v1/image-artifacts/${attachment.artifact.id}`);
        const imageResponse = await GET(new Request(`http://localhost:7158${imagePath}`));
        expect(imageResponse.status).toBe(200);
        expect(imageResponse.headers.get("Content-Type")).toBe("image/png");
        expect((await imageResponse.arrayBuffer()).byteLength).toBe(attachment.artifact.size_bytes);
      }
    }

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

  it("keeps image-job requests on the backend proxy path in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    const request = new Request("https://platform.example/web/v1/image-jobs?limit=12");

    const response = await GET(request);

    expect(response.status).toBe(204);
    expect(getWebApiInternalOrigin).toHaveBeenCalledTimes(1);
    expect(proxyWebApiRequest).toHaveBeenCalledWith(
      request,
      "/web/v1/image-jobs?limit=12",
      "http://backend.internal:8080",
    );
  });
});
