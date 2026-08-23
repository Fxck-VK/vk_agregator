import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ImageGenerationGuide } from "./ImageGenerationGuide";

describe("ImageGenerationGuide", () => {
  afterEach(() => {
    cleanup();
  });

  it("shows the three generation steps by default", () => {
    render(<ImageGenerationGuide />);

    expect(screen.getByRole("tab", { name: ru.imageGeneration.guide.howToTab })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getAllByTestId("image-generation-guide-step")).toHaveLength(3);
    expect(screen.getByText(ru.imageGeneration.guide.steps[0].title)).toBeInTheDocument();
    expect(screen.getByText(ru.imageGeneration.guide.steps[1].title)).toBeInTheDocument();
    expect(screen.getByText(ru.imageGeneration.guide.steps[2].title)).toBeInTheDocument();
  });

  it("switches between the guide and generation examples", () => {
    render(<ImageGenerationGuide />);

    fireEvent.click(screen.getByRole("tab", { name: ru.imageGeneration.guide.examplesTab }));

    expect(screen.getByRole("tab", { name: ru.imageGeneration.guide.examplesTab })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tabpanel", { name: ru.imageGeneration.guide.examplesTab })).toBeInTheDocument();
    expect(screen.getAllByTestId("image-generation-example-placeholder")).toHaveLength(3);
    expect(screen.queryByTestId("image-generation-guide-step")).not.toBeInTheDocument();
  });
});
