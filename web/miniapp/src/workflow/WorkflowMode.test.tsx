import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, test, vi } from "vitest";
import { WorkflowMode } from "./WorkflowMode";
import { estimateJob, listModelCatalog, uploadArtifact, uploadVideoArtifact } from "../api/client";

vi.mock("@vkontakte/vkui", () => ({
  Button: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props} />,
  NativeSelect: (props: React.SelectHTMLAttributes<HTMLSelectElement>) => <select {...props} />,
}));

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  listModelCatalog: vi.fn(),
  estimateJob: vi.fn(),
  uploadArtifact: vi.fn(),
  uploadVideoArtifact: vi.fn(),
}));

const gptImage25QualityOptions = [
  "1K-low",
  "1K-medium",
  "1K-high",
  "1K-xhigh",
  "1K-max",
  "2K-low",
  "2K-medium",
  "2K-high",
  "2K-xhigh",
  "2K-max",
  "4K-low",
  "4K-medium",
  "4K-high",
  "4K-xhigh",
  "4K-max",
];

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

test("Motion Control uploads image and video refs and submits video options", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn((file: File) => `blob:${file.name}`) });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  const imageID = "550e8400-e29b-41d4-a716-446655440001";
  const videoID = "550e8400-e29b-41d4-a716-446655440002";
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_kling_2_6_motion_control", alias: "video_kling_2_6_motion_control", name: "Kling 2.6 Motion Control", enabled: true,
    allowed_durations_sec: [3, 10, 30], default_duration_sec: 8,
    allowed_resolutions: ["std", "pro"], default_resolution: "std",
    allowed_aspect_ratios: ["16:9"], supports_reference_image: true, requires_start_image: true, max_reference_images: 1,
    allowed_reference_image_counts: [1], requires_reference_video: true,
  }]);
  vi.mocked(uploadArtifact).mockResolvedValue(imageID);
  vi.mocked(uploadVideoArtifact).mockResolvedValue({ artifact_id: videoID, duration_sec: 8 });
  vi.mocked(estimateJob).mockResolvedValue({ operation: "video_generate", cost_estimate: 24, balance_credits: 100, enough_credits: true });
  const onCreateJob = vi.fn().mockResolvedValue({ id: "job-1", operation: "video_generate", modality: "video", status: "received", cost_estimate: 24, cost_captured: 0, output_artifact_ids: [], created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" });
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{ name: "Test", firstName: "Test", avatar: null }} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={() => {}} onCreateJob={onCreateJob} />); });
    const imageInput = container.querySelector<HTMLInputElement>("#workflow-reference-input")!;
    const videoInput = container.querySelector<HTMLInputElement>("#workflow-reference-video-input")!;
    await act(async () => {
      Object.defineProperty(imageInput, "files", { configurable: true, value: [new File(["png"], "start.png", { type: "image/png" })] });
      imageInput.dispatchEvent(new Event("change", { bubbles: true }));
      Object.defineProperty(videoInput, "files", { configurable: true, value: [new File(["mp4"], "source.mp4", { type: "video/mp4" })] });
      videoInput.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(Array.from(container.querySelectorAll("button")).some((button) => button.textContent?.trim() === "8 сек")).toBe(false);
    await act(async () => {
      const quality = container.querySelector<HTMLSelectElement>('select[aria-label="Качество видео"]')!;
      Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(quality, "pro");
      quality.dispatchEvent(new Event("change", { bubbles: true }));
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(prompt, "Animate the reference");
      prompt.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({ video_route_alias: "video_kling_2_6_motion_control", reference_artifact_ids: [imageID], reference_video_artifact_id: videoID, duration_sec: 8, video_resolution: "pro", character_orientation: "image", keep_original_sound: true }));
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click(); });
    expect(onCreateJob).toHaveBeenCalledWith("Animate the reference", expect.objectContaining({ referenceArtifactIds: [imageID], referenceVideoArtifactId: videoID, durationSec: 8, videoResolution: "pro", characterOrientation: "image", keepOriginalSound: true }));
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test("Kling V3 audio checkbox requotes with video audio", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_kling_v3", alias: "video_kling_v3", name: "Kling V3", enabled: true,
    allowed_durations_sec: [5, 10], default_duration_sec: 5,
    allowed_resolutions: ["720p", "1080p", "4k"], default_resolution: "720p",
    allowed_aspect_ratios: ["16:9", "9:16", "1:1"], supports_reference_image: true, requires_start_image: false, max_reference_images: 2,
    supports_audio: true,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({ operation: "video_generate", cost_estimate: 12, balance_credits: 100, enough_credits: true });
  const onCreateJob = vi.fn().mockResolvedValue(null);
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{ name: "Test", firstName: "Test", avatar: null }} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={() => {}} onCreateJob={onCreateJob} />); });
    await act(async () => {
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(prompt, "Cinematic scene");
      prompt.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({ video_route_alias: "video_kling_v3", video_audio: false }));
    const callsBeforeAudio = vi.mocked(estimateJob).mock.calls.length;
    await act(async () => { container.querySelector<HTMLInputElement>('input[aria-label="Добавить звук"]')!.click(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({ video_route_alias: "video_kling_v3", video_audio: true }));
    expect(vi.mocked(estimateJob).mock.calls.length).toBeGreaterThan(callsBeforeAudio);
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test.each([
  {
    name: "Kling 3.0 Turbo",
    alias: "video_kling_3_0_turbo",
    description: "Текст или первый кадр, 3-15 секунд",
    promptLimit: 3072,
    minDuration: 3,
    selectedResolutionLabel: "1080p",
    selectedResolutionValue: "1080p",
    resolutions: ["720p", "1080p"],
    ratios: ["16:9", "9:16", "1:1"],
  },
  {
    name: "MiniMax H3",
    alias: "video_minimax_h3",
    description: "Текст или первый кадр, 4-15 секунд",
    promptLimit: 7000,
    minDuration: 4,
    selectedResolutionLabel: "768P",
    selectedResolutionValue: "768p",
    resolutions: ["2k", "768p"],
    ratios: ["16:9", "9:16", "1:1", "4:3", "3:4", "21:9"],
  },
])("$name uses prompt caps, first-frame upload and no audio toggle", async ({
  name,
  alias,
  description,
  promptLimit,
  minDuration,
  selectedResolutionLabel,
  selectedResolutionValue,
  resolutions,
  ratios,
}) => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn((file: File) => `blob:${file.name}`) });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  const imageID = "550e8400-e29b-41d4-a716-446655440003";
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: alias, alias, name, description, enabled: true,
    allowed_durations_sec: Array.from({ length: 16 - minDuration }, (_, index) => minDuration + index),
    default_duration_sec: 5,
    allowed_resolutions: resolutions,
    default_resolution: resolutions[0],
    allowed_aspect_ratios: ratios,
    supports_reference_image: true,
    requires_start_image: false,
    max_reference_images: 1,
    supports_audio: true,
    max_prompt_chars: promptLimit,
  }]);
  vi.mocked(uploadArtifact).mockResolvedValue(imageID);
  vi.mocked(estimateJob).mockResolvedValue({ operation: "video_generate", cost_estimate: 42, balance_credits: 100, enough_credits: true });
  const onCreateJob = vi.fn().mockResolvedValue(null);
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{ name: "Test", firstName: "Test", avatar: null }} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={() => {}} onCreateJob={onCreateJob} />); });
    const buttons = () => Array.from(container.querySelectorAll("button"));
    expect(container.textContent).toContain(name);
    expect(container.textContent).toContain(description);
    expect(container.textContent?.toLowerCase()).toContain("первый кадр");
    expect(container.querySelector<HTMLInputElement>('input[aria-label="Добавить звук"]')).toBeNull();
    for (let seconds = minDuration; seconds <= 15; seconds++) {
      expect(buttons().some((button) => button.textContent?.trim() === `${seconds} сек`)).toBe(true);
    }
    for (const ratio of ratios) expect(container.textContent).toContain(ratio);
    if (selectedResolutionLabel !== selectedResolutionValue) {
      expect(buttons().some((button) => button.textContent?.trim() === selectedResolutionLabel)).toBe(true);
      expect(buttons().some((button) => button.textContent?.trim() === selectedResolutionValue)).toBe(false);
    }

    vi.mocked(estimateJob).mockClear();
    await act(async () => {
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(prompt, "x".repeat(promptLimit + 1));
      prompt.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    const promptWarning = `${(promptLimit + 1).toLocaleString("ru-RU")} / ${promptLimit.toLocaleString("ru-RU")}`.replace(/\u00a0/g, " ");
    expect((container.textContent ?? "").replace(/\u00a0/g, " ")).toContain(promptWarning);
    expect(estimateJob).not.toHaveBeenCalled();

    await act(async () => {
      const imageInput = container.querySelector<HTMLInputElement>("#workflow-reference-input")!;
      Object.defineProperty(imageInput, "files", { configurable: true, value: [new File(["png"], "first-frame.png", { type: "image/png" })] });
      imageInput.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    await act(async () => {
      buttons().find((button) => button.textContent?.trim() === "15 сек")!.click();
      buttons().find((button) => button.textContent?.trim() === selectedResolutionLabel)!.click();
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(prompt, "Synthetic first-frame scene");
      prompt.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    const estimateCalls = vi.mocked(estimateJob).mock.calls;
    const payload = estimateCalls[estimateCalls.length - 1]?.[0];
    expect(payload).toEqual(expect.objectContaining({
      video_route_alias: alias,
      reference_artifact_ids: [imageID],
      duration_sec: 15,
      video_resolution: selectedResolutionValue,
    }));
    expect(payload?.video_audio).toBeUndefined();
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click(); });
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic first-frame scene", expect.objectContaining({
      referenceArtifactIds: [imageID],
      durationSec: 15,
      videoResolution: selectedResolutionValue,
    }));
    const createCalls = onCreateJob.mock.calls;
    expect(createCalls[createCalls.length - 1]?.[1].videoAudio).toBeUndefined();
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test("Kling 3.0 Turbo submits an uploaded first frame without prompt text", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn((file: File) => `blob:${file.name}`) });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  const imageID = "550e8400-e29b-41d4-a716-446655440004";
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_kling_3_0_turbo", alias: "video_kling_3_0_turbo", name: "Kling 3.0 Turbo", enabled: true,
    allowed_durations_sec: [3, 4, 5], default_duration_sec: 5,
    allowed_resolutions: ["720p", "1080p"], default_resolution: "720p",
    allowed_aspect_ratios: ["16:9"], supports_reference_image: true, requires_start_image: false, max_reference_images: 1,
    max_prompt_chars: 3072,
  }]);
  vi.mocked(uploadArtifact).mockResolvedValue(imageID);
  vi.mocked(estimateJob).mockResolvedValue({ operation: "video_generate", cost_estimate: 42, balance_credits: 100, enough_credits: true });
  const onCreateJob = vi.fn().mockResolvedValue({ id: "job-kling", operation: "video_generate", modality: "video", status: "received", cost_estimate: 42, cost_captured: 0, output_artifact_ids: [], created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" });
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{ name: "Test", firstName: "Test", avatar: null }} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={() => {}} onCreateJob={onCreateJob} />); });
    const submit = () => container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!;
    expect(submit().disabled).toBe(true);
    vi.mocked(estimateJob).mockClear();
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).not.toHaveBeenCalled();

    await act(async () => {
      const imageInput = container.querySelector<HTMLInputElement>("#workflow-reference-input")!;
      Object.defineProperty(imageInput, "files", { configurable: true, value: [new File(["png"], "first-frame.png", { type: "image/png" })] });
      imageInput.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({
      prompt: "",
      video_route_alias: "video_kling_3_0_turbo",
      reference_artifact_ids: [imageID],
    }));
    expect(submit().disabled).toBe(false);
    await act(async () => { submit().click(); });
    expect(onCreateJob).toHaveBeenCalledWith("", expect.objectContaining({
      videoRouteAlias: "video_kling_3_0_turbo",
      referenceArtifactIds: [imageID],
    }));
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test("MiniMax H3 still requires prompt text when a first frame is attached", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn((file: File) => `blob:${file.name}`) });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  const imageID = "550e8400-e29b-41d4-a716-446655440005";
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_minimax_h3", alias: "video_minimax_h3", name: "MiniMax H3", enabled: true,
    allowed_durations_sec: [4, 5], default_duration_sec: 5,
    allowed_resolutions: ["2k", "768p"], default_resolution: "2k",
    allowed_aspect_ratios: ["16:9"], supports_reference_image: true, requires_start_image: false, max_reference_images: 1,
    max_prompt_chars: 7000,
  }]);
  vi.mocked(uploadArtifact).mockResolvedValue(imageID);
  vi.mocked(estimateJob).mockResolvedValue({ operation: "video_generate", cost_estimate: 42, balance_credits: 100, enough_credits: true });
  const onCreateJob = vi.fn().mockResolvedValue(null);
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{ name: "Test", firstName: "Test", avatar: null }} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={() => {}} onCreateJob={onCreateJob} />); });
    const submit = () => container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!;
    expect(submit().disabled).toBe(true);
    vi.mocked(estimateJob).mockClear();
    await act(async () => {
      const imageInput = container.querySelector<HTMLInputElement>("#workflow-reference-input")!;
      Object.defineProperty(imageInput, "files", { configurable: true, value: [new File(["png"], "first-frame.png", { type: "image/png" })] });
      imageInput.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).not.toHaveBeenCalled();
    expect(submit().disabled).toBe(true);
    await act(async () => { submit().click(); });
    expect(onCreateJob).not.toHaveBeenCalled();
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test.each([false, true])("Omni EXT=%s uses its documented duration controls and resolution tariff", async (ext) => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  const alias = `video_gemini_omni_1_1_flash${ext ? "_ext" : ""}`;
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: alias, alias, name: "Gemini Omni 1.1 Flash", enabled: true,
    automatic_duration: !ext, allowed_durations_sec: ext ? [6, 4, 8, 10] : [10], default_duration_sec: ext ? 6 : 10,
    allowed_resolutions: ["720p", "1080p", "360p", "4k"], default_resolution: "720p",
    allowed_aspect_ratios: ["16:9", "9:16"], supports_reference_image: true, requires_start_image: false,
    max_reference_images: ext ? 3 : 10, allowed_reference_image_counts: ext ? [0, 1, 3] : undefined,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"video_generate",cost_estimate:ext ? 510 : 1585,balance_credits:10000,enough_credits:true});
  const onCreateJob = vi.fn().mockResolvedValue(null);
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>); });
    const buttons = () => Array.from(container.querySelectorAll("button"));
    if (ext) {
      for (const seconds of [4, 6, 8, 10]) expect(buttons().some(b => b.textContent?.trim() === `${seconds} сек`)).toBe(true);
      await act(async () => { buttons().find(b => b.textContent?.trim() === "8 сек")!.click(); });
    } else {
      expect(container.textContent).toContain("Автоматически: 3–10 секунд");
      expect(buttons().some(b => b.textContent?.trim() === "10 сек")).toBe(false);
    }
    await act(async () => {
      buttons().find(b => b.textContent?.trim() === "4k")!.click();
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({video_route_alias:alias,duration_sec:ext ? 8 : 10,video_resolution:"4k"}));
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click(); });
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic scene",expect.objectContaining({durationSec:ext ? 8 : 10,videoResolution:"4k"}));
  } finally { await act(async () => { root.unmount(); }); container.remove(); }
});

