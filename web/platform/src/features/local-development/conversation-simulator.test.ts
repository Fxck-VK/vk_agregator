// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { parseConversationMessageList, parseWebChatJob, parseConversationItem } from "@/lib/web-api/contracts";
import { createConversationSimulator } from "./conversation-simulator";
import { decodePreviewImageID } from "./preview-image";
import catalog from "@/features/session/model-catalog.preview.json";
import { parseModelCatalog, projectImageModelCatalog } from "@/features/models/model-catalog-contract";

const conversationID = "20000000-0000-4000-8000-000000000001";
const artifactID = "f0000000-0000-4000-8000-000000000001";
const now = "2026-09-17T00:00:00.000Z";
const seed = {
  conversations: [{ id: conversationID, title: "Sample", created_at: now, updated_at: now }],
  messages: { [conversationID]: { items: [{ id: "21000000-0000-4000-8000-000000000001", seq: 1, role: "assistant" as const, text: "Sample", rating: null, created_at: now }], has_more_before: false } },
  imageModels: [{ id: "sample", name: "Sample", quality_options: ["2K"], default_quality: "2K", supports_reference_image: true,
    max_reference_images: 1, max_output_count: 15, allowed_aspect_ratios: ["1:1", "9:16"], price_by_quality: { "2K": 20 } }],
};
const messagePath = `/web/v1/conversations/${conversationID}/messages` as const;
const request = { method: "POST", headers: { "X-Idempotency-Key": "message-1" }, body: JSON.stringify({ prompt: "Photo message", model_id: "sample", reference_artifact_ids: [artifactID] }) };
function simulator(scenario: () => "success" | "error" | "slow" = () => "success") {
  return createConversationSimulator({ seed, scenario, hasArtifact: id => id === artifactID, reply: () => "Local reply" });
}
afterEach(() => vi.useRealTimers());

