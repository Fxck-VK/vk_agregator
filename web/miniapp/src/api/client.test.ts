import { afterEach, describe, expect, test, vi } from "vitest";

import {
  ApiError,
  artifactUrl,
  apiUserMessage,
  createChatMessage,
  createJob,
  errorLabel,
  estimateJob,
  launchParamsFromLocation,
  listChatMessages,
  normalizeRawParams,
  referralCodeFromRaw,
  resetLaunchParamsCacheForTest,
  statusKind,
  stringifyBridgeLaunchParams,
  telemetryLabel,
  telemetryRoute,
} from "./client";
import type { CreateChatMessageInput, CreateJobInput, EstimateInput, Job } from "./client";

const ARTIFACT_ID = "550e8400-e29b-41d4-a716-446655440000";

afterEach(() => {
  vi.restoreAllMocks();
  resetLaunchParamsCacheForTest();
  window.history.replaceState({}, "", "/");
});

describe("telemetry safety helpers", () => {
  test("normalizes routes without query, hash, prompts or launch params", () => {
    const route = telemetryRoute(
      `/miniapp/jobs/${ARTIFACT_ID}?prompt=secret-text&launch_params=vk_user_id%3D1#private_url=secret`,
    );

    expect(route).toBe("/miniapp/jobs/:id");
    expect(route).not.toContain("prompt");
    expect(route).not.toContain("launch");
    expect(route).not.toContain("secret");
    expect(route).not.toContain(ARTIFACT_ID);
  });

  test("bounds label characters and length", () => {
    const label = telemetryLabel(" Payment Failed!!! /Raw URL?x=1 ".repeat(4), "unknown");

    expect(label.length).toBeLessThanOrEqual(96);
    expect(label).toMatch(/^[a-z0-9_./:-]+$/);
    expect(label).not.toContain("?");
  });
});

describe("launch and referral parsing helpers", () => {
  test("normalizes raw query/hash prefixes", () => {
    expect(normalizeRawParams("?vk_user_id=42")).toBe("vk_user_id=42");
    expect(normalizeRawParams("#vk_user_id=42")).toBe("vk_user_id=42");
  });

  test("extracts only sign and vk-prefixed launch params when a VK user identity is present", () => {
    window.history.replaceState(
      {},
      "",
      "/?vk_user_id=42&vk_ts=1&sign=fake&vk_new_param=keep&ref=ABCD2345&private_url=secret&unsafe=drop",
    );
    const raw = launchParamsFromLocation();
    expect(raw).toContain("vk_user_id=42");
    expect(raw).toContain("vk_ts=1");
    expect(raw).toContain("sign=fake");
    expect(raw).toContain("vk_new_param=keep");
    expect(raw).not.toContain("ref=");
    expect(raw).not.toContain("private_url");
    expect(raw).not.toContain("unsafe");

    expect(referralCodeFromRaw(window.location.search)).toBe("ABCD2345");

    window.history.replaceState({}, "", "/?ref=ABCD1234");
    expect(launchParamsFromLocation()).toBe("");
  });

  test("serializes bridge launch params without undefined or null values", () => {
    const raw = stringifyBridgeLaunchParams({
      vk_user_id: 42,
      vk_ts: 1,
      sign: "fake",
      vk_new_param: "keep",
      ref: "ABCD2345",
      private_url: "secret",
      ignored: undefined,
      empty: null,
    });

    expect(raw).toContain("vk_user_id=42");
    expect(raw).toContain("vk_ts=1");
    expect(raw).toContain("sign=fake");
    expect(raw).toContain("vk_new_param=keep");
    expect(raw).not.toContain("ref=");
    expect(raw).not.toContain("private_url");
    expect(raw).not.toContain("ignored");
    expect(raw).not.toContain("empty");
  });

  test("does not pin an empty launch-param lookup for later API calls", async () => {
    window.history.replaceState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(jsonResponse({ error: "unauthorized" }, 401))
      .mockResolvedValueOnce(
        jsonResponse({
          items: [],
          pagination: { limit: 20, offset: 0, count: 0, has_more: false },
        }),
      );

    await expect(listChatMessages()).rejects.toMatchObject({ status: 401 });

    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    await listChatMessages();

    const secondHeaders = fetchMock.mock.calls[1]?.[1]?.headers as Record<string, string>;
    expect(secondHeaders["X-Launch-Params"]).toContain("vk_user_id=42");
  });

  test("accepts only public referral-code shape", () => {
    expect(referralCodeFromRaw("?ref=ABCD2345")).toBe("ABCD2345");
    expect(referralCodeFromRaw("?ref=bad code")).toBe("");
    expect(referralCodeFromRaw("?ref=vk_user_id=42")).toBe("");
  });
});