test("FLUX.2 offers MP tiers without reference upload and submits the selected tier", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "flux_2_pro", name: "FLUX.2 Pro", enabled: true,
    quality_options: ["1MP", "2MP", "3MP", "4MP"], default_quality: "1MP", estimate_credits: 15,
    supports_reference_image: false, requires_start_image: false, max_reference_images: 0,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:40,balance_credits:100,enough_credits:true});
  const onCreateJob = vi.fn().mockResolvedValue(null);
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>); });
    expect(container.querySelector('input[type="file"]')).toBeNull();
    const buttons = () => Array.from(container.querySelectorAll("button"));
    for (const tier of ["1MP", "2MP", "3MP", "4MP"]) expect(buttons().some(b => b.textContent?.trim() === tier)).toBe(true);
    await act(async () => {
      buttons().find(b => b.textContent?.trim() === "4MP")!.click();
      const prompt = container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({model_id:"flux_2_pro",image_quality:"4MP"}));
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click(); });
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic scene",expect.objectContaining({modelId:"flux_2_pro",imageQuality:"4MP"}));
  } finally {
    await act(async () => { root.unmount(); }); container.remove();
  }
});

test("GPT Image 2.5 shows clean composite quality labels and keeps raw request values", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "gpt_image_2_5_flare", name: "GPT Image 2.5 Flare", enabled: true,
    quality_options: gptImage25QualityOptions, default_quality: "1K-medium", estimate_credits: 30,
    supports_reference_image: false, requires_start_image: false, max_reference_images: 0,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:120,balance_credits:200,enough_credits:true});
  const onCreateJob=vi.fn().mockResolvedValue(null);
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>);});
    const buttons=()=>Array.from(container.querySelectorAll("button"));
    expect(buttons().some(button=>button.textContent?.trim()==="1K·Среднее")).toBe(true);
    expect(buttons().some(button=>button.textContent?.trim()==="2K·Максимум")).toBe(true);
    expect(buttons().some(button=>button.textContent?.trim()==="4K·Максимум")).toBe(true);
    expect(container.textContent).not.toContain("1K-medium");
    await act(async()=>{
      buttons().find(button=>button.textContent?.trim()==="4K·Максимум")!.click();
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({model_id:"gpt_image_2_5_flare",image_quality:"4K-max"}));
    await act(async()=>{container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click();});
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic scene",expect.objectContaining({modelId:"gpt_image_2_5_flare",imageQuality:"4K-max"}));
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("GPT Image 2.5 blocks prompts above 4096 UTF-8 bytes before estimate", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "gpt_image_2_5_sunburst", name: "GPT Image 2.5 Sunburst", enabled: true,
    quality_options: ["1K-medium"], default_quality: "1K-medium", estimate_credits: 30,
    supports_reference_image: false, requires_start_image: false, max_reference_images: 0,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:30,balance_credits:200,enough_credits:true});
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={vi.fn()}/>);});
    vi.mocked(estimateJob).mockClear();
    await act(async()=>{
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"а".repeat(2049));
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(container.textContent).toMatch(/4\s098 \/ 4\s096 байт UTF-8/);
    expect(estimateJob).not.toHaveBeenCalled();
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("Seedream 5 Lite uses catalog quality and the 14-reference cap for one implicit output", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "seedream_5_0_lite", name: "Seedream 5.0 Lite", enabled: true,
    quality_options: ["2K", "3K", "4K"], default_quality: "2K", estimate_credits: 30,
    supports_reference_image: true, requires_start_image: false, max_reference_images: 14,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:30,balance_credits:200,enough_credits:true});
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={vi.fn()}/>);});
    const buttons=()=>Array.from(container.querySelectorAll("button"));
    for (const quality of ["2K", "3K", "4K"]) expect(buttons().some(button=>button.textContent?.trim()===quality)).toBe(true);
    expect(container.textContent).toContain("до 14 файлов");
    await act(async()=>{
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({model_id:"seedream_5_0_lite",image_quality:"2K"}));
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("Seedream 5 Pro uses the backend default quality without forcing 4K", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "seedream_5_0_pro", name: "Seedream 5.0 Pro", enabled: true,
    quality_options: ["1K", "1.5K", "2K"], default_quality: "1.5K", estimate_credits: 30,
    supports_reference_image: true, requires_start_image: false, max_reference_images: 10,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:30,balance_credits:200,enough_credits:true});
  const onCreateJob=vi.fn().mockResolvedValue(null);
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>);});
    expect(Array.from(container.querySelectorAll("button")).some(button=>button.textContent?.trim()==="1.5K")).toBe(true);
    await act(async()=>{
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({model_id:"seedream_5_0_pro",image_quality:"1.5K"}));
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("Midjourney estimates and submits the chosen speed", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "image", id: "midjourney_v7", name: "Midjourney V7", enabled: true,
    quality_options: ["relax", "fast", "turbo"], default_quality: "relax", estimate_credits: 30,
    supports_reference_image: true, requires_start_image: false, max_reference_images: 4,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"image_generate",cost_estimate:35,balance_credits:100,enough_credits:true});
  const onCreateJob=vi.fn().mockResolvedValue(null);
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>);});
    expect(container.querySelector('[aria-label="Режим генерации"]')).not.toBeNull();
    await act(async()=>{
      Array.from(container.querySelectorAll("button")).find(b=>b.textContent?.trim()==="Fast")!.click();
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({model_id:"midjourney_v7",image_quality:"fast"}));
    await act(async()=>{container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]')!.click();});
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic scene",expect.objectContaining({modelId:"midjourney_v7",imageQuality:"fast"}));
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("Seedance exposes every duration and estimates the selected resolution before submit", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_seedance_2_5", alias: "video_seedance_2_5", name: "Seedance 2.5", enabled: true,
    allowed_durations_sec: [5, 10, 15, 30], default_duration_sec: 5,
    allowed_resolutions: ["480p", "720p", "1080p"], default_resolution: "480p",
    allowed_aspect_ratios: ["16:9"], supports_reference_image: true, requires_start_image: false, max_reference_images: 4,
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"video_generate",cost_estimate:6930,balance_credits:10000,enough_credits:true});
  const onCreateJob=vi.fn().mockResolvedValue(null);
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={onCreateJob}/>);});
    const buttons=()=>Array.from(container.querySelectorAll("button"));
    for(const duration of [5,10,15,30]) expect(buttons().some(button=>button.textContent?.trim()===`${duration} сек`)).toBe(true);
    await act(async()=>{
      buttons().find(button=>button.textContent?.trim()==="30 сек")!.click();
      buttons().find(button=>button.textContent?.trim()==="1080p")!.click();
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({video_route_alias:"video_seedance_2_5",duration_sec:30,video_resolution:"1080p"}));
    const submit=container.querySelector<HTMLButtonElement>('button[aria-label="Запустить генерацию"]');
    expect(submit).toBeDefined();
    await act(async()=>{submit!.click();});
    expect(onCreateJob).toHaveBeenCalledWith("Synthetic scene",expect.objectContaining({durationSec:30,videoResolution:"1080p"}));
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});

