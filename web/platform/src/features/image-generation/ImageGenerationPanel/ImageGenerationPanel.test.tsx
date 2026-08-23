import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: vi.fn(),
}));

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import type { ImageModelList } from "@/lib/web-api/contracts";
import { useSearchParams } from "next/navigation";

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import {
  useWorkspaceModelSelection,
  WorkspaceModelSelectionProvider,
} from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";

import { ImageGenerationPanel } from "./ImageGenerationPanel";

const job = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "prepared",
  prompt: "night city after rain",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  image_quality: "2K",
  cost_estimate: 60,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
};

const modelsResponse: ImageModelList = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 16, "2K": 60 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
  ],
};

const multipleModelsResponse: ImageModelList = {
  items: [
    {
      id: "first-model",
      name: "First model",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 12, "2K": 24 },
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
      max_output_count: 4,
    },
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["2K", "4K"],
      price_by_quality: { "2K": 60, "4K": 120 },
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
  ],
};

function renderReadyEditor() {
  vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(modelsResponse);
  render(<ImageGenerationPanel />);
  return screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
}

function getGenerateButton() {
  return screen.getByRole("button", { name: new RegExp(`^${ru.imageGeneration.generate}`) });
}

function getQualityButton(value: string) {
  return screen.getByRole("button", { name: `Разрешение: ${value}` });
}

function selectQuality(value: string) {
  fireEvent.click(screen.getByRole("button", { name: /^Разрешение:/ }));
  fireEvent.click(screen.getByRole("radio", { name: value }));
}

function WorkspaceModelSelectionProbe() {
  const selection = useWorkspaceModelSelection();

  return (
    <>
      <output>{selection?.selectedModelId ?? "none"}</output>
      <button onClick={() => selection?.setSelectedModelId("nano-banana-2")} type="button">
        Выбрать Nano Banana 2
      </button>
    </>
  );
}