describe("artifact and status helpers", () => {
  test("builds artifact URLs from UUIDs only", () => {
    expect(artifactUrl(ARTIFACT_ID)).toBe(`/miniapp/artifacts/${ARTIFACT_ID}`);
    expect(artifactUrl(`${ARTIFACT_ID}?launch_params=secret`)).toBeNull();
    expect(artifactUrl("https://storage.local/private/file.png")).toBeNull();
  });

  test("maps terminal statuses without exposing backend details", () => {
    expect(statusKind("succeeded")).toBe("done");
    expect(statusKind("failed_terminal")).toBe("failed");
    expect(statusKind("provider_running")).toBe("progress");
    expect(errorLabel({ status: "failed_terminal", error_code: "unknown" } as never)).toBeTruthy();
  });

  test("renders safe media failure labels without provider details", () => {
    const label = errorLabel({
      status: "failed_terminal",
      error_code: "media_provider_output_invalid",
    } as never);

    expect(label).toContain("⭐️ не списаны");
    expect(label.toLowerCase()).not.toContain("provider");
    expect(label.toLowerCase()).not.toContain("prompt");
    expect(label).not.toContain(ARTIFACT_ID);
  });

  test("renders safe media API errors without raw backend details", () => {
    const msg = apiUserMessage(new ApiError(503, "media_overloaded_retry_later"));

    expect(msg).toContain("⭐️ не списаны");
    expect(msg.toLowerCase()).not.toContain("provider");
    expect(msg.toLowerCase()).not.toContain("launch");
    expect(msg.toLowerCase()).not.toContain("payload");
  });

  test("prefers backend safe user message over local error code label", () => {
    const label = errorLabel(
      jobFixture({
        error_code: "model_unavailable",
        user_message: "Безопасное сообщение от backend",
      }),
    );

    expect(label).toBe("Безопасное сообщение от backend");
  });

  test("keeps local error code fallback for older backend responses", () => {
    expect(errorLabel(jobFixture({ error_code: "model_unavailable" }))).toBe(
      "Выбранная модель сейчас недоступна. Попробуйте другую модель. ⭐️ не списаны",
    );
    expect(errorLabel(jobFixture({ error_code: "invalid_request", user_message: "   " }))).toBe(
      "Модель не приняла запрос. Попробуйте другую модель или измените описание; возможны ограничения по содержанию. ⭐️ не списаны",
    );
    expect(errorLabel(jobFixture({ error_code: "content_rejected" }))).toBe(
      "Запрос отклонён правилами безопасности. Измените описание. ⭐️ не списаны",
    );
  });
});

