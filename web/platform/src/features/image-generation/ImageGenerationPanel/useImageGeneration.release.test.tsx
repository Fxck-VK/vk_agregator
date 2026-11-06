import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { imageModelFixture } from "@/test/model-catalog";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { useImageGeneration } from "./useImageGeneration";
import type { ImageModelList } from "@/lib/web-api/contracts";

const release = vi.hoisted(() => ({ view: "dev" as "dev" | "prod" }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => release }));
vi.mock("@/features/models/image-model-catalog-cache", () => ({ loadImageModelCatalog: vi.fn() }));
vi.mock("@/features/models/WorkspaceModelSelection/WorkspaceModelSelection", () => ({ useWorkspaceModelSelection: () => null }));
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

const model = imageModelFixture({ id: "shared-image", name: "Shared image", quality_options: ["1K", "2K"], default_quality: "1K", price_by_quality: { "1K": 10, "2K": 20 }, supports_reference_image: false, max_reference_images: 0, max_output_count: 4, allowed_aspect_ratios: ["1:1", "16:9"] });
const initialValues = { modelID: model.id, imageQuality: "1K", prompt: "Initial seed" };
function deferred<T>() { let resolve!: (value: T) => void, reject!: (reason: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
beforeEach(() => { release.view = "dev"; vi.mocked(loadImageModelCatalog).mockReset(); });
afterEach(cleanup);

it("reloads standalone models per view, hides the previous list while pending and preserves draft/settings", async () => {
  const next = deferred<ImageModelList>();
  vi.mocked(loadImageModelCatalog).mockResolvedValueOnce({ items: [model] }).mockReturnValueOnce(next.promise);
  const { result, rerender } = renderHook(() => useImageGeneration({ initialValues }));
  await waitFor(() => expect(result.current.stage).toBe("editor"));
  act(() => { result.current.changePrompt("Typed draft"); result.current.composerProps.onImageQualityChange("2K"); result.current.composerProps.onAspectRatioChange("16:9"); result.current.composerProps.onOutputCountChange(3); });
  release.view = "prod";
  rerender();
  expect(loadImageModelCatalog).toHaveBeenCalledTimes(2);
  expect(result.current.selectedModel).toBeNull();
  expect(result.current.canPrepare).toBe(false);
  expect(result.current.prompt).toBe("Typed draft");
  await act(async () => { next.resolve({ items: [model] }); });
  expect(result.current.selectedModel?.id).toBe(model.id);
  expect(result.current.prompt).toBe("Typed draft");
  expect(result.current.composerProps.imageQuality).toBe("2K");
  expect(result.current.composerProps.aspectRatio).toBe("16:9");
  expect(result.current.composerProps.outputCount).toBe(3);
});

it("fails closed after the new view read fails without replacing the typed draft", async () => {
  vi.mocked(loadImageModelCatalog).mockResolvedValueOnce({ items: [model] }).mockRejectedValueOnce(new Error("Unavailable"));
  const { result, rerender } = renderHook(() => useImageGeneration({ initialValues }));
  await waitFor(() => expect(result.current.stage).toBe("editor"));
  act(() => result.current.changePrompt("Keep this draft"));
  release.view = "prod"; rerender();
  await waitFor(() => expect(result.current.stage).toBe("loadFailure"));
  expect(result.current.selectedModel).toBeNull();
  expect(result.current.canPrepare).toBe(false);
  expect(result.current.prompt).toBe("Keep this draft");
});

it("ignores a late DEV catalogue after the PROD catalogue has loaded", async () => {
  const old = deferred<ImageModelList>();
  vi.mocked(loadImageModelCatalog).mockReturnValueOnce(old.promise).mockResolvedValueOnce({ items: [model] });
  const { result, rerender } = renderHook(() => useImageGeneration({ initialValues }));
  release.view = "prod"; rerender();
  await waitFor(() => expect(result.current.selectedModel?.id).toBe(model.id));
  await act(async () => { old.resolve({ items: [{ ...model, id: "dev-only", name: "DEV only" }] }); });
  expect(result.current.selectedModel?.id).toBe(model.id);
});

it("keeps old models hidden when returning to DEV before PROD has resolved", async () => {
  const prod = deferred<ImageModelList>();
  const dev = deferred<ImageModelList>();
  vi.mocked(loadImageModelCatalog).mockResolvedValueOnce({ items: [model] }).mockReturnValueOnce(prod.promise).mockReturnValueOnce(dev.promise);
  const { result, rerender } = renderHook(() => useImageGeneration({ initialValues }));
  await waitFor(() => expect(result.current.stage).toBe("editor"));
  release.view = "prod"; rerender();
  release.view = "dev"; rerender();
  expect(result.current.selectedModel).toBeNull();
  expect(result.current.stage).toBe("loading");
  await act(async () => { prod.resolve({ items: [model] }); });
  expect(result.current.selectedModel).toBeNull();
  await act(async () => { dev.resolve({ items: [model] }); });
  expect(result.current.selectedModel?.id).toBe(model.id);
});
