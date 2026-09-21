import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { inspirationExamples } from "../inspiration-examples";
import { InspirationGallery } from "./InspirationGallery";

describe("InspirationGallery", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("omits the standalone gallery eyebrow", () => {
    const { container } = render(<InspirationGallery />);

    expect(screen.queryByText("Галерея NeiroHub")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: ru.inspiration.title })).toBeInTheDocument();
    expect(container.querySelectorAll("video")).toHaveLength(2);
  });

  it("uses the shared fallback artwork for a model without a logo", () => {
    render(<InspirationGallery />);

    const card = screen.getByRole("button", { name: ru.inspiration.openExample });

    expect(within(card).getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(card).not.toHaveTextContent("✦");
  });

  it("plays a video card from the beginning only while it is hovered", () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    render(<InspirationGallery />);

    const card = screen.getByRole("button", { name: "Открыть пример «Видеопример 1»" });
    const video = card.querySelector("video")!;
    video.currentTime = 4;

    expect(video).not.toHaveAttribute("autoplay");
    fireEvent.mouseEnter(card);
    expect(video.currentTime).toBe(0);
    expect(play).toHaveBeenCalledTimes(1);

    video.currentTime = 3;
    fireEvent.mouseLeave(card);
    expect(pause).toHaveBeenCalledTimes(1);
    expect(video.currentTime).toBe(0);
  });

  it("opens one inspiration example in a named modal dialog", () => {
    render(<InspirationGallery />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    expect(screen.getByRole("dialog", { name: ru.inspiration.dialogLabel })).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: ru.inspiration.exampleAlt })).toHaveLength(3);
    expect(screen.getByText(ru.inspiration.prompt)).toBeInTheDocument();
  });

  it("uses the shared floating scrollbar for the preview information panel", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const infoPanel = within(dialog).getByText(ru.inspiration.modelName).closest("aside");
    const scrollRoot = infoPanel?.closest('[data-orientation="vertical"]');

    expect(infoPanel).toHaveAttribute("data-scroll-area-viewport", "true");
    expect(scrollRoot).toBeInstanceOf(HTMLElement);
    if (!(scrollRoot instanceof HTMLElement)) {
      throw new Error("Expected the information panel scroll root");
    }
    expect(scrollRoot).toContainElement(within(scrollRoot).getByTestId("floating-scrollbar-track"));
  });

  it("shows only the model icon and model name in the preview header", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const imageExampleIndex = inspirationExamples.findIndex(
      (example) => example.quality === "Изображение",
    );
    const videoExampleIndex = inspirationExamples.findIndex(
      (example) => example.quality === "Видео",
    );

    expect(within(dialog).getByText(ru.inspiration.modelName)).toBeInTheDocument();
    expect(within(dialog).getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(within(dialog).queryByText("1K")).not.toBeInTheDocument();

    fireEvent.click(thumbnails[imageExampleIndex]);
    expect(within(dialog).queryByText("Изображение")).not.toBeInTheDocument();

    fireEvent.click(thumbnails[videoExampleIndex]);
    expect(within(dialog).queryByText("Видео")).not.toBeInTheDocument();
  });

  it("shows a toggle only when the prompt exceeds five visible lines", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const prompt = screen.getByText(ru.inspiration.prompt);
    Object.defineProperty(prompt, "clientHeight", { configurable: true, value: 100 });
    Object.defineProperty(prompt, "scrollHeight", { configurable: true, value: 160 });

    expect(screen.queryByRole("button", { name: "Показать ещё" })).not.toBeInTheDocument();

    fireEvent(window, new Event("resize"));
    const showMore = screen.getByRole("button", { name: "Показать ещё" });
    expect(showMore).toHaveAttribute("aria-expanded", "false");
    expect(prompt).toContainElement(showMore);

    fireEvent.click(showMore);
    const collapse = screen.getByRole("button", { name: "Свернуть" });
    expect(collapse).toHaveAttribute("aria-expanded", "true");
    expect(prompt).toContainElement(collapse);

    fireEvent.click(collapse);
    expect(screen.getByRole("button", { name: "Показать ещё" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
  });

  it("shows every inspiration item in the thumbnail rail and switches the active preview", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const secondExample = inspirationExamples[1];
    const mediaStage = within(dialog).getByTestId("inspiration-media-stage");

    expect(thumbnails).toHaveLength(inspirationExamples.length);
    expect(mediaStage).toContainElement(
      within(within(dialog).getByTestId("inspiration-preview")).getByRole("img", {
        name: ru.inspiration.exampleAlt,
      }),
    );
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
    expect(thumbnails[1]).not.toHaveAttribute("aria-current");

    fireEvent.click(thumbnails[1]);

    expect(thumbnails[0]).not.toHaveAttribute("aria-current");
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");
    expect(
      within(within(dialog).getByTestId("inspiration-preview")).getByRole("img", {
        name: secondExample.mediaAlt,
      }),
    ).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: ru.inspiration.download })).toHaveAttribute(
      "href",
      secondExample.mediaPath,
    );
  });

  it("wraps the visible preview media in a frame with its original aspect ratio", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const mediaSurface = within(dialog).getByTestId("inspiration-media-surface");
    const example = inspirationExamples[0];

    expect(mediaSurface).toHaveStyle({
      aspectRatio: `${example.mediaWidth} / ${example.mediaHeight}`,
    });
    expect(mediaSurface).toContainElement(
      within(mediaSurface).getByRole("img", { name: example.mediaAlt }),
    );
  });

  it("moves cyclically through examples with the previous and next controls", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const previous = within(dialog).getByRole("button", { name: ru.inspiration.previousExample });
    const next = within(dialog).getByRole("button", { name: ru.inspiration.nextExample });

    fireEvent.click(next);
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");

    fireEvent.click(previous);
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");

    fireEvent.click(previous);
    expect(thumbnails.at(-1)).toHaveAttribute("aria-current", "true");

    fireEvent.click(next);
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
  });

  it("moves through examples with the left and right arrow keys", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");

    fireEvent.keyDown(dialog, { key: "ArrowRight" });
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");

    fireEvent.keyDown(dialog, { key: "ArrowLeft" });
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
  });

  it("uses arrow keys to navigate when the preview video has focus", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const videoIndex = inspirationExamples.findIndex((example) => example.mediaType === "video");
    const nextIndex = (videoIndex + 1) % inspirationExamples.length;

    fireEvent.click(thumbnails[videoIndex]);
    const previewVideo = within(dialog).getByTestId("inspiration-media-surface").querySelector("video")!;

    expect(fireEvent.keyDown(previewVideo, { key: "ArrowRight" })).toBe(false);
    expect(thumbnails[nextIndex]).toHaveAttribute("aria-current", "true");
  });

  it("autoplays the selected preview video muted and resets it when leaving", () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const videoIndex = inspirationExamples.findIndex((example) => example.mediaType === "video");

    play.mockClear();
    pause.mockClear();
    fireEvent.click(thumbnails[videoIndex]);
    const previewVideo = within(dialog).getByTestId("inspiration-media-surface").querySelector("video")!;

    expect(previewVideo.muted).toBe(true);
    expect(play).toHaveBeenCalledTimes(1);

    previewVideo.currentTime = 4;
    fireEvent.click(thumbnails[0]);

    expect(pause).toHaveBeenCalledTimes(1);
    expect(previewVideo.currentTime).toBe(0);
  });

  it("scopes document-level arrow navigation to the open preview dialog", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const backdrop = dialog.closest("[data-state]")!;

    fireEvent.click(within(dialog).getByTestId("inspiration-media-stage"));
    fireEvent.keyDown(document, { key: "ArrowRight" });
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");

    fireEvent.click(within(dialog).getByRole("button", { name: ru.inspiration.close }));
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.keyDown(document, { key: "ArrowRight" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("does not intercept arrow keys from editable controls inside the preview dialog", () => {
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const thumbnails = within(dialog).getAllByTestId("inspiration-thumbnail");
    const input = document.createElement("input");
    dialog.append(input);

    fireEvent.keyDown(input, { key: "ArrowRight" });

    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
  });

  it("closes with Escape, restores page scrolling and returns focus to the card", async () => {
    render(<InspirationGallery />);
    const card = screen.getByRole("button", { name: ru.inspiration.openExample });

    fireEvent.click(card);
    expect(document.body.style.overflow).toBe("hidden");
    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    const backdrop = dialog.closest("[data-state]")!;

    fireEvent.keyDown(window, { key: "Escape" });

    expect(backdrop).toHaveAttribute("data-state", "closing");
    expect(document.body.style.overflow).toBe("hidden");
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });

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
    expect(await screen.findByRole("button", { name: "Скопировано" })).toBeInTheDocument();
    expect(screen.getByTestId("copy-success-icon")).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("clears copied feedback when another example is selected", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.copyPrompt }));
    expect(await screen.findByRole("button", { name: "Скопировано" })).toBeInTheDocument();

    const dialog = screen.getByRole("dialog", { name: ru.inspiration.dialogLabel });
    fireEvent.click(within(dialog).getAllByTestId("inspiration-thumbnail")[1]);

    expect(screen.getByRole("button", { name: ru.inspiration.copyPrompt })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Скопировано" })).not.toBeInTheDocument();
  });

  it("returns the copy button to its default state after two seconds", async () => {
    vi.useFakeTimers();
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<InspirationGallery />);

    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: ru.inspiration.copyPrompt }));
      await Promise.resolve();
    });

    expect(screen.getByRole("button", { name: "Скопировано" })).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(2_000));
    expect(screen.getByRole("button", { name: ru.inspiration.copyPrompt })).toBeInTheDocument();
  });

  it("renders the close icon without a baseline-dependent text glyph", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const closeButton = screen.getByRole("button", { name: ru.inspiration.close });

    expect(closeButton).not.toHaveTextContent("×");
    expect(closeButton.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });

  it("uses the supplied repost icon in the share button", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const shareButton = screen.getByRole("button", { name: ru.inspiration.share });
    const shareIcon = shareButton.querySelector("img");

    expect(shareIcon).toHaveAttribute("src", "/assets/icons/ui/repost-white.svg");
    expect(shareButton).not.toHaveTextContent("↗");
  });

  it("uses the supplied icons in the copy, download and recreate controls", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const copyButton = screen.getByRole("button", { name: ru.inspiration.copyPrompt });
    const downloadLink = screen.getByRole("link", { name: ru.inspiration.download });
    const recreateLink = screen.getByRole("link", { name: ru.inspiration.recreate });

    expect(copyButton.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/copy-white.svg");
    expect(downloadLink.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/download-white.svg");
    expect(recreateLink.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/star-white.svg");
    expect(copyButton).not.toHaveTextContent("▣");
    expect(downloadLink).not.toHaveTextContent("↓");
    expect(recreateLink).not.toHaveTextContent("✦");
  });

  it("offers a local download and prefilled image generator without starting a paid task", () => {
    render(<InspirationGallery />);
    fireEvent.click(screen.getByRole("button", { name: ru.inspiration.openExample }));

    const download = screen.getByRole("link", { name: ru.inspiration.download });
    expect(download).toHaveAttribute("href", "/assets/images/inspiration/paper-crane-cloud.png");
    expect(download).toHaveAttribute("download", "neirohub-paper-crane-cloud.png");

    const recreate = screen.getByRole("link", { name: ru.inspiration.recreate });
    const recreateUrl = new URL(recreate.getAttribute("href")!, "https://neirohub.test");
    expect(recreateUrl.pathname).toBe("/ru/app/image");
    expect(recreateUrl.searchParams.get("model")).toBe("gpt_image_2");
    expect(recreateUrl.searchParams.get("quality")).toBe("1K");
    expect(recreateUrl.searchParams.get("prompt")).toBe(ru.inspiration.prompt);
  });
});
