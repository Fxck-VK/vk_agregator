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

describe("web API route local workspace preview", () => {
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
