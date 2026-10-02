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
