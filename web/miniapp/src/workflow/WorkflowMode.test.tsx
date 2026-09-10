import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, test, vi } from "vitest";
import { WorkflowMode } from "./WorkflowMode";
import { estimateJob, listModelCatalog } from "../api/client";

vi.mock("@vkontakte/vkui", () => ({
  Button: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props} />,
  NativeSelect: (props: React.SelectHTMLAttributes<HTMLSelectElement>) => <select {...props} />,
}));

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  listModelCatalog: vi.fn(),
  estimateJob: vi.fn(),
}));

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

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
