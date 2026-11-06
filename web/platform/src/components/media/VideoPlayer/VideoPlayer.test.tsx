import { act, cleanup, createEvent, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { VideoPlayer } from "./VideoPlayer";

describe("VideoPlayer", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  function observeViewport() {
    let notify: IntersectionObserverCallback;
    const disconnect = vi.fn();
    const observe = vi.fn();
    let options: IntersectionObserverInit | undefined;
    const Observer = vi.fn(function (callback: IntersectionObserverCallback, init?: IntersectionObserverInit) {
      notify = callback;
      options = init;
      return { observe, disconnect };
    });
    vi.stubGlobal("IntersectionObserver", Observer);
    return {
      observe,
      disconnect,
      Observer,
      get options() { return options; },
      intersect: (isIntersecting: boolean) => act(() => {
        notify([{ isIntersecting } as IntersectionObserverEntry], {} as IntersectionObserver);
      }),
    };
  }

  const deferredPlayer = <VideoPlayer deferPoster poster="/poster.webp" source={{ src: "/video.mp4", type: "video/mp4" }} title="Demo" />;

  it("omits deferred poster URLs from server HTML and a distant player's fetch attributes", () => {
    const viewport = observeViewport();
    expect(renderToString(deferredPlayer)).not.toContain("/poster.webp");
    const { container } = render(deferredPlayer);
    viewport.intersect(false);
    expect(container.querySelector("video")).not.toHaveAttribute("poster");
    expect(screen.getByTestId("video-poster-overlay")).toHaveStyle({ "--video-player-poster": "none" });
    expect(container.querySelector("video")).toHaveAttribute("preload", "none");
  });

  it("loads the poster before entering the viewport and retains it after leaving", () => {
    const viewport = observeViewport();
    render(deferredPlayer);
    expect(viewport.observe).toHaveBeenCalledOnce();
    expect(Number.parseInt(viewport.options?.rootMargin ?? "0", 10)).toBeGreaterThan(0);
    viewport.intersect(true);
    expect(screen.getByLabelText("Demo")).toHaveAttribute("poster", "/poster.webp");
    expect(screen.getByTestId("video-poster-overlay")).toHaveStyle({ "--video-player-poster": 'url("/poster.webp")' });
    expect(viewport.disconnect).toHaveBeenCalled();
    viewport.intersect(false);
    expect(screen.getByLabelText("Demo")).toHaveAttribute("poster", "/poster.webp");
  });

  it("loads a deferred poster on keyboard focus before any viewport notification", () => {
    observeViewport();
    render(deferredPlayer);
    expect(screen.getByLabelText("Demo")).not.toHaveAttribute("poster");
    fireEvent.focus(screen.getByRole("button"));
    expect(screen.getByLabelText("Demo")).toHaveAttribute("poster", "/poster.webp");
    expect(screen.getByLabelText("Demo")).toHaveAttribute("preload", "none");
  });

  it("plays a deferred player on demand with audio and preserves pause/end/replay controls", () => {
    observeViewport();
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    render(deferredPlayer);
    const video = screen.getByLabelText("Demo") as HTMLVideoElement;
    expect(video).not.toHaveAttribute("poster");
    fireEvent.click(screen.getByRole("button"));
    expect(video).toHaveAttribute("poster", "/poster.webp");
    expect(play).toHaveBeenCalledOnce();
    expect(video.muted).toBe(false);
    expect(video).not.toHaveAttribute("autoplay");
    expect(video).toHaveAttribute("controls");
    fireEvent.pause(video);
    fireEvent.click(screen.getByRole("button"));
    expect(play).toHaveBeenCalledTimes(2);
    fireEvent.ended(video);
    fireEvent.click(screen.getByRole("button"));
    expect(play).toHaveBeenCalledTimes(3);
  });

  it("loads the poster without IntersectionObserver and disconnects on unmount", async () => {
    vi.stubGlobal("IntersectionObserver", undefined);
    const { unmount } = render(deferredPlayer);
    await waitFor(() => expect(screen.getByLabelText("Demo")).toHaveAttribute("poster", "/poster.webp"));
    unmount();
    const viewport = observeViewport();
    const deferred = render(deferredPlayer);
    deferred.unmount();
    expect(viewport.disconnect).toHaveBeenCalledOnce();
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
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
      "https://cdn.epifera.com/video/how-it-works.mp4",
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
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
        source={{ src: "https://cdn.epifera.com/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    fireEvent.error(screen.getByLabelText("Как работает NeiroHub"));

    expect(screen.getByText("Видео временно недоступно")).toBeVisible();
    expect(screen.queryByLabelText("Как работает NeiroHub")).not.toBeInTheDocument();
  });
});
