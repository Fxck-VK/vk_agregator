import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { ru } from "@/i18n/ru";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { SlideImageGeneration } from "./SlideImageGeneration";

vi.mock("@/features/models/image-model-catalog-cache", () => ({ loadImageModelCatalog: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

const initialPrompt = "Иллюстрация этапов 3D-моделирования";
const revisedPrompt = "Трёхмерная модель кузова на светлом фоне";
const jobID = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const otherJobID = "c6c83c57-baf5-4c3d-9cae-66ea2727a700";
const artifactID = "4e9defcb-59d7-4d45-bc2e-7cdb770ad729";
const artifactURL = `/web/v1/image-artifacts/${artifactID}`;
const closeLabel = "Закрыть генерацию изображения";
const bytes = Uint8Array.of(137, 80, 78, 71, 13, 10, 26, 10);
const image = { id: artifactID, mime_type: "image/png", size_bytes: bytes.length, width: 1024, height: 1024 };
const job = {
  id: jobID, status: "prepared", prompt: revisedPrompt, model_id: "gpt_image_2", model_name: "GPT Image 2",
  image_quality: "1K", aspect_ratio: "1:1", output_count: 1, cost_estimate: 45,
  created_at: "2026-11-25T08:00:00Z", updated_at: "2026-11-25T08:00:00Z",
};

function mockImageJob(resultID = jobID, resultImage = image, activationInterrupted = false) {
  const writes = vi.mocked(webBrowserMutation)
    .mockResolvedValueOnce(Response.json({ job, balance: 80, can_afford: true }, { status: 201 }));
  if (activationInterrupted) writes.mockRejectedValueOnce(new Error("Activation response interrupted"));
  writes.mockResolvedValueOnce(Response.json({ job: { ...job, status: "succeeded" } }));
  vi.mocked(webBrowserFetch).mockImplementation(async path => {
    if (path === `/web/v1/image-jobs/${jobID}`) return Response.json({ job: { ...job, status: "succeeded" } });
    if (path === `/web/v1/image-jobs/${jobID}/result`) return Response.json({ job_id: resultID, status: "succeeded", artifacts: [resultImage] });
    if (path === artifactURL) return new Response(bytes, { headers: { "Content-Type": "image/png" } });
    throw new Error("Unexpected web API route");
  });
}

async function prepareFromEditor() {
  const description = await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
  expect(description).toHaveValue(initialPrompt);
  expect(screen.getByLabelText(/Стоимость.*40 звёзд/)).toBeVisible();
  expect(webBrowserMutation).not.toHaveBeenCalled();
  fireEvent.change(description, { target: { value: revisedPrompt } });
  fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
  await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
  const [[route, request]] = vi.mocked(webBrowserMutation).mock.calls;
  expect(route).toBe("/web/v1/image-jobs/prepare");
  expect(request.headers).toMatchObject({ "X-Idempotency-Key": expect.any(String) });
  expect(JSON.parse(String(request.body))).toEqual({
    prompt: revisedPrompt, model_id: "gpt_image_2", image_quality: "1K", aspect_ratio: "1:1", output_count: 1,
  });
  expect(screen.getAllByLabelText("45 звёзд").length).toBeGreaterThan(0);
}

function confirmGeneration() {
  fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));
}

describe("SlideImageGeneration", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(loadImageModelCatalog).mockResolvedValue({ items: [{
      id: "gpt_image_2", name: "GPT Image 2", quality_options: ["1K", "2K"], default_quality: "1K",
      allowed_aspect_ratios: ["1:1"], default_aspect_ratio: "1:1", price_by_variant: { "1K:1:1": 40, "2K:1:1": 80 },
      supports_reference_image: false, max_reference_images: 0, max_output_count: 4,
    }] });
  });

  afterEach(cleanup);

  it("prepares the selected model at the nearest priced ratio for the drawn frame", async () => {
    const base = { quality_options: ["1K"], default_quality: "1K", default_aspect_ratio: "1:1", supports_reference_image: false, max_reference_images: 0 };
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce({ items: [
      { ...base, id: "square", name: "Square", allowed_aspect_ratios: ["1:1"], price_by_quality: { "1K": 20 } },
      { ...base, id: "landscape", name: "Landscape", allowed_aspect_ratios: ["1:1", "4:3", "16:9"],
        price_by_variant: { "1K:1:1": 30, "1K:4:3": 40, "1K:16:9": 85 } },
    ] });
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({
      job: { ...job, model_id: "landscape", model_name: "Landscape", aspect_ratio: "16:9", cost_estimate: 90 }, balance: 100, can_afford: true,
    }, { status: 201 }));
    render(<SlideImageGeneration frame={{ x: 1000, y: 2000, width: 4000, height: 4000 }}
      prompt={initialPrompt} onImage={vi.fn()} onClose={vi.fn()} />);
    const description = await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    fireEvent.change(description, { target: { value: revisedPrompt } });
    fireEvent.change(screen.getByRole("combobox", { name: ru.imageGeneration.modelLabel }), { target: { value: "landscape" } });
    expect(description).toHaveValue(revisedPrompt);
    expect(screen.getByText("Формат генерации · 16:9")).toBeVisible();
    expect(screen.getByLabelText(/Стоимость.*85 звёзд/)).toBeVisible();
    expect(webBrowserMutation).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    const [[path, request]] = vi.mocked(webBrowserMutation).mock.calls;
    expect(path).toBe("/web/v1/image-jobs/prepare");
    expect(JSON.parse(String(request.body))).toEqual({
      prompt: revisedPrompt, model_id: "landscape", image_quality: "1K", aspect_ratio: "16:9", output_count: 1,
    });
    expect(screen.getAllByLabelText("90 звёзд").length).toBeGreaterThan(0);
  });

  it("confirms the server price separately and inserts one image from the exact authenticated job", async () => {
    mockImageJob();
    const onImage = vi.fn();
    const onClose = vi.fn();
    render(<SlideImageGeneration prompt={initialPrompt} onImage={onImage} onClose={onClose} />);
    expect(screen.getByRole("dialog")).toBeVisible();
    await prepareFromEditor();
    expect(webBrowserFetch).not.toHaveBeenCalled();
    confirmGeneration();

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(onImage).toHaveBeenCalledTimes(1);
    expect(onImage.mock.calls[0][0]).toMatchObject({ type: "image/png", size: bytes.length });
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
    expect(webBrowserMutation).toHaveBeenLastCalledWith(`/web/v1/image-jobs/${jobID}/activate`, { method: "POST" });
    expect(webBrowserFetch).toHaveBeenLastCalledWith(artifactURL, expect.objectContaining({ redirect: "error" }));
  });

  it("retries adding the same image after an editor failure without another generation", async () => {
    mockImageJob();
    const onImage = vi.fn().mockRejectedValueOnce(new Error("Image cannot be decoded")).mockResolvedValueOnce(undefined);
    const onClose = vi.fn();
    render(<SlideImageGeneration prompt={initialPrompt} onImage={onImage} onClose={onClose} />);
    await prepareFromEditor();
    confirmGeneration();
    await screen.findByRole("alert");
    expect(onImage).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Повторить добавление" }));

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
    expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => path === artifactURL)).toHaveLength(2);
  });

  it("retries the same activation after its response is lost", async () => {
    mockImageJob(jobID, image, true);
    const onImage = vi.fn();
    render(<SlideImageGeneration prompt={initialPrompt} onImage={onImage} onClose={vi.fn()} />);
    await prepareFromEditor();
    confirmGeneration();
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.activationFailure);
    expect(screen.queryByRole("button", { name: "К описанию" })).not.toBeInTheDocument();

    confirmGeneration();
    await waitFor(() => expect(onImage).toHaveBeenCalledTimes(1));
    expect(vi.mocked(webBrowserMutation).mock.calls.map(([path]) => path)).toEqual([
      "/web/v1/image-jobs/prepare", `/web/v1/image-jobs/${jobID}/activate`, `/web/v1/image-jobs/${jobID}/activate`,
    ]);
  });

  it("keeps an activating or queued job open until there is a tracker error", async () => {
    let activate!: (response: Response) => void;
    const activation = new Promise<Response>(resolve => { activate = resolve; });
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 80, can_afford: true }, { status: 201 }))
      .mockReturnValueOnce(activation);
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }));
    const onClose = vi.fn();
    render(<SlideImageGeneration prompt={initialPrompt} onImage={vi.fn()} onClose={onClose} />);
    await prepareFromEditor();
    const close = screen.getByRole("button", { name: closeLabel });
    expect(close).toBeEnabled();
    confirmGeneration();
    expect(close).toBeDisabled();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.getByTestId("landing-dialog-backdrop")).toHaveAttribute("data-state", "open");

    activate(Response.json({ job: { ...job, status: "queued" } }));
    const refresh = await screen.findByRole("button", { name: ru.imageGeneration.statusRefresh });
    expect(close).toBeDisabled();
    fireEvent.click(refresh);
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.statusFailure);
    expect(close).toBeEnabled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("prevents dismissal during insertion and restores it after an attachment failure", async () => {
    mockImageJob();
    let failInsertion!: (reason: Error) => void;
    const insertion = new Promise<void>((_, reject) => { failInsertion = reject; });
    const onImage = vi.fn(() => insertion);
    render(<SlideImageGeneration prompt={initialPrompt} onImage={onImage} onClose={vi.fn()} />);
    await prepareFromEditor();
    confirmGeneration();
    await waitFor(() => expect(onImage).toHaveBeenCalledTimes(1));
    const close = screen.getByRole("button", { name: closeLabel });
    expect(close).toBeDisabled();

    failInsertion(new Error("Image could not be inserted"));
    await screen.findByRole("alert");
    expect(close).toBeEnabled();
  });

  it.each(["mismatched job", "unsafe media"])("does not fetch an artifact with %s", async failure => {
    if (failure === "mismatched job") mockImageJob(otherJobID);
    else mockImageJob(jobID, { ...image, mime_type: "image/svg+xml" });
    const onImage = vi.fn();
    render(<SlideImageGeneration prompt={initialPrompt} onImage={onImage} onClose={vi.fn()} />);
    await prepareFromEditor();
    confirmGeneration();
    await screen.findByRole("alert");
    expect(onImage).not.toHaveBeenCalled();
    expect(vi.mocked(webBrowserFetch).mock.calls.some(([path]) => path === artifactURL)).toBe(false);
  });
});
