import { cleanup, createEvent, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
    const posterOverlay = screen.getByTestId("video-poster-overlay");

    expect(playButton).toBeVisible();
    expect(playButton).toBe(posterOverlay);
    expect(posterOverlay).toHaveStyle({
      "--video-player-poster": 'url("/assets/images/video/how-it-works-poster.webp")',
    });
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

  it("shows the centered play control over the current frame while paused", async () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    render(
      <VideoPlayer
        poster="/assets/images/video/how-it-works-poster.webp"
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" }));
    const video = screen.getByLabelText("Как работает NeiroHub");
    fireEvent.play(video);
    fireEvent.pause(video);

    const resumeButton = screen.getByRole("button", { name: "Продолжить: Как работает NeiroHub" });
    expect(resumeButton).toBeVisible();
    expect(screen.getByTestId("video-pause-overlay")).toBeVisible();
    expect(screen.queryByTestId("video-poster-overlay")).not.toBeInTheDocument();

    fireEvent.click(resumeButton);
    await waitFor(() => expect(play).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole("button", { name: "Продолжить: Как работает NeiroHub" })).toBeNull();
  });

  it("pauses with Space only while the video is playing", async () => {
    vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    render(
      <VideoPlayer
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.keyDown(window, { code: "Space", key: " " });
    expect(pause).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" }));
    const video = screen.getByLabelText("Как работает NeiroHub");
    Object.defineProperty(video, "paused", { configurable: true, value: false, writable: true });
    fireEvent.play(video);
    fireEvent.keyDown(window, { code: "Space", key: " " });
    expect(pause).toHaveBeenCalledOnce();

    fireEvent.pause(video);
    Object.defineProperty(video, "paused", { configurable: true, value: true, writable: true });
    const repeatedKeyDown = createEvent.keyDown(window, { code: "Space", key: " ", repeat: true });
    fireEvent(window, repeatedKeyDown);
    expect(repeatedKeyDown.defaultPrevented).toBe(true);

    const keyUp = createEvent.keyUp(window, { code: "Space", key: " " });
    fireEvent(window, keyUp);
    expect(keyUp.defaultPrevented).toBe(true);
    expect(pause).toHaveBeenCalledOnce();

    const pausedKeyDown = createEvent.keyDown(window, { code: "Space", key: " " });
    fireEvent(window, pausedKeyDown);
    expect(pausedKeyDown.defaultPrevented).toBe(false);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Продолжить: Как работает NeiroHub" })).toBeVisible();
    });
  });

  it("uses the video's real playback state after seeking with native controls", () => {
    vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    render(
      <VideoPlayer
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Воспроизвести: Как работает NeiroHub" }));
    const video = screen.getByLabelText("Как работает NeiroHub");
    fireEvent.play(video);
    fireEvent.pause(video);
    Object.defineProperty(video, "paused", { configurable: true, value: false });

    fireEvent.keyDown(video, { code: "Space", key: " " });
    expect(pause).toHaveBeenCalledOnce();
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