describe("local conversation simulation", () => {
  it("renames a preview conversation and keeps the new title in subsequent reads", async () => {
    const local = simulator();
    const path = `/web/v1/conversations/${conversationID}` as const;
    const response = local.handle(path, { method: "PATCH", body: JSON.stringify({ title: "  Длинное название для проверки многоточия  " }) });
    expect(response?.status).toBe(200);
    const renamed = parseConversationItem(await response!.json());
    expect(renamed).toMatchObject({ id: conversationID, title: "Длинное название для проверки многоточия", created_at: now });
    expect(parseConversationItem(await local.handle(path)!.json())).toEqual(renamed);
    expect((await local.handle("/web/v1/conversations?limit=20")!.json()).items).toContainEqual(renamed);
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toEqual(seed.messages[conversationID].items);
    expect(seed.conversations[0].title).toBe("Sample");
  });

  it.each(["not json", "{}", JSON.stringify({ title: " " }), JSON.stringify({ title: "x".repeat(121) }), JSON.stringify({ title: "Valid", account_id: conversationID })])("rejects an invalid preview rename: %s", async body => {
    const local = simulator();
    const path = `/web/v1/conversations/${conversationID}` as const;
    expect(local.handle(path, { method: "PATCH", body })?.status).toBe(400);
    expect(parseConversationItem(await local.handle(path)!.json()).title).toBe("Sample");
  });

  it("freezes the catalogue batch price in generated image history", async () => {
    vi.useFakeTimers();
    const model = projectImageModelCatalog(parseModelCatalog(catalog)).items.find(item => item.id === "gpt_image_2_5_sunburst")!;
    const local = createConversationSimulator({ seed: { ...seed, imageModels: [model] }, scenario: () => "success", hasArtifact: () => false, reply: () => "Local reply" });
    const input = { ...request, body: JSON.stringify({ prompt: "Four photos", model_id: model.id, image_quality: "1K-medium", aspect_ratio: "16:9", output_count: 4 }) };
    expect(local.handle(messagePath, input)!.status).toBe(201);
    // An already accepted job keeps its original quote even if the source object changes.
    model.price_by_count!["1K-medium:16:9:4"] = 999;
    vi.advanceTimersByTime(1500);
    const history = parseConversationMessageList(await local.handle(messagePath)!.json());
    expect(history.items.at(-1)!.images).toHaveLength(4);
    expect(history.items.at(-1)!.images!.map(image => image.job.cost_estimate)).toEqual([30, 30, 30, 30]);
  });
  it("pins the first accepted model and ignores subsequent model switches and rejected sends", async () => {
    let scenario: "success" | "error" = "error";
    const local = simulator(() => scenario);
    expect(local.handle(messagePath, request)!.status).toBe(503);
    expect(await local.handle(`/web/v1/conversations/${conversationID}`)!.json()).not.toHaveProperty("first_model_id");
    scenario = "success";
    expect(local.handle(messagePath, request)!.status).toBe(201);
    local.handle(messagePath, { ...request, headers: { "X-Idempotency-Key": "message-2" }, body: JSON.stringify({ prompt: "Next", model_id: "gpt_5" }) });
    expect(await local.handle(`/web/v1/conversations/${conversationID}`)!.json()).toMatchObject({ first_model_id: "sample" });
    expect(await local.handle("/web/v1/conversations")!.json()).toMatchObject({ items: [{ first_model_id: "sample" }] });
  });
  it("uses the same generated reference quote in image history", async () => {
    vi.useFakeTimers();
    const model = projectImageModelCatalog(parseModelCatalog(catalog)).items.find(item => item.id === "seedream_4_5")!;
    const local = createConversationSimulator({ seed: { ...seed, imageModels: [model] }, scenario: () => "success", hasArtifact: id => id === artifactID, reply: () => "Local reply" });
    const input = { ...request, body: JSON.stringify({ prompt: "Four edited photos", model_id: model.id, image_quality: "4K", aspect_ratio: "1:1", output_count: 4, reference_artifact_ids: [artifactID] }) };
    expect(local.handle(messagePath, input)!.status).toBe(201);
    vi.advanceTimersByTime(1500);
    const images = parseConversationMessageList(await local.handle(messagePath)!.json()).items.at(-1)!.images!;
    expect(images.map(image => image.job.cost_estimate)).toEqual([180, 180, 180, 180]);
  });
  it("rejects unavailable image options without adding a turn", async () => {
    const local = createConversationSimulator({ seed: { ...seed, imageModels: [{ id: "sample", name: "Sample", quality_options: ["2K"],
      default_quality: "2K", supports_reference_image: false, max_reference_images: 0, max_output_count: 4, allowed_aspect_ratios: ["1:1"] }] },
      scenario: () => "success", hasArtifact: () => false, reply: () => "Local reply" });
    for (const options of [{ output_count: 5 }, { image_quality: "4K" }, { aspect_ratio: "9:16" }, { model_id: "unknown" }]) {
      const input = { prompt: "Sample", model_id: "sample", image_quality: "2K", aspect_ratio: "1:1", output_count: 1, ...options };
      expect(local.handle(messagePath, { ...request, body: JSON.stringify(input) })!.status).toBe(400);
    }
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(1);
  });
  it.each([1, 2, 3, 4, 15])("returns %i distinct images after the shared job finishes", async count => {
    vi.useFakeTimers();
    const local = simulator(() => "slow");
    const input = { ...request, body: JSON.stringify({ prompt: "Preview batch", model_id: "sample", image_quality: "2K", aspect_ratio: "9:16", output_count: count }) };
    const job = parseWebChatJob(await local.handle(messagePath, input)!.json());
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(2);
    vi.advanceTimersByTime(8000);
    const history = parseConversationMessageList(await local.handle(messagePath)!.json());
    const images = history.items.at(-1)!.images!;
    expect(images).toHaveLength(count);
    expect(new Set(images.map(image => image.artifact.id)).size).toBe(count);
    for (const image of images) {
      expect(image.job).toMatchObject({ id: job.job_id, status: "succeeded", model_id: "sample", image_quality: "2K" });
      expect(image.artifact.width / image.artifact.height).toBe(9 / 16);
      expect(decodePreviewImageID(image.artifact.id)).toEqual({ width: image.artifact.width, height: image.artifact.height });
    }
    expect(parseWebChatJob(await local.handle(messagePath, input)!.json()).job_id).toBe(job.job_id);
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(3);
  });
  it("accepts a message with input images, exposes polling and replies after a delay", async () => {
    vi.useFakeTimers();
    const local = simulator();
    const response = local.handle(messagePath, request)!;
    expect(response.status).toBe(201);
    const job = parseWebChatJob(await response.json()); expect(job.status).toBe("queued");
    const early = parseConversationMessageList(await local.handle(`${messagePath}?after_seq=1`)!.json());
    expect(early.items).toHaveLength(1);
    expect(early.items[0]).toMatchObject({ seq: 2, role: "user", text: "Photo message", input_images: [artifactID] });
    expect(local.isArtifactReferenced(artifactID)).toBe(true);
    vi.advanceTimersByTime(2000);
    expect(parseWebChatJob(await local.handle(`/web/v1/conversations/${conversationID}/jobs/${job.job_id}`)!.json()).status).toBe("succeeded");
    const later = parseConversationMessageList(await local.handle(`${messagePath}?after_seq=2`)!.json());
    expect(later.items).toMatchObject([{ seq: 3, role: "assistant", text: "Local reply" }]);
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(3);
  });
  it("replays an idempotency key without duplicate turns and rejects changed payloads", async () => {
    const local = simulator();
    const first = await local.handle(messagePath, request)!.json();
    const replay = local.handle(messagePath, request)!;
    expect(replay.status).toBe(200); expect(await replay.json()).toEqual(first);
    expect(local.handle(messagePath, { ...request, body: JSON.stringify({ prompt: "different" }) })!.status).toBe(409);
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(2);
  });
  it("keeps failed sends out of history and accepts a retry of the same message", async () => {
    let scenario: "success" | "error" = "error";
    const local = simulator(() => scenario);
    expect(local.handle(messagePath, request)!.status).toBe(503);
    expect(parseConversationMessageList(await local.handle(messagePath)!.json()).items).toHaveLength(1);
    scenario = "success";
    expect(local.handle(messagePath, request)!.status).toBe(201);
  });
  it("keeps a slow request pending for eight seconds even if the next scenario changes", async () => {
    vi.useFakeTimers();
    let scenario: "slow" | "success" = "slow";
    const local = simulator(() => scenario);
    const job = parseWebChatJob(await local.handle(messagePath, request)!.json());
    scenario = "success";
    vi.advanceTimersByTime(7999);
    expect(parseWebChatJob(await local.handle(`/web/v1/conversations/${conversationID}/jobs/${job.job_id}`)!.json()).status).toBe("queued");
    vi.advanceTimersByTime(1);
    expect(parseWebChatJob(await local.handle(`/web/v1/conversations/${conversationID}/jobs/${job.job_id}`)!.json()).status).toBe("succeeded");
  });
  it("creates new conversations idempotently and supports title and history reads", async () => {
    const local = simulator();
    const create = { method: "POST", headers: { "X-Idempotency-Key": "new-chat" } };
    const conversation = parseConversationItem(await local.handle("/web/v1/conversations", create)!.json());
    expect(parseConversationItem(await local.handle("/web/v1/conversations", create)!.json()).id).toBe(conversation.id);
    expect(local.handle(`/web/v1/conversations/${conversation.id}/messages`, request)!.status).toBe(201);
    expect(parseConversationItem(await local.handle(`/web/v1/conversations/${conversation.id}`)!.json()).title).toBe("Photo message");
  });
  it("does not expose jobs from another conversation or accept unknown attachments", async () => {
    const local = simulator();
    const job = await local.handle(messagePath, request)!.json();
    const other = parseConversationItem(await local.handle("/web/v1/conversations", { method: "POST", headers: { "X-Idempotency-Key": "other" } })!.json());
    expect(local.handle(`/web/v1/conversations/${other.id}/jobs/${job.job_id}`)!.status).toBe(404);
    expect(local.handle(messagePath, { ...request, headers: { "X-Idempotency-Key": "missing-photo" }, body: JSON.stringify({ prompt: "Missing", reference_artifact_ids: [conversationID] }) })!.status).toBe(400);
  });
});
