import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { InspirationGallery } from "./InspirationGallery";

describe("InspirationGallery", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("opens one inspiration example in a named modal dialog", () => {
    render(<InspirationGallery />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    expect(screen.getByRole("dialog", { name: ru.inspiration.dialogLabel })).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: ru.inspiration.exampleAlt })).toHaveLength(3);
    expect(screen.getByText(ru.inspiration.prompt)).toBeInTheDocument();
  });

  it("closes with Escape, restores page scrolling and returns focus to the card", async () => {
    render(<InspirationGallery />);
    const card = screen.getByRole("button", { name: ru.inspiration.openExample });

    fireEvent.click(card);
    expect(document.body.style.overflow).toBe("hidden");

    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.body.style.overflow).toBe("");
    await waitFor(() => expect(card).toHaveFocus());
  });

  it("copies the public example prompt", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.copyPrompt }));

    expect(writeText).toHaveBeenCalledWith(ru.inspiration.prompt);
    expect(await screen.findByText(ru.inspiration.copied)).toBeInTheDocument();
  });

  it("offers a local download and prefilled image generator without starting a paid task", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const download = screen.getByRole("link", { name: ru.inspiration.download });
    expect(download).toHaveAttribute("href", "/assets/images/inspiration/paper-crane-cloud.png");
    expect(download).toHaveAttribute("download", "neirohub-paper-crane-cloud.png");

    const recreate = screen.getByRole("link", { name: ru.inspiration.recreate });
    const recreateUrl = new URL(recreate.getAttribute("href")!, "https://neirohub.test");
    expect(recreateUrl.pathname).toBe("/app/image");
    expect(recreateUrl.searchParams.get("model")).toBe("gpt-image-2");
    expect(recreateUrl.searchParams.get("quality")).toBe("1K");
    expect(recreateUrl.searchParams.get("prompt")).toBe(ru.inspiration.prompt);
  });
});
