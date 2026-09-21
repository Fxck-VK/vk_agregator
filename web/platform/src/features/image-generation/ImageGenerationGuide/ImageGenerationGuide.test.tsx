import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
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
    const examplesPanel = screen.getByRole("tabpanel", { name: ru.imageGeneration.guide.examplesTab });
    expect(within(examplesPanel).getAllByRole("listitem")).toHaveLength(6);
    expect(within(examplesPanel).getAllByRole("img")).toHaveLength(6);
    expect(screen.getByRole("button", { name: ru.inspiration.openExample })).toBeInTheDocument();
    expect(screen.queryByTestId("image-generation-example-placeholder")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: ru.imageGeneration.guide.viewMoreExamples })).toHaveAttribute(
      "href",
      "/ru/app/inspiration",
    );
    expect(screen.queryByTestId("image-generation-guide-step")).not.toBeInTheDocument();
  });

  it("opens the shared inspiration dialog from an example card", () => {
    render(<ImageGenerationGuide />);

    fireEvent.click(screen.getByRole("tab", { name: ru.imageGeneration.guide.examplesTab }));
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    expect(screen.getByRole("dialog", { name: ru.inspiration.dialogLabel })).toBeInTheDocument();
    expect(screen.getByText(ru.inspiration.prompt)).toBeInTheDocument();
  });
});
