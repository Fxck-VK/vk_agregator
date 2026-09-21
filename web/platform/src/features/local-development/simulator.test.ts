// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createLocalSimulator } from "./simulator";
import { defaultScenarios } from "./scenarios";

describe("local API simulator", () => {
  beforeEach(() => { vi.useFakeTimers(); vi.stubGlobal("createImageBitmap", vi.fn(async () => ({ width: 800, height: 600, close: vi.fn() }))); });
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });
  function upload() {
    const body = new FormData();
    body.set("file", new File(["local-image-bytes"], "photo.png", { type: "image/png" }));
    return body;
  }
  it("reports progressive upload, returns reusable in-memory artifacts and quotes without network", async () => {
    const network = vi.fn(); vi.stubGlobal("fetch", network);
    const simulator = createLocalSimulator(() => defaultScenarios);
    const progress = vi.fn();
    const pending = simulator.handle("/web/v1/input-artifacts?model_id=preview", { method: "POST", body: upload(), onUploadProgress: progress })!;
    await vi.advanceTimersByTimeAsync(400);
    expect(progress.mock.calls.some(([value]) => value > 0 && value < 100)).toBe(true);
    await vi.runAllTimersAsync();
    const data = await (await pending).json();
    expect(data).toMatchObject({ artifact_id: expect.any(String), width: 800, height: 600, mime_type: "image/png" });
    const artifact = await simulator.handle(`/web/v1/image-artifacts/${data.artifact_id}`);
    expect(artifact?.headers.get("content-type")).toBe("image/png");
    const quote = await simulator.handle("/web/v1/image-reference-quote?output_count=2");
    expect(await quote?.json()).toEqual({ credits: 40 });
    expect(await (await simulator.handle("/web/v1/image-reference-quote?output_count=15"))?.json()).toEqual({ credits: 300 });
    expect(network).not.toHaveBeenCalled();
    simulator.dispose();
  });
  it("can fail then retry successfully after changing scenarios", async () => {
    let scenarios = { ...defaultScenarios, upload: "error" as "error" | "success" };
    const simulator = createLocalSimulator(() => scenarios);
    const failed = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload() })!;
    await vi.runAllTimersAsync();
    expect((await failed).status).toBe(503);
    scenarios = { ...scenarios, upload: "success" };
    const retry = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload() })!;
    await vi.runAllTimersAsync();
    expect((await retry).status).toBe(201);
    simulator.dispose();
  });
  it("simulates a disconnected upload as a transport failure without storing an artifact", async () => {
    const simulator = createLocalSimulator(() => ({ ...defaultScenarios, upload: "offline" }));
    const progress = vi.fn();
    const pending = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload(), onUploadProgress: progress })!;
    const result = expect(pending).rejects.toMatchObject({ name: "WebNetworkError" });
    await vi.runAllTimersAsync();
    await result;
    expect(progress).toHaveBeenCalledWith(0);
    expect(progress).not.toHaveBeenCalledWith(100);
    simulator.dispose();
  });
  it("cancels slow uploads immediately without completing them", async () => {
    const simulator = createLocalSimulator(() => ({ ...defaultScenarios, upload: "slow" }));
    const controller = new AbortController(); const progress = vi.fn();
    const pending = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload(), signal: controller.signal, onUploadProgress: progress })!;
    const result = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    await vi.advanceTimersByTimeAsync(600); controller.abort();
    await result;
    const count = progress.mock.calls.length;
    await vi.runAllTimersAsync();
    expect(progress).toHaveBeenCalledTimes(count);
    expect(progress).not.toHaveBeenCalledWith(100);
    simulator.dispose();
  });
  it("rejects malformed images and unsupported writes; passes through fixture reads", async () => {
    const simulator = createLocalSimulator(() => defaultScenarios);
    expect(simulator.handle("/web/v1/models")).toBeUndefined();
    expect((await simulator.handle("/web/v1/payments/intents", { method: "POST" }))?.status).toBe(503);
    expect((await simulator.handle("/web/v1/input-artifacts", { method: "POST", body: new FormData() }))?.status).toBe(400);
    vi.mocked(createImageBitmap).mockRejectedValueOnce(new Error("Invalid image"));
    expect((await simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload() }))?.status).toBe(400);
    simulator.dispose();
  });
  it("disposal aborts pending requests and disabled simulation leaves normal transport alone", async () => {
    const simulator = createLocalSimulator(() => defaultScenarios);
    const pending = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload() })!;
    const result = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    simulator.dispose(); await result;
    const disabled = createLocalSimulator(() => ({ ...defaultScenarios, enabled: false }));
    expect(disabled.handle("/web/v1/input-artifacts", { method: "POST", body: upload() })).toBeUndefined();
    disabled.dispose();
  });
  it("keeps photos used in sent messages when evicting older unused uploads", async () => {
    const simulator = createLocalSimulator(() => defaultScenarios, { reply: () => "Test reply" });
    const addPhoto = async () => {
      const response = simulator.handle("/web/v1/input-artifacts", { method: "POST", body: upload() })!;
      await vi.runAllTimersAsync();
      return (await (await response).json()).artifact_id as string;
    };
    const used = await addPhoto();
    const unused = await addPhoto();
    const conversation = await (await simulator.handle("/web/v1/conversations", { method: "POST", headers: { "X-Idempotency-Key": "new" } }))!.json();
    const sent = await simulator.handle(`/web/v1/conversations/${conversation.id}/messages`, {
      method: "POST", headers: { "X-Idempotency-Key": "send" },
      body: JSON.stringify({ prompt: "Keep photo", reference_artifact_ids: [used] }),
    });
    expect(sent?.status).toBe(201);
    for (let index = 0; index < 49; index++) await addPhoto();
    const retained = await simulator.handle(`/web/v1/input-artifacts/${used}`);
    expect(retained?.status).toBe(200);
    expect(await retained?.text()).toBe("local-image-bytes");
    expect((await simulator.handle(`/web/v1/input-artifacts/${unused}`))?.status).toBe(404);
    simulator.dispose();
    expect((await simulator.handle(`/web/v1/input-artifacts/${used}`))?.status).toBe(404);
  });
});
