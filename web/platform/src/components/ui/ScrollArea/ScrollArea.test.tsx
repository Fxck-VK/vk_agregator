import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ScrollArea } from "./ScrollArea";

let animationFrames: FrameRequestCallback[] = [];

function flushAnimationFrames() {
  while (animationFrames.length > 0) {
    const currentFrames = animationFrames;
    animationFrames = [];
    currentFrames.forEach((callback) => callback(performance.now()));
  }
}

function setScrollMetrics(
  viewport: HTMLElement,
  track: HTMLElement,
  { clientHeight, scrollHeight, scrollTop = 0 }: { clientHeight: number; scrollHeight: number; scrollTop?: number },
) {
  Object.defineProperties(viewport, {
    clientHeight: { configurable: true, value: clientHeight },
    scrollHeight: { configurable: true, value: scrollHeight },
    scrollTop: { configurable: true, value: scrollTop, writable: true },
  });
  Object.defineProperty(track, "clientHeight", { configurable: true, value: clientHeight });
}

function setHorizontalScrollMetrics(
  viewport: HTMLElement,
  track: HTMLElement,
  { clientWidth, scrollLeft = 0, scrollWidth }: { clientWidth: number; scrollLeft?: number; scrollWidth: number },
) {
  Object.defineProperties(viewport, {
    clientWidth: { configurable: true, value: clientWidth },
    scrollLeft: { configurable: true, value: scrollLeft, writable: true },
    scrollWidth: { configurable: true, value: scrollWidth },
  });
  Object.defineProperty(track, "clientWidth", { configurable: true, value: clientWidth });
}

