import { useState } from "react";

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ImageGenerationEditor } from "./ImageGenerationEditor";

const models = [
  {
    id: "nano-banana-2",
    name: "Nano Banana 2",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 16, "2K": 60 },
    default_quality: "1K",
    supports_reference_image: false,
    max_reference_images: 0,
  },
];

describe("ImageGenerationEditor", () => {
  afterEach(() => {
    cleanup();
  });

  it("uses only supported model fields and reports the edited prompt", () => {
    const onPromptChange = vi.fn();
    const onSubmit = vi.fn();

    function StatefulEditor() {
      const [prompt, setPrompt] = useState("");
      return (
        <ImageGenerationEditor
          canSubmit
          errorMessage={null}
          imageQuality="2K"
          isSubmitting={false}
          modelID="nano-banana-2"
          models={models}
          onImageQualityChange={vi.fn()}
          onModelChange={vi.fn()}
          onPromptChange={(value) => {
            onPromptChange(value);
            setPrompt(value);
          }}
          onSubmit={onSubmit}
          price={60}
          prompt={prompt}
        />
      );
    }

    render(
      <StatefulEditor />,
    );

    fireEvent.change(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel }), {
      target: { value: "night city after rain" },
    });
    expect(screen.getByText(`${ru.imageGeneration.priceLabel}: 60 \u2605`)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: `${ru.imageGeneration.generate} \u00b7 60 \u2605` }));

    expect(onPromptChange).toHaveBeenCalledWith("night city after rain");
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/provider|pricing_snapshot|model_code/i)).not.toBeInTheDocument();
  });

  it("disables mutable inputs while the server prepares the quote", () => {
    render(
      <ImageGenerationEditor
        canSubmit={false}
        errorMessage={null}
        imageQuality="1K"
        isSubmitting
        modelID="nano-banana-2"
        models={models}
        onImageQualityChange={vi.fn()}
        onModelChange={vi.fn()}
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        price={16}
        prompt="night city after rain"
      />,
    );

    expect(screen.getByRole("combobox", { name: ru.imageGeneration.modelLabel })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: ru.imageGeneration.qualityLabel })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel })).toBeDisabled();
    expect(screen.getByRole("button", { name: ru.imageGeneration.preparing })).toBeDisabled();
  });

  it("does not offer a submit action when the selected quality has no public price", () => {
    render(
      <ImageGenerationEditor
        canSubmit={false}
        errorMessage={null}
        imageQuality="2K"
        isSubmitting={false}
        modelID="nano-banana-2"
        models={models}
        onImageQualityChange={vi.fn()}
        onModelChange={vi.fn()}
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        price={null}
        prompt="night city after rain"
      />,
    );

    expect(screen.getByText(ru.imageGeneration.priceUnavailable)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.imageGeneration.generate })).toBeDisabled();
  });
});
