import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel", () => ({
  ImageGenerationPanel: () => <p>generator panel</p>,
}));

vi.mock("@/features/image-generation/ImageJobHistory/ImageJobHistory", () => ({
  ImageJobHistory: () => <p>history panel</p>,
}));

import { ru } from "@/i18n/ru";

import { ImageWorkspace } from "./ImageWorkspace";

describe("ImageWorkspace", () => {
  it("groups the manual generator and history in a labelled workspace region", () => {
    render(<ImageWorkspace />);

    expect(screen.getByRole("region", { name: ru.imageGeneration.title })).toContainElement(screen.getByText("generator panel"));
    expect(screen.getByRole("region", { name: ru.imageGeneration.title })).toContainElement(screen.getByText("history panel"));
  });
});
