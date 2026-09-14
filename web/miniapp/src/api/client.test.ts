import { afterEach, describe, expect, test, vi } from "vitest";

import {
  ApiError,
  artifactUrl,
  apiUserMessage,
  createChatMessage,
  createJob,
  errorLabel,
  estimateJob,
  getAccountProfile,
  launchParamsFromLocation,
  listChatMessages,
  listModelCatalog,
  listTextModels,
  normalizeRawParams,
  referralCodeFromRaw,
  requestAccountEmailCode,
  resetLaunchParamsCacheForTest,
  statusKind,
  stringifyBridgeLaunchParams,
  telemetryLabel,
  telemetryRoute,
  verifyAccountEmailCode,
} from "./client";
import type { CreateChatMessageInput, CreateJobInput, EstimateInput, Job } from "./client";

const ARTIFACT_ID = "550e8400-e29b-41d4-a716-446655440000";

afterEach(() => {
  vi.restoreAllMocks();
  resetLaunchParamsCacheForTest();
  window.history.replaceState({}, "", "/");
});

test("text catalog accepts every supported model and keeps the server quote", async () => {
  const ids = ["chatgpt", "gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro", "claude_opus_4_8", "gpt_5_6_terra", "gpt_6_astra", "claude_opus_5", "gemini_3_7_flash", "claude_fable_5_1", "claude_fable_5", "gemini_3_6_flash"];
  const items = ids.map((id) => ({ id, name: id, estimate_credits: id === "claude_fable_5_1" ? 90 : 5 }));
  vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(jsonResponse({ items }));
  expect(await listTextModels()).toEqual(items);
});

test("text catalog rejects private provider IDs", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(jsonResponse({ items: [{ id: "claude-fable-5.1", name: "Synthetic", estimate_credits: 90 }] }));
  await expect(listTextModels()).rejects.toMatchObject({ status: 500 });
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
        video_resolution: "1080p",
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
      ["duration_sec", "operation", "prompt", "video_route_alias", "video_resolution"].sort(),
    );
	 expect(bodies[1].video_resolution).toBe("1080p");
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

  test("sends more than the legacy fallback reference count when UUIDs are valid", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const refs = Array.from({ length: 5 }, (_, index) => `550e8400-e29b-41d4-a716-44665544000${index}`);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
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
    );

    await createJob(
      {
        operation: "image_generate",
        prompt: "many refs",
        model_id: "nano_banana_2",
        reference_artifact_ids: refs,
      },
      { idempotencyKey: "idem-many-refs" },
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const body = JSON.parse(String((fetchMock.mock.calls[0][1] as RequestInit).body ?? "{}")) as Record<
      string,
      unknown
    >;
    expect(body.reference_artifact_ids).toEqual(refs);
  });

  test("rejects invalid reference artifact UUIDs before calling the backend", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(jsonResponse({}));

    await expect(
      createJob(
        {
          operation: "image_generate",
          prompt: "bad ref",
          model_id: "nano_banana_2",
          reference_artifact_ids: ["not-a-uuid"],
        },
        { idempotencyKey: "idem-invalid-ref" },
      ),
    ).rejects.toMatchObject({ status: 400, code: "validation_error" });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("model capability response safety", () => {
  test("keeps legacy catalog items and discards malformed nested capabilities", async () => {
    const validCapabilities = {
      schema_version: 1,
      api: {
        image: {
          images: { support: "supported", extensions: [".png"], max_count: 16 },
          aspect_ratios: ["1:1"],
          resolutions: ["1024x1024"],
          quality_modes: ["high"],
          speed_modes: ["fast"],
          max_output_count: 1,
          max_combined_images: 16,
        },
      },
      application: {
        image: {
          images: { support: "unsupported", extensions: [], max_count: 0 },
          aspect_ratios: ["1:1"],
          resolutions: ["1024x1024"],
          quality_modes: ["high"],
          speed_modes: ["fast"],
          max_output_count: 1,
          max_combined_images: 1,
        },
      },
    };
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        items: [
          {
            type: "image",
            id: "legacy",
            name: "Legacy",
            enabled: true,
            supports_reference_image: false,
            requires_start_image: false,
          },
          {
            type: "image",
            id: "valid",
            name: "Valid",
            enabled: true,
            supports_reference_image: false,
            requires_start_image: false,
            capabilities: validCapabilities,
          },
          {
            type: "image",
            id: "malformed",
            name: "Malformed",
            enabled: true,
            supports_reference_image: false,
            requires_start_image: false,
            capabilities: { schema_version: 1, api: { text: {}, image: {} }, application: { text: {} } },
          },
          {
            type: "video",
            id: "wrong-purpose",
            name: "Wrong purpose",
            enabled: true,
            supports_reference_image: false,
            requires_start_image: false,
            capabilities: validCapabilities,
          },
        ],
      }),
    );

    const items = await listModelCatalog();

    expect(items).toHaveLength(4);
    expect(items[0]?.capabilities).toBeUndefined();
    expect(items[1]?.capabilities).toEqual(validCapabilities);
    expect(items[2]?.capabilities).toBeUndefined();
    expect(items[3]?.capabilities).toBeUndefined();
  });

  test("keeps text models when malformed capabilities are discarded", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        items: [
          {
            id: "chatgpt",
            name: "НейроХаб",
            estimate_credits: 0,
            capabilities: {
              schema_version: 2,
              api: { text: {} },
              application: { text: {} },
            },
          },
          {
            id: "gpt_5_5",
            name: "GPT 5.5",
            estimate_credits: 5,
            capabilities: {
              schema_version: 1,
              api: {
                image: {
                  images: { support: "supported", extensions: [], max_count: 4 },
                  aspect_ratios: ["1:1"],
                  resolutions: ["1024x1024"],
                  quality_modes: ["high"],
                  speed_modes: [],
                  max_output_count: 1,
                  max_combined_images: 4,
                },
              },
              application: {
                image: {
                  images: { support: "unsupported", extensions: [], max_count: 0 },
                  aspect_ratios: ["1:1"],
                  resolutions: ["1024x1024"],
                  quality_modes: ["high"],
                  speed_modes: [],
                  max_output_count: 1,
                  max_combined_images: 1,
                },
              },
            },
          },
        ],
      }),
    );

    expect(await listTextModels()).toEqual([
      { id: "chatgpt", name: "НейроХаб", estimate_credits: 0 },
      { id: "gpt_5_5", name: "GPT 5.5", estimate_credits: 5 },
    ]);
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

