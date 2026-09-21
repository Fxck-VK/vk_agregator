import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MediaVideo } from "./MediaVideo";

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe("poster-first video", () => {
  it("does not request offscreen video and keeps its poster through metadata and buffering", () => {
    let intersect!: IntersectionObserverCallback;
    const disconnect = vi.fn();
    vi.stubGlobal("IntersectionObserver", class {
      constructor(callback: IntersectionObserverCallback) { intersect = callback; }
      observe() {}
      disconnect = disconnect;
    });
    const ref = createRef<HTMLVideoElement>();
    const { container } = render(<MediaVideo src="/clip.mp4" poster="/cover.webp" loading="lazy" preload="auto" videoRef={ref} passive />);
    const video = container.querySelector("video")!;
    const poster = container.querySelector('[data-ui="video-poster"]')!;
    fireEvent.load(poster);
    expect(video).not.toHaveAttribute("src");
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    act(() => intersect([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver));
    expect(video).toHaveAttribute("src", "/clip.mp4");
    expect(ref.current).toBe(video);
    fireEvent.loadedMetadata(video);
    fireEvent.waiting(video);
    expect(poster).toBeVisible();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    fireEvent.loadedData(video);
    expect(poster).not.toBeVisible();
    expect(disconnect).toHaveBeenCalled();
  });

  it("retains the cover on failure and retry until a decoded frame is available", () => {
    const load = vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
    const { container } = render(<MediaVideo src="/clip.mp4" poster="/cover.webp" controls />);
    const video = container.querySelector("video")!;
    const poster = container.querySelector('[data-ui="video-poster"]')!;
    fireEvent.load(poster);
    fireEvent.loadedData(video);
    fireEvent.error(video);
    expect(poster).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(load).toHaveBeenCalledOnce();
    expect(poster).toBeVisible();
    fireEvent.canPlay(video);
    expect(poster).not.toBeVisible();
  });

  it("falls back to the shared loading state if the poster itself fails", () => {
    const { container } = render(<MediaVideo src="/clip.mp4" poster="/missing.webp" preload="auto" />);
    fireEvent.error(container.querySelector('[data-ui="video-poster"]')!);
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(container.querySelector("video")).toHaveAttribute("preload", "auto");
    fireEvent.loadedData(container.querySelector("video")!);
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });
});
