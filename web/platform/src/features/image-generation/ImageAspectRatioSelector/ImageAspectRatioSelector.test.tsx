import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ImageAspectRatioSelector } from "./ImageAspectRatioSelector";

describe("ImageAspectRatioSelector", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
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

  it("keeps the panel inside a narrow viewport instead of clipping it by the workspace", () => {
    vi.stubGlobal("innerWidth", 582);
    vi.stubGlobal("innerHeight", 452);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function getRect(this: HTMLElement) {
      if (this.getAttribute("aria-label")?.startsWith("Соотношение сторон:")) {
        return {
          bottom: 440,
          height: 52,
          left: 430,
          right: 560,
          top: 388,
          width: 130,
          x: 430,
          y: 388,
          toJSON: () => undefined,
        };
      }
      return {
        bottom: 0,
        height: 0,
        left: 0,
        right: 0,
        top: 0,
        width: 0,
        x: 0,
        y: 0,
        toJSON: () => undefined,
      };
    });
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(360);

    render(<ImageAspectRatioSelector disabled={false} onChange={vi.fn()} value="16:9" />);
    fireEvent.click(screen.getByRole("button", { name: "Соотношение сторон: 16:9" }));

    const panel = screen.getByRole("dialog", { name: "Соотношение сторон" });
    expect(panel.parentElement).toBe(document.body);
    expect(panel).toHaveStyle({
      left: "16px",
      maxHeight: "420px",
      top: "16px",
      width: "550px",
    });
  });
});
