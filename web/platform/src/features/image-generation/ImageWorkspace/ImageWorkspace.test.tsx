import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel", () => ({
  ImageGenerationPanel: ({ onJobChange }: { onJobChange?: (job: { prompt: string }) => void }) => (
    <>
      <p>generator panel</p>
      <button onClick={() => onJobChange?.({ prompt: "fresh image job" })} type="button">emit image job</button>
    </>
  ),
}));

vi.mock("@/features/image-generation/ImageJobHistory/ImageJobHistory", () => ({
  ImageJobHistory: ({ latestJob }: { latestJob?: { prompt: string } | null }) => <p>{latestJob?.prompt ?? "history panel"}</p>,
}));

import { ru } from "@/i18n/ru";

import { ImageWorkspace } from "./ImageWorkspace";

describe("ImageWorkspace", () => {
  afterEach(() => {
    cleanup();
  });

  it("groups the manual generator and history in a labelled workspace region", () => {
    render(<ImageWorkspace />);

    expect(screen.getByRole("region", { name: ru.imageGeneration.title })).toContainElement(screen.getByText("generator panel"));
    expect(screen.getByRole("region", { name: ru.imageGeneration.title })).toContainElement(screen.getByText("history panel"));
  });

  it("passes only the latest in-memory job from the generator to history", () => {
    render(<ImageWorkspace />);

    fireEvent.click(screen.getByRole("button", { name: "emit image job" }));

    expect(screen.getByText("fresh image job")).toBeInTheDocument();
  });
});
