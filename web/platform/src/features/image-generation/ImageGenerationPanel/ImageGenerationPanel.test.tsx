import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { useSearchParams } from "next/navigation";

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

const modelsResponse = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["1K", "2K"],
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
  ],
};

const multipleModelsResponse = {
  items: [
    {
      id: "first-model",
      name: "First model",
      quality_options: ["1K", "2K"],
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["2K", "4K"],
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
  ],
};

function renderReadyEditor() {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(modelsResponse));
  render(<ImageGenerationPanel />);
  fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.open }));
  return screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
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

  it("keeps the generator closed until the user explicitly opens it", async () => {
    render(<ImageGenerationPanel />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.open }));

    await vi.waitFor(() => expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-models"));
  });

  it("selects a known requested model after the user explicitly opens the generator", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=nano-banana-2") as never);
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(multipleModelsResponse));
    render(<ImageGenerationPanel />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.open }));

    expect(await screen.findByRole("combobox", { name: ru.imageGeneration.modelLabel })).toHaveValue("nano-banana-2");
    expect(screen.getByRole("combobox", { name: ru.imageGeneration.qualityLabel })).toHaveValue("2K");
  });

  it("falls back to the first model and its default quality for an unknown requested model", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=unknown-model") as never);
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(multipleModelsResponse));
    render(<ImageGenerationPanel />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.open }));

    expect(await screen.findByRole("combobox", { name: ru.imageGeneration.modelLabel })).toHaveValue("first-model");
    expect(screen.getByRole("combobox", { name: ru.imageGeneration.qualityLabel })).toHaveValue("1K");
  });

  it("uses only explicit public inputs and shows the server-calculated confirmation", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation).mockResolvedValue(
      Response.json({ job, balance: 104, can_afford: true }, { status: 201 }),
    );

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.change(screen.getByRole("combobox", { name: ru.imageGeneration.qualityLabel }), {
      target: { value: "2K" },
    });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));

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
      }),
    });
    expect(screen.getByText("60 ★")).toBeInTheDocument();
    expect(screen.getByText("104 ★")).toBeInTheDocument();
    expect(screen.queryByText(/provider|model_code|pricing_snapshot/i)).not.toBeInTheDocument();
  });

  it("preserves the prepared job and shows a neutral balance state after a 402", async () => {
    const promptInput = await renderReadyEditor();
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ job, balance: 10, can_afford: false }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job: { ...job, status: "awaiting_payment" } }, { status: 402 }));

    fireEvent.change(promptInput, { target: { value: job.prompt } });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.insufficientBalance);
    expect(screen.getByText("60 ★")).toBeInTheDocument();
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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));

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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));

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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    await screen.findByRole("textbox", { name: ru.imageGeneration.promptLabel });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));

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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.activationFailure);
    expect(screen.getByRole("heading", { name: ru.imageGeneration.confirmationTitle })).toBeInTheDocument();
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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
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
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.prepare }));
    await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));
    await screen.findByRole("heading", { name: ru.imageGeneration.statusTitle });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.imageGeneration.resultFailure);
    expect(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh })).toBeInTheDocument();
    expect(screen.queryByText(ru.imageGeneration.statusFailure)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }));

    expect(await screen.findByRole("img", { name: ru.imageGeneration.resultImageAlt })).toHaveAttribute(
      "src",
      "/web/v1/image-artifacts/4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
    );
  });
});