describe("ImageGenerationPanel", () => {
  beforeEach(() => {
	vi.resetAllMocks();
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams() as never);
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn().mockReturnValue("11111111-1111-4111-8111-111111111111"),
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("loads the compact composer without a duplicated model selector", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(modelsResponse);
    render(<ImageGenerationPanel />);

    expect(await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel })).toBeEnabled();
    expect(screen.queryByRole("combobox", { name: ru.imageGeneration.modelLabel })).not.toBeInTheDocument();
    expect(getQualityButton("1K")).toBeVisible();
    expect(loadImageModelCatalog).toHaveBeenCalledTimes(1);
  });

  it("shows the selected model and quality price immediately without preparing a job", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(modelsResponse);
    render(<ImageGenerationPanel />);

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 16 \u2605`)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.imageGeneration.generate })).toBeDisabled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("updates the preview price for a quality change but not for a prompt edit", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(multipleModelsResponse);
    render(<ImageGenerationPanel />);

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 12 \u2605`)).toBeInTheDocument();

    selectQuality("2K");
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 24 \u2605`)).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel }), { target: { value: "new prompt" } });
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 24 \u2605`)).toBeInTheDocument();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("updates the total price and sends the selected output count", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation).mockResolvedValue(
      Response.json({ job: { ...job, cost_estimate: 32 }, balance: 104, can_afford: true }, { status: 201 }),
    );

    expect(screen.getByText("1 / 4")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.increaseOutputCount }));
    expect(screen.getByText("2 / 4")).toBeVisible();
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 32 \u2605`)).toBeVisible();

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());

    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenCalledWith(
      "/web/v1/image-jobs/prepare",
      expect.objectContaining({
        body: JSON.stringify({
          prompt: job.prompt,
          model_id: "nano-banana-2",
          image_quality: "1K",
          aspect_ratio: "16:9",
          output_count: 2,
        }),
      }),
    );
  });

  it("publishes the initial image model to the persistent workspace selection", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(multipleModelsResponse);
    render(
      <WorkspaceModelSelectionProvider>
        <ImageGenerationPanel />
        <WorkspaceModelSelectionProbe />
      </WorkspaceModelSelectionProvider>,
    );
    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    expect(screen.getByText("first-model", { selector: "output" })).toBeInTheDocument();
  });

  it("adopts a valid model selected from the floating workspace selector", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(multipleModelsResponse);
    render(
      <WorkspaceModelSelectionProvider>
        <ImageGenerationPanel />
        <WorkspaceModelSelectionProbe />
      </WorkspaceModelSelectionProvider>,
    );
    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });

    fireEvent.click(screen.getByRole("button", { name: "Выбрать Nano Banana 2" }));

    await waitFor(() => {
      expect(getQualityButton("2K")).toBeVisible();
    });
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 60 \u2605`)).toBeInTheDocument();
  });

  it("selects a known requested model when the direct editor loads", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=nano-banana-2") as never);
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(multipleModelsResponse);
    render(<ImageGenerationPanel />);

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    expect(getQualityButton("2K")).toBeVisible();
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 60 \u2605`)).toBeInTheDocument();
  });

  it("restores every image setting from an expired-generation retry link", async () => {
    vi.mocked(useSearchParams).mockReturnValue(
      new URLSearchParams("model=nano-banana-2&quality=2K&prompt=night+city+after+rain") as never,
    );
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(modelsResponse);
    render(<ImageGenerationPanel />);

    expect(await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel })).toHaveValue("night city after rain");
    expect(getQualityButton("2K")).toBeVisible();
  });

  it("falls back to the first model and its default quality for an unknown requested model", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=unknown-model") as never);
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(multipleModelsResponse);
    render(<ImageGenerationPanel />);

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    expect(getQualityButton("1K")).toBeVisible();
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 12 \u2605`)).toBeInTheDocument();
  });

  it("uses only explicit public inputs and shows the server-calculated confirmation", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation).mockResolvedValue(
      Response.json({ job, balance: 104, can_afford: true }, { status: 201 }),
    );

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    selectQuality("2K");
    fireEvent.click(screen.getByRole("button", { name: "Соотношение сторон: 16:9" }));
    fireEvent.click(screen.getByRole("radio", { name: "4:5" }));
    fireEvent.click(getGenerateButton());

    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/image-jobs/prepare", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Idempotency-Key": "11111111-1111-4111-8111-111111111111",
      },
      body: JSON.stringify({
        prompt: job.prompt,
        model_id: "nano-banana-2",
        image_quality: "2K",
        aspect_ratio: "4:5",
        output_count: 1,
      }),
    });
    expect(screen.getAllByLabelText("60 звёзд").length).toBeGreaterThan(0);
    expect(screen.getByLabelText("104 звезды")).toBeInTheDocument();
    expect(screen.queryByText(/provider|model_code|pricing_snapshot/i)).not.toBeInTheDocument();
  });

  it("preserves the prepared job and shows a neutral balance state after a 402", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 10, can_afford: false }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "awaiting_payment" } }, { status: 402 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.insufficientBalance);
    expect(screen.getAllByLabelText("60 звёзд").length).toBeGreaterThan(0);
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
  });

  it("clears an expired preparation intent after a 409 so the next prepare uses a new key", async () => {
    vi.mocked(crypto.randomUUID)
      .mockReturnValueOnce("11111111-1111-4111-8111-111111111111")
      .mockReturnValueOnce("22222222-2222-4222-8222-222222222222");
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ error: "image generation preparation conflict" }, { status: 409 }))
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("alert");
    fireEvent.click(getGenerateButton());

    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "11111111-1111-4111-8111-111111111111" }),
    }));
    expect(webBrowserMutation).toHaveBeenNthCalledWith(2, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "22222222-2222-4222-8222-222222222222" }),
    }));
  });

  it("keeps a preparation intent after a transient prepare failure", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ error: "temporarily unavailable" }, { status: 503 }))
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("alert");
    fireEvent.click(getGenerateButton());

    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "11111111-1111-4111-8111-111111111111" }),
    }));
    expect(webBrowserMutation).toHaveBeenNthCalledWith(2, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "11111111-1111-4111-8111-111111111111" }),
    }));
  });

  it("clears an expired confirmation after a 409 and starts a new preparation", async () => {
    vi.mocked(crypto.randomUUID)
      .mockReturnValueOnce("11111111-1111-4111-8111-111111111111")
      .mockReturnValueOnce("22222222-2222-4222-8222-222222222222");
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ error: "image generation preparation conflict" }, { status: 409 }))
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    fireEvent.click(getGenerateButton());

    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "11111111-1111-4111-8111-111111111111" }),
    }));
    expect(webBrowserMutation).toHaveBeenNthCalledWith(3, "/web/v1/image-jobs/prepare", expect.objectContaining({
      headers: expect.objectContaining({ "X-Idempotency-Key": "22222222-2222-4222-8222-222222222222" }),
    }));
  });

  it("keeps the confirmation after a transient activation failure", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ error: "temporarily unavailable" }, { status: 503 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.activationFailure);
    expect(screen.getByRole("heading", { name: ru.imageGeneration.confirmationTitle })).toBeInTheDocument();
  });

  it("emits the activated job without reloading the workspace", async () => {
    const onJobChange = vi.fn();
    vi.mocked(loadImageModelCatalog).mockResolvedValueOnce(modelsResponse);
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "queued" } }, { status: 200 }));
    render(<ImageGenerationPanel onJobChange={onJobChange} />);

    fireEvent.change(await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel }), {
      target: { value: job.prompt },
    });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    await vi.waitFor(() => expect(onJobChange).toHaveBeenCalledWith(expect.objectContaining({ status: "queued" })));
  });

  it("reads the submitted job until it succeeds and renders only a platform artifact URL", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "queued" } }, { status: 200 }));
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "succeeded" } }))
      .mockResolvedValueOnce(
        Response.json({
          job_id: job.id,
          status: "succeeded",
          artifacts: [
            {
              id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
              mime_type: "image/png",
              size_bytes: 42,
              width: 1024,
              height: 1024,
            },
          ],
        }),
      );

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));
    await screen.findByRole("heading", { name: ru.imageGeneration.statusTitle });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }));

    const image = await screen.findByRole("img", { name: ru.imageGeneration.resultImageAlt });
    expect(image).toHaveAttribute("src", "/web/v1/image-artifacts/4e9defcb-59d7-4d45-bc2e-7cdb770ad729");
    expect(screen.queryByRole("button", { name: ru.imageGeneration.statusRefresh })).not.toBeInTheDocument();
    expect(screen.queryByText("https://objects.example.test/private-key")).not.toBeInTheDocument();
  });

  it("keeps manual refresh available when a successful job result fails schema validation", async () => {
    const promptInput = await renderReadyEditor();
    const artifact = {
      id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
      mime_type: "image/png",
      size_bytes: 42,
      width: 1024,
      height: 1024,
    };
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 104, can_afford: true }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "queued" } }, { status: 200 }));
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json({ job_id: job.id, status: "succeeded", artifacts: [] }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json({ job_id: job.id, status: "succeeded", artifacts: [artifact] }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(getGenerateButton());
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));
    await screen.findByRole("heading", { name: ru.imageGeneration.statusTitle });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.resultFailure);
    await waitFor(() => expect(webBrowserFetch).toHaveBeenCalledTimes(2));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    expect(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh })).toBeInTheDocument();
    expect(screen.queryByText(ru.imageGeneration.statusFailure)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }));

    expect(await screen.findByRole("img", { name: ru.imageGeneration.resultImageAlt })).toHaveAttribute(
      "src",
      "/web/v1/image-artifacts/4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
    );
  });
});
