import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { VideoPlayer } from "./VideoPlayer";

describe("VideoPlayer", () => {
  afterEach(() => {
    cleanup();
  });

  it("does not create a video request while no source is configured", () => {
    const { container } = render(<VideoPlayer title="Как работает NeiroHub" />);

    expect(screen.getByText("Видео скоро появится")).toBeVisible();
    expect(container.querySelector("video")).toBeNull();
    expect(container.querySelector("source")).toBeNull();
  });

  it("prepares an MP4 source for on-demand playback", () => {
    const { container } = render(
      <VideoPlayer
        poster="/assets/images/video/how-it-works-poster.webp"
        source={{ src: "https://cdn.neirohub.ru/video/how-it-works.mp4", type: "video/mp4" }}
        title="Как работает NeiroHub"
      />,
    );

    const video = screen.getByLabelText("Как работает NeiroHub");
    expect(video).toHaveAttribute("controls");
    expect(video).toHaveAttribute("poster", "/assets/images/video/how-it-works-poster.webp");
    expect(video).toHaveAttribute("preload", "none");
    expect(container.querySelector("source")).toHaveAttribute(
      "src",
      "https://cdn.neirohub.ru/video/how-it-works.mp4",
    );
    expect(container.querySelector("source")).toHaveAttribute("type", "video/mp4");
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
