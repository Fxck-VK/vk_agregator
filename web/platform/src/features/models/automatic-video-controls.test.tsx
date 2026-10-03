import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import catalog from "@/features/session/model-catalog.preview.json";
import { parseModelCatalog } from "./model-catalog-contract";
import { resolveGenerationOptions, useGenerationControls } from "./generation-options";
import type { GenerationModel } from "./generation-model-catalog";

afterEach(cleanup);

it("shows automatic duration with a fixed quote and repairs a saved manual duration", () => {
  const candidate = parseModelCatalog(catalog).items.find(item => item.id === "gemini_omni_flash_preview")!;
  const video = { ...candidate.operations[0].video!, id: candidate.id, name: candidate.name, description: candidate.description, category: "video" } as GenerationModel;
  const resolved = resolveGenerationOptions(video, { duration_sec: 5 });
  expect(resolved.options.duration_sec).toBe(10);
  expect(resolved.cost).toBe(530);
  function Controls() { return useGenerationControls(video, false, () => {}).controls; }
  render(<Controls />);
  expect(screen.getByRole("button", { name: /Авто, 3–10 с · фиксированная цена/ })).toBeDisabled();
  expect(screen.queryByRole("button", { name: "Длительность: 10 с" })).not.toBeInTheDocument();
});

it("hides resolution choices for automatic video resolution while ordinary video keeps them", () => {
  const base = {
    category: "video" as const,
    description: "Video",
    id: "runway_gen4_5",
    name: "Runway",
    allowed_resolutions: ["720p"],
    allowed_durations_sec: [5],
    allowed_aspect_ratios: ["16:9"],
    default_resolution: "720p",
    default_duration_sec: 5,
    default_aspect_ratio: "16:9",
    price_by_option: { "720p:5": 40 },
    variants: [{ resolution: "720p", duration_sec: 5, aspect_ratio: "16:9", fps: null, audio: null }],
  } satisfies Extract<GenerationModel, { category: "video" }>;
  function Controls({ model }: { model: GenerationModel }) {
    return useGenerationControls(model, false, () => {}).controls;
  }

  const { rerender } = render(<Controls model={{ ...base, automatic_resolution: true }} />);

  expect(resolveGenerationOptions({ ...base, automatic_resolution: true }, { resolution: "1080p" }).options).toMatchObject({ resolution: "720p" });
  expect(screen.queryByRole("button", { name: /^Разрешение:/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Длительность: 5 с" })).toBeEnabled();

  rerender(<Controls model={{ ...base, id: "veo_manual", allowed_resolutions: ["720p", "1080p"] }} />);

  expect(screen.getByRole("button", { name: "Разрешение: 720p" })).toBeEnabled();
});
