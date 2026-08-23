import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ImageAspectRatioSelector } from "./ImageAspectRatioSelector";

describe("ImageAspectRatioSelector", () => {
  afterEach(() => {
    cleanup();
  });

  it("opens the aspect-ratio panel and exposes every supported ratio", () => {
    render(
      <ImageAspectRatioSelector disabled={false} onChange={vi.fn()} value="16:9" />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Соотношение сторон: 16:9" }));

    expect(screen.getByRole("dialog", { name: "Соотношение сторон" })).toBeVisible();
    for (const ratio of ["16:9", "1:1", "21:9", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16"]) {
      expect(screen.getByRole("radio", { name: ratio })).toBeVisible();
    }
    expect(screen.getByRole("radio", { name: "16:9" })).toHaveAttribute("aria-checked", "true");
  });

  it("commits a selected ratio and closes the panel", () => {
    const onChange = vi.fn();
    render(
      <ImageAspectRatioSelector disabled={false} onChange={onChange} value="16:9" />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Соотношение сторон: 16:9" }));
    fireEvent.click(screen.getByRole("radio", { name: "4:5" }));

    expect(onChange).toHaveBeenCalledWith("4:5");
    expect(screen.queryByRole("dialog", { name: "Соотношение сторон" })).not.toBeInTheDocument();
  });

  it("closes on Escape and an outside pointer press", () => {
    render(
      <div>
        <ImageAspectRatioSelector disabled={false} onChange={vi.fn()} value="16:9" />
        <button type="button">Снаружи</button>
      </div>,
    );

    const trigger = screen.getByRole("button", { name: "Соотношение сторон: 16:9" });
    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Соотношение сторон" })).not.toBeInTheDocument();

    fireEvent.click(trigger);
    fireEvent.mouseDown(screen.getByRole("button", { name: "Снаружи" }));
    expect(screen.queryByRole("dialog", { name: "Соотношение сторон" })).not.toBeInTheDocument();
  });
});
