import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LoadingIndicator, StateNotice } from "./AsyncState";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { createRef } from "react";

afterEach(cleanup);

describe("shared async states", () => {
  it("exposes known progress and keeps indeterminate progress without a fabricated percentage", () => {
    const { rerender } = render(<LoadingIndicator label="Upload" progress={42} />);
    expect(screen.getByRole("progressbar", { name: "Upload" })).toHaveAttribute("aria-valuenow", "42");
    rerender(<LoadingIndicator label="Upload" />);
    expect(screen.getByRole("progressbar")).not.toHaveAttribute("aria-valuenow");
  });
  it("preserves the error and calls the supplied retry exactly once", () => {
    const retry = vi.fn();
    render(<StateNotice kind="error" action={{ label: "Retry", onClick: retry }}>Unavailable</StateNotice>);
    expect(screen.getByRole("alert")).toHaveTextContent("Unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });
  it("handles image failure and retry without triggering the image action", () => {
    const open = vi.fn();
    render(<MediaImage src="/image.png" alt="Photo" loadingLabel="Loading" errorLabel="Unavailable" retryLabel="Retry" action={{ label: "Open photo", onClick: open }} />);
    expect(screen.getByRole("progressbar", { name: "Loading" })).toBeInTheDocument();
    fireEvent.error(screen.getByRole("img", { name: "Photo" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Unavailable");
    const oldImage = screen.getByRole("img", { name: "Photo" });
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(open).not.toHaveBeenCalled();
    const image = screen.getByRole("img", { name: "Photo" });
    expect(image).not.toBe(oldImage);
    expect(image).toHaveAttribute("src", "/image.png");
    fireEvent.load(image);
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open photo" }));
    expect(open).toHaveBeenCalledTimes(1);
    expect(document.querySelector("button button")).toBeNull();
  });
  it("resets an old error when the source changes", () => {
    const { rerender } = render(<MediaImage src="/first.png" alt="Photo" />);
    fireEvent.error(screen.getByRole("img", { name: "Photo" }));
    rerender(<MediaImage src="/second.png" alt="Photo" />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("loads and retries a video without replacing the caller's media ref", () => {
    const ref = createRef<HTMLVideoElement>();
    const load = vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
    const { container } = render(<MediaVideo src="/clip.mp4" videoRef={ref} controls />);
    const video = container.querySelector("video")!;
    expect(ref.current).toBe(video);
    fireEvent.loadedMetadata(video);
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    fireEvent.loadedData(video);
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    fireEvent.waiting(video);
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    fireEvent.error(video);
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(load).toHaveBeenCalledTimes(1);
    expect(ref.current).toBe(video);
    fireEvent.canPlay(video);
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    load.mockRestore();
  });
});