describe("ScrollArea", () => {
  beforeEach(() => {
    animationFrames = [];
    vi.useFakeTimers();
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback: FrameRequestCallback) => {
      animationFrames.push(callback);
      return animationFrames.length;
    });
    vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders a semantic native viewport inside a decorative overlay", () => {
    render(
      <ScrollArea
        data-testid="scroll-root"
        viewportAs="main"
        viewportProps={{ "data-testid": "scroll-viewport", tabIndex: -1 }}
      >
        <p>Содержимое</p>
      </ScrollArea>,
    );

    expect(screen.getByTestId("scroll-viewport").tagName).toBe("MAIN");
    expect(screen.getByTestId("floating-scrollbar-track")).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByTestId("scroll-root")).toContainElement(screen.getByText("Содержимое"));
  });

  it("uses a textarea as the native viewport and refreshes its thumb after input", () => {
    const onInput = vi.fn();
    render(
      <ScrollArea
        data-testid="scroll-root"
        viewportAs="textarea"
        viewportProps={{
          "aria-label": "Промпт",
          "data-testid": "scroll-viewport",
          onInput,
        }}
      >
        {null}
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");

    flushAnimationFrames();
    expect(root).toHaveAttribute("data-scrollable", "false");

    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 250 });
    fireEvent.input(viewport, { target: { value: "Длинный промпт" } });
    flushAnimationFrames();

    expect(onInput).toHaveBeenCalledOnce();
    expect(viewport.tagName).toBe("TEXTAREA");
    expect(viewport).toHaveAttribute("data-scroll-area-viewport", "true");
    expect(root).toHaveAttribute("data-scrollable", "true");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-size")).toBe("40px");
  });

  it("marks an outside track so horizontal content can keep its lower alignment", () => {
    render(
      <ScrollArea
        data-testid="scroll-root"
        orientation="horizontal"
        trackPlacement="outside"
      >
        <div>Широкое содержимое</div>
      </ScrollArea>,
    );

    expect(screen.getByTestId("scroll-root")).toHaveAttribute("data-track-placement", "outside");
  });

  it("calculates thumb geometry and marks the area active while it scrolls", () => {
    render(
      <ScrollArea data-testid="scroll-root" viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Длинное содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");
    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 200, scrollTop: 50 });

    flushAnimationFrames();
    fireEvent.scroll(viewport);
    flushAnimationFrames();

    expect(root).toHaveAttribute("data-scrollable", "true");
    expect(root).toHaveAttribute("data-scrolling", "true");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-size")).toBe("50px");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-offset")).toBe("25px");

    vi.advanceTimersByTime(700);
    expect(root).not.toHaveAttribute("data-scrolling");
  });

  it("updates the thumb immediately when animation frames are delayed", () => {
    render(
      <ScrollArea data-testid="scroll-root" viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Длинное содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");
    setScrollMetrics(viewport, track, { clientHeight: 120, scrollHeight: 480, scrollTop: 120 });

    fireEvent.scroll(viewport);

    expect(root).toHaveAttribute("data-scrollable", "true");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-size")).toBe("32px");
    expect(parseFloat(thumb.style.getPropertyValue("--scroll-area-thumb-offset"))).toBeCloseTo(29.333, 3);
  });

  it("keeps the overlay inactive when the content does not overflow", () => {
    render(
      <ScrollArea data-testid="scroll-root" viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Короткое содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 100 });

    flushAnimationFrames();

    expect(root).toHaveAttribute("data-scrollable", "false");
  });

  it("moves the native viewport when its floating track is pressed", () => {
    render(
      <ScrollArea viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Длинное содержимое</div>
      </ScrollArea>,
    );
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 300 });
    vi.spyOn(track, "getBoundingClientRect").mockReturnValue({
      bottom: 100,
      height: 100,
      left: 0,
      right: 6,
      top: 0,
      width: 6,
      x: 0,
      y: 0,
      toJSON: () => undefined,
    });
    flushAnimationFrames();

    fireEvent.pointerDown(track, { clientY: 75, pointerId: 1 });
    flushAnimationFrames();

    expect(viewport.scrollTop).toBeCloseTo(175, 0);
  });

  it("drags the floating thumb without replacing native scrolling", () => {
    render(
      <ScrollArea data-testid="scroll-root" viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Длинное содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");
    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 300 });
    Object.defineProperty(thumb, "setPointerCapture", { configurable: true, value: vi.fn() });
    Object.defineProperty(thumb, "releasePointerCapture", { configurable: true, value: vi.fn() });
    flushAnimationFrames();

    fireEvent.pointerDown(thumb, { clientY: 10, pointerId: 4 });
    fireEvent.pointerMove(thumb, { clientY: 43, pointerId: 4 });
    flushAnimationFrames();

    expect(root).toHaveAttribute("data-dragging", "true");
    expect(viewport.scrollTop).toBeCloseTo(99, 0);

    fireEvent.pointerUp(thumb, { pointerId: 4 });
    expect(root).not.toHaveAttribute("data-dragging");
  });

  it("calculates horizontal thumb geometry from the native inline scroll position", () => {
    render(
      <ScrollArea
        data-testid="scroll-root"
        orientation="horizontal"
        viewportProps={{ "data-testid": "scroll-viewport" }}
      >
        <div>Широкое содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");
    setHorizontalScrollMetrics(viewport, track, { clientWidth: 200, scrollWidth: 800, scrollLeft: 200 });

    fireEvent.scroll(viewport);

    expect(root).toHaveAttribute("data-orientation", "horizontal");
    expect(root).toHaveAttribute("data-scrollable", "true");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-size")).toBe("50px");
    expect(thumb.style.getPropertyValue("--scroll-area-thumb-offset")).toBe("50px");
  });

  it("drags a horizontal thumb along the inline axis", () => {
    render(
      <ScrollArea
        data-testid="scroll-root"
        orientation="horizontal"
        viewportProps={{ "data-testid": "scroll-viewport" }}
      >
        <div>Широкое содержимое</div>
      </ScrollArea>,
    );
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    const thumb = screen.getByTestId("floating-scrollbar-thumb");
    setHorizontalScrollMetrics(viewport, track, { clientWidth: 200, scrollWidth: 800 });
    Object.defineProperty(thumb, "setPointerCapture", { configurable: true, value: vi.fn() });
    Object.defineProperty(thumb, "releasePointerCapture", { configurable: true, value: vi.fn() });

    fireEvent.pointerDown(thumb, { clientX: 10, pointerId: 7 });
    fireEvent.pointerMove(thumb, { clientX: 60, pointerId: 7 });

    expect(viewport.scrollLeft).toBeCloseTo(200, 0);
  });

  it("maps a vertical mouse wheel to horizontal overflow without trapping the page at an edge", () => {
    render(
      <ScrollArea orientation="horizontal" viewportProps={{ "data-testid": "scroll-viewport" }}>
        <div>Широкое содержимое</div>
      </ScrollArea>,
    );
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    setHorizontalScrollMetrics(viewport, track, { clientWidth: 200, scrollWidth: 800, scrollLeft: 100 });

    const wheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
    viewport.dispatchEvent(wheel);

    expect(viewport.scrollLeft).toBe(180);
    expect(wheel.defaultPrevented).toBe(true);

    viewport.scrollLeft = 600;
    const edgeWheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
    viewport.dispatchEvent(edgeWheel);

    expect(viewport.scrollLeft).toBe(600);
    expect(edgeWheel.defaultPrevented).toBe(false);
  });

  it.each(["floating-scrollbar-track", "floating-scrollbar-thumb"])(
    "scrolls over %s without moving focus from another panel",
    (targetTestId) => {
      render(
        <>
          <button type="button">Предыдущий файл</button>
          <ScrollArea viewportProps={{ "data-testid": "scroll-viewport" }}>
            <p>Параметры редактирования</p>
          </ScrollArea>
        </>,
      );
      const viewport = screen.getByTestId("scroll-viewport");
      const track = screen.getByTestId("floating-scrollbar-track");
      const previousButton = screen.getByRole("button", { name: "Предыдущий файл" });
      setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 400, scrollTop: 100 });
      previousButton.focus();

      const wheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
      screen.getByTestId(targetTestId).dispatchEvent(wheel);

      expect(viewport.scrollTop).toBe(180);
      expect(wheel.defaultPrevented).toBe(true);
      expect(previousButton).toHaveFocus();
    },
  );

  it("keeps native vertical scrolling and browser zoom gestures intact", () => {
    render(<ScrollArea viewportProps={{ "data-testid": "scroll-viewport" }} />);
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    setScrollMetrics(viewport, track, { clientHeight: 100, scrollHeight: 400, scrollTop: 100 });

    const wheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
    viewport.dispatchEvent(wheel);
    const zoomWheel = new WheelEvent("wheel", {
      bubbles: true, cancelable: true, ctrlKey: true, deltaY: 80,
    });
    track.dispatchEvent(zoomWheel);

    expect(wheel.defaultPrevented).toBe(false);
    expect(zoomWheel.defaultPrevented).toBe(false);
    expect(viewport.scrollTop).toBe(100);
  });

  it("keeps a horizontal overlay inactive when inline content does not overflow", () => {
    render(
      <ScrollArea
        data-testid="scroll-root"
        orientation="horizontal"
        viewportProps={{ "data-testid": "scroll-viewport" }}
      >
        <div>Короткое содержимое</div>
      </ScrollArea>,
    );
    const root = screen.getByTestId("scroll-root");
    const viewport = screen.getByTestId("scroll-viewport");
    const track = screen.getByTestId("floating-scrollbar-track");
    setHorizontalScrollMetrics(viewport, track, { clientWidth: 200, scrollWidth: 200 });

    flushAnimationFrames();

    expect(root).toHaveAttribute("data-scrollable", "false");
  });
});
