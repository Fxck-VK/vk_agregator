import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ImageGenerationResult } from "./ImageGenerationResult";

const result = {
  job_id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "succeeded" as const,
  artifacts: [
    {
      id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
      mime_type: "image/png",
      size_bytes: 42,
      width: 1024,
      height: 1024,
    },
  ],
};

describe("ImageGenerationResult", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders only a same-origin artifact link and lets the user repeat the prompt", () => {
    const onCreateAnother = vi.fn();
    render(
      <ImageGenerationResult
        onCreateAnother={onCreateAnother}
        prompt="night city after rain"
        result={result}
      />,
    );

    const image = screen.getByRole("img", { name: ru.imageGeneration.resultImageAlt });
    expect(image).toHaveAttribute("src", "/web/v1/image-artifacts/4e9defcb-59d7-4d45-bc2e-7cdb770ad729");
    const download = screen.getByRole("link", { name: ru.imageGeneration.downloadResult });
    expect(download).toHaveAttribute("href", "/web/v1/image-artifacts/4e9defcb-59d7-4d45-bc2e-7cdb770ad729");
    expect(download).toHaveAttribute("download");

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.createAnother }));
    expect(onCreateAnother).toHaveBeenCalledWith("night city after rain");
    expect(screen.queryByText("https://objects.example.test/private-key")).not.toBeInTheDocument();
  });

  it("copies only the user prompt locally", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(
      <ImageGenerationResult
        onCreateAnother={vi.fn()}
        prompt="night city after rain"
        result={result}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.copyPrompt }));

    expect(writeText).toHaveBeenCalledWith("night city after rain");
    expect(await screen.findByText(ru.imageGeneration.promptCopied)).toBeInTheDocument();
  });
});