describe("generation request pricing contract", () => {
  test("serializes only public fields for create and estimate requests", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse({
          id: ARTIFACT_ID,
          operation: "image_generate",
          modality: "image",
          status: "received",
          cost_estimate: 16,
          cost_captured: 0,
          output_artifact_ids: [],
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          operation: "video_generate",
          video_route_alias: "video_kling_o3_standard",
          cost_estimate: 100,
          balance_credits: 200,
          enough_credits: true,
        }),
      );

    await createJob(
      withPrivatePricingFields({
        operation: "image_generate",
        prompt: "public image prompt",
        model_id: "nano_banana_2",
        image_quality: "2K",
        reference_artifact_ids: [ARTIFACT_ID],
      }) as CreateJobInput,
      { idempotencyKey: "idem-create" },
    );
    await estimateJob(
      withPrivatePricingFields({
        operation: "video_generate",
        prompt: "public video prompt",
        video_route_alias: "video_kling_o3_standard",
        duration_sec: 5,
      }) as EstimateInput,
    );

    expect(fetchMock).toHaveBeenCalledTimes(2);
    const bodies = fetchMock.mock.calls.map(([, init]) =>
      JSON.parse(String((init as RequestInit | undefined)?.body ?? "{}")) as Record<string, unknown>,
    );
    expect(Object.keys(bodies[0]).sort()).toEqual(
      ["image_quality", "model_id", "operation", "prompt", "reference_artifact_ids"].sort(),
    );
    expect(Object.keys(bodies[1]).sort()).toEqual(
      ["duration_sec", "operation", "prompt", "video_route_alias"].sort(),
    );
    for (const body of bodies) {
      expect(body).not.toHaveProperty("price");
      expect(body).not.toHaveProperty("cost");
      expect(body).not.toHaveProperty("cost_estimate");
      expect(body).not.toHaveProperty("provider");
      expect(body).not.toHaveProperty("provider_cost");
      expect(body).not.toHaveProperty("provider_cost_credits");
      expect(body).not.toHaveProperty("multiplier");
      expect(body).not.toHaveProperty("price_multiplier");
      expect(body).not.toHaveProperty("floor");
      expect(body).not.toHaveProperty("resolution");
      expect(body).not.toHaveProperty("aspect_ratio");
      expect(body).not.toHaveProperty("provider_model_id");
      expect(body).not.toHaveProperty("model_code");
    }
  });
});

describe("chat API single default contract", () => {
  test("serializes chat messages without client conversation id", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        id: ARTIFACT_ID,
        operation: "text_generate",
        modality: "text",
        status: "received",
        conversation_id: "default",
        cost_estimate: 0,
        cost_captured: 0,
        output_artifact_ids: [],
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      }),
    );

    await createChatMessage(
      { prompt: "hello chat", conversation_id: "custom-a" } as CreateChatMessageInput & { conversation_id: string },
      { idempotencyKey: "idem-chat" },
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/miniapp/chat/messages");
    const body = JSON.parse(String((fetchMock.mock.calls[0][1] as RequestInit).body ?? "{}")) as Record<
      string,
      unknown
    >;
    expect(body).toEqual({ prompt: "hello chat" });
    expect(body).not.toHaveProperty("conversation_id");
  });

  test("loads chat history from the default messages endpoint", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        items: [
          {
            id: ARTIFACT_ID,
            job_id: ARTIFACT_ID,
            seq: 1,
            role: "user",
            text: "saved message",
            created_at: "2026-01-01T00:00:00Z",
          },
        ],
        pagination: { limit: 20, offset: 0, count: 1, has_more: false },
      }),
    );

    const messages = await listChatMessages();

    expect(messages).toHaveLength(1);
    expect(messages[0]?.text).toBe("saved message");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/miniapp/chat/messages");
    expect(String(fetchMock.mock.calls[0][0])).not.toContain("/chat/conversations/");
  });
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function jobFixture(overrides: Partial<Job> = {}): Job {
  return {
    id: ARTIFACT_ID,
    operation: "image_generate",
    modality: "image",
    status: "failed_terminal",
    cost_estimate: 0,
    cost_captured: 0,
    output_artifact_ids: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function withPrivatePricingFields<T extends Record<string, unknown>>(value: T): T {
  return {
    ...value,
    price: 1,
    cost: 1,
    cost_estimate: 1,
    provider: "client-provider",
    provider_cost: 1,
    provider_cost_credits: 1,
    multiplier: 1,
    price_multiplier: 1,
    floor: 1,
    resolution: "client-resolution",
    aspect_ratio: "client-aspect",
    provider_model_id: "client-provider-model",
    model_code: "client-model-code",
  };
}