describe("account API safe identity contract", () => {
  test("loads account profile through launch-authenticated safe endpoint", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        account_id: "acc_1234567890",
        identity_refs: [
          {
            id: "ident_1",
            account_id: "acc_1234567890",
            provider: "vk",
            label: "VK",
            verified: true,
            created_at: "2026-01-01T00:00:00Z",
          },
        ],
      }),
    );

    const profile = await getAccountProfile();

    expect(profile.identity_refs[0]?.label).toBe("VK");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/account/me");
    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Record<string, string>;
    expect(headers["X-Launch-Params"]).toContain("vk_user_id=42");
    expect(Object.prototype.hasOwnProperty.call(profile.identity_refs[0] ?? {}, "external_id")).toBe(false);
    expect(Object.prototype.hasOwnProperty.call(profile.identity_refs[0] ?? {}, "normalized_id")).toBe(false);
  });

  test("uses method-specific email link flow without generic identity payload", async () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(jsonResponse({ status: "sent", expires_in_seconds: 600 }, 202))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            id: "ident_email",
            account_id: "acc_1234567890",
            provider: "email",
            label: "u***@example.com",
            verified: true,
            created_at: "2026-01-01T00:00:00Z",
          },
          201,
        ),
      );

    await requestAccountEmailCode("user@example.com");
    const identity = await verifyAccountEmailCode("user@example.com", "123456");

    expect(identity.provider).toBe("email");
    expect(fetchMock.mock.calls[0][0]).toBe("/account/identities/email/request-code");
    expect(fetchMock.mock.calls[1][0]).toBe("/account/identities/email/verify");
    const bodies = fetchMock.mock.calls.map(([, init]) =>
      JSON.parse(String((init as RequestInit | undefined)?.body ?? "{}")) as Record<string, unknown>,
    );
    for (const body of bodies) {
      expect(body).not.toHaveProperty("external_id");
      expect(body).not.toHaveProperty("verification_token");
      expect(body).not.toHaveProperty("provider");
    }
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
