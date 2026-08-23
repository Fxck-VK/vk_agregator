import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel", () => ({
  ImageGenerationPanel: () => <p>generator panel</p>,
}));

vi.mock("@/features/image-generation/ImageJobHistory/ImageJobHistory", () => ({
  ImageJobHistory: ({ latestJob }: { latestJob?: { prompt: string } | null }) => <p>{latestJob?.prompt ?? "history panel"}</p>,
}));

vi.mock("@/features/image-generation/ImageGenerationGuide/ImageGenerationGuide", () => ({
  ImageGenerationGuide: () => <p>generation guide</p>,
}));

import { ru } from "@/i18n/ru";

import { ImageWorkspace } from "./ImageWorkspace";

describe("ImageWorkspace", () => {
  afterEach(() => {
    cleanup();
  });

  it("groups the generator and guide without the history panel", () => {
    render(<ImageWorkspace />);

    expect(screen.getByRole("region", { name: ru.imageGeneration.title })).toContainElement(screen.getByText("generator panel"));
    expect(screen.getByText("generation guide")).toBeInTheDocument();
    expect(screen.queryByText("history panel")).not.toBeInTheDocument();
  });

  it("places the guide after the generator", () => {
    render(<ImageWorkspace />);

    const workspace = screen.getByRole("region", { name: ru.imageGeneration.title });
    const visibleSections = Array.from(workspace.querySelectorAll("p")).map((element) => element.textContent);

    expect(visibleSections).toEqual(["generator panel", "generation guide"]);
  });
});
