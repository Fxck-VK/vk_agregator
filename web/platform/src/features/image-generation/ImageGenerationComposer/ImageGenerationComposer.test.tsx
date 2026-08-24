import { useState } from "react";

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";
import { inspirationExamples } from "@/features/inspiration/inspiration-examples";

import { ImageGenerationComposer } from "./ImageGenerationComposer";

describe("ImageGenerationComposer", () => {
  afterEach(() => {
    cleanup();
  });

  it("edits the prompt, changes quality, and submits through the shared composer", () => {
    const onImageQualityChange = vi.fn();
    const onPromptChange = vi.fn();
    const onSubmit = vi.fn();

    function StatefulComposer() {
      const [prompt, setPrompt] = useState("");
      return (
        <ImageGenerationComposer
          aspectRatio="16:9"
          canSubmit={prompt.trim() !== ""}
          errorMessage={null}
          imageQuality="2K"
          isSubmitting={false}
          maxOutputCount={4}
          onAspectRatioChange={vi.fn()}
          onImageQualityChange={onImageQualityChange}
          onOutputCountChange={vi.fn()}
          onPromptChange={(value) => {
            setPrompt(value);
            onPromptChange(value);
          }}
          onSubmit={onSubmit}
          price={60}
          prompt={prompt}
          qualityOptions={["1K", "2K"]}
          outputCount={1}
        />
      );
    }

    const { container } = render(<StatefulComposer />);

    expect(container.querySelectorAll('[data-ui="input-control-chip"]')).toHaveLength(5);

    fireEvent.change(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel }), {
      target: { value: "night city after rain" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Разрешение: 2K" }));
    expect(screen.getByRole("dialog", { name: "Разрешение" })).toBeVisible();
    fireEvent.click(screen.getByRole("radio", { name: "1K" }));
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));

    expect(onPromptChange).toHaveBeenCalledWith("night city after rain");
    expect(onImageQualityChange).toHaveBeenCalledWith("1K");
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText(`${ru.imageGeneration.priceLabel}: 60 звёзд`)).toBeVisible();
    expect(screen.getByTestId("credit-star-icon")).toBeInTheDocument();
    expect(container).not.toHaveTextContent("\u2605");
  });

  it("fills the prompt from a shared Inspiration template without leaving the composer", () => {
    function StatefulComposer() {
      const [prompt, setPrompt] = useState("");
      return (
        <ImageGenerationComposer
          aspectRatio="16:9"
          canSubmit={prompt.trim() !== ""}
          errorMessage={null}
          imageQuality="1K"
          isSubmitting={false}
          maxOutputCount={4}
          onAspectRatioChange={vi.fn()}
          onImageQualityChange={vi.fn()}
          onOutputCountChange={vi.fn()}
          onPromptChange={setPrompt}
          onSubmit={vi.fn()}
          price={50}
          prompt={prompt}
          qualityOptions={["1K"]}
          outputCount={1}
        />
      );
    }

    render(<StatefulComposer />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open }));
    expect(
      screen.getByRole("dialog", { name: ru.imageGeneration.templatePicker.title }),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", {
        name: `${ru.imageGeneration.templatePicker.select} ${inspirationExamples[0].title}`,
      }),
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel })).toHaveValue(
      inspirationExamples[0].prompt,
    );
  });

  it("disables mutable controls while preparing and reports an unavailable price", () => {
    render(
      <ImageGenerationComposer
        aspectRatio="16:9"
        canSubmit={false}
        errorMessage={null}
        imageQuality="1K"
        isSubmitting
        maxOutputCount={4}
        onAspectRatioChange={vi.fn()}
        onImageQualityChange={vi.fn()}
        onOutputCountChange={vi.fn()}
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        price={null}
        prompt="night city after rain"
        qualityOptions={["1K", "2K"]}
        outputCount={1}
      />,
    );

    expect(screen.getByRole("textbox", { name: ru.imageGeneration.promptLabel })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Разрешение: 1K" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Соотношение сторон: 16:9" })).toBeDisabled();
    expect(screen.getByRole("button", { name: ru.imageGeneration.preparing })).toBeDisabled();
    expect(screen.getByText(ru.imageGeneration.priceUnavailable)).toBeVisible();
  });

  it("renders workflow errors below the compact composer", () => {
    render(
      <ImageGenerationComposer
        aspectRatio="16:9"
        canSubmit
        errorMessage={ru.imageGeneration.prepareFailure}
        imageQuality="1K"
        isSubmitting={false}
        maxOutputCount={4}
        onAspectRatioChange={vi.fn()}
        onImageQualityChange={vi.fn()}
        onOutputCountChange={vi.fn()}
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        price={16}
        prompt="night city after rain"
        qualityOptions={["1K"]}
        outputCount={1}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(ru.imageGeneration.prepareFailure);
  });
});
