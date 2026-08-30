import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { VideoPlayer } from "./VideoPlayer";

describe("VideoPlayer", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("does not create a video request while no source is configured", () => {
    const { container } = render(<VideoPlayer title="Как работает NeiroHub" />);

    expect(screen.getByText("Видео скоро появится")).toBeVisible();
    expect(container.querySelector("video")).toBeNull();
    expect(container.querySelector("source")).toBeNull();
  });

  it("shows a branded start overlay and reveals controls after playback starts", async () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const { container } = render(
      <VideoPlayer
        poster="/assets/images/video/how-it-works-poster.webp"
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    const video = screen.getByLabelText("Как работает NeiroHub");
    const playButton = screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" });

    expect(playButton).toBeVisible();
    expect(screen.queryByText("Видео скоро появится")).not.toBeInTheDocument();
    expect(video).not.toHaveAttribute("controls");
    expect(video).toHaveAttribute("poster", "/assets/images/video/how-it-works-poster.webp");
    expect(video).toHaveAttribute("preload", "none");
    expect(container.querySelector("source")).toHaveAttribute(
      "src",
      "https://cdn.neirohub.ru/video/how-it-works.mp4",
    );
    expect(container.querySelector("source")).toHaveAttribute("type", "video/mp4");

    fireEvent.click(playButton);

    await waitFor(() => expect(play).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button", { name: "Воспроизвести: Как работает NeiroHub" })).toBeNull();
    expect(video).toHaveAttribute("controls");
  });

  it("restores the start overlay when playback cannot begin", async () => {
    vi.spyOn(HTMLMediaElement.prototype, "play").mockRejectedValue(new Error("blocked"));
    render(
      <VideoPlayer
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" })).toBeVisible();
    });
    expect(screen.getByLabelText("Как работает NeiroHub")).not.toHaveAttribute("controls");
  });

  it("shows a neutral error state when playback fails", () => {
    render(
      <VideoPlayer
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.error(screen.getByLabelText("Как работает NeiroHub"));

    expect(screen.getByText("Видео временно недоступно")).toBeVisible();
    expect(screen.queryByLabelText("Как работает NeiroHub")).not.toBeInTheDocument();
  });
});