test("capabilities metadata overrides stale legacy video duration, reference and audio flags", async () => {
  vi.useFakeTimers();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, React });
  vi.mocked(listModelCatalog).mockResolvedValue([{
    type: "video", id: "video_seedance_2_5", alias: "video_seedance_2_5", name: "Seedance 2.5", enabled: true,
    allowed_durations_sec: [5, 10], default_duration_sec: 10,
    allowed_resolutions: ["720p", "1080p"], default_resolution: "1080p",
    allowed_aspect_ratios: ["16:9"], supports_reference_image: true, requires_start_image: false, max_reference_images: 4,
    allowed_reference_image_counts: [4], supports_audio: true,
    capabilities: {
      schema_version: 1,
      api: {
        video: {
          images: { support: "supported", extensions: [], max_count: 10 },
          videos: { support: "unknown", extensions: [], max_count: null },
          allowed_image_counts: [0, 10],
          duration: { mode: "selected", min_seconds: 5, max_seconds: 10, allowed_seconds: [5, 10], by_resolution: { "720p": [5, 10], "1080p": [5] } },
          resolutions: ["720p", "1080p"],
          quality_modes: null,
          aspect_ratios: ["16:9"],
          audio: { mode: "optional", selectable: true },
          start_frame: "optional",
          end_frame: "unsupported",
        },
      },
      application: {
        video: {
          images: { support: "supported", extensions: [], max_count: 10 },
          videos: { support: "unsupported", extensions: [], max_count: 0 },
          allowed_image_counts: [0, 10],
          duration: { mode: "selected", min_seconds: 5, max_seconds: 10, allowed_seconds: [5, 10], by_resolution: { "720p": [5, 10], "1080p": [5] } },
          resolutions: ["720p", "1080p"],
          quality_modes: null,
          aspect_ratios: ["16:9"],
          audio: { mode: "silent", selectable: false },
          start_frame: "optional",
          end_frame: "unsupported",
        },
      },
    },
  }]);
  vi.mocked(estimateJob).mockResolvedValue({operation:"video_generate",cost_estimate:100,balance_credits:1000,enough_credits:true});
  const container=document.createElement("div");document.body.append(container);
  const root=createRoot(container);
  try {
    await act(async()=>{root.render(<WorkflowMode user={{name:"Test",firstName:"Test",avatar:null}} jobs={[]} chats={[]} loading={false} submitting={false} openJobRequest={null} onOpenJobRequestHandled={()=>{}} onCreateJob={vi.fn()}/>);});
    const buttons=()=>Array.from(container.querySelectorAll("button"));
    expect(container.textContent).toContain("до 10 файлов");
    expect(container.querySelector<HTMLInputElement>('input[aria-label="Добавить звук"]')).toBeNull();
    expect(buttons().some(button=>button.textContent?.trim()==="5 сек")).toBe(true);
    expect(buttons().some(button=>button.textContent?.trim()==="10 сек")).toBe(false);
    await act(async()=>{
      const prompt=container.querySelector("textarea")!;
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(prompt,"Synthetic scene");
      prompt.dispatchEvent(new Event("input",{bubbles:true}));
    });
    await act(async()=>{await vi.advanceTimersByTimeAsync(500);});
    expect(estimateJob).toHaveBeenLastCalledWith(expect.objectContaining({video_route_alias:"video_seedance_2_5",duration_sec:5,video_resolution:"1080p"}));
    const estimateCalls = vi.mocked(estimateJob).mock.calls;
    expect(estimateCalls[estimateCalls.length - 1]?.[0].video_audio).toBeUndefined();
  } finally {
    await act(async()=>{root.unmount();});container.remove();
  }
});
