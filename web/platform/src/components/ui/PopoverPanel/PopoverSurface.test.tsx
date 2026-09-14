import { createRef } from "react";
import { act, cleanup, createEvent, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PopoverSurface } from "./PopoverPanel";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function finishTransition(element: Element, propertyName = "opacity") {
  const event = createEvent.transitionEnd(element);
  Object.defineProperty(event, "propertyName", { value: propertyName });
  fireEvent(element, event);
}

describe("PopoverSurface", () => {
  it("animates a plain surface by default and forwards its ref and attributes", () => {
    const ref = createRef<HTMLDivElement>();
    render(<PopoverSurface aria-label="Панель" className="custom-panel" ref={ref} role="region">Содержимое</PopoverSurface>);
    const surface = screen.getByRole("region", { name: "Панель" });
    expect(ref.current).toBe(surface);
    expect(surface).toHaveClass("custom-panel");
    expect(surface).toHaveAttribute("data-motion-state", "open");
    expect(surface).toHaveAttribute("data-motion-origin", "top");
    expect(surface).not.toHaveAttribute("inert");
  });

  it("owns the full close lifecycle without an external presence hook", () => {
    const ref = createRef<HTMLDivElement>();
    const onAfterClose = vi.fn();
    const onTransitionEnd = vi.fn();
    const content = (isOpen: boolean) => (
      <PopoverSurface isOpen={isOpen} motionOrigin="bottom" onAfterClose={onAfterClose} onTransitionEnd={onTransitionEnd} ref={ref} role="region">
        <button>Действие</button>
      </PopoverSurface>
    );
    const view = render(content(false));
    expect(screen.queryByRole("region", { hidden: true })).toBeNull();
    expect(onAfterClose).not.toHaveBeenCalled();
    view.rerender(content(true));
    const surface = screen.getByRole("region");
    expect(surface).toHaveAttribute("data-motion-origin", "bottom");
    view.rerender(content(false));
    expect(surface).toBeInTheDocument();
    expect(surface).toHaveAttribute("inert");
    expect(surface).toHaveAttribute("aria-hidden", "true");
    expect(surface).toHaveAttribute("data-motion-state", "closing");
    finishTransition(surface.querySelector("button")!);
    finishTransition(surface, "color");
    expect(surface).toBeInTheDocument();
    finishTransition(surface);
    expect(surface).not.toBeInTheDocument();
    expect(ref.current).toBeNull();
    expect(onAfterClose).toHaveBeenCalledOnce();
    expect(onTransitionEnd).toHaveBeenCalledTimes(3);
    view.rerender(content(false));
    expect(onAfterClose).toHaveBeenCalledOnce();
  });

  it("opens and closes immediately when animated is false", () => {
    const onAfterClose = vi.fn();
    const view = render(<PopoverSurface animated={false} isOpen onAfterClose={onAfterClose} role="region" />);
    const surface = screen.getByRole("region");
    expect(surface).not.toHaveAttribute("data-motion-state");
    view.rerender(<PopoverSurface animated={false} isOpen={false} onAfterClose={onAfterClose} role="region" />);
    expect(surface).not.toBeInTheDocument();
    expect(onAfterClose).toHaveBeenCalledOnce();
  });

  it("can disable motion during closing without leaving or resurrecting a hidden surface", () => {
    const view = render(<PopoverSurface isOpen role="region" />);
    const surface = screen.getByRole("region");
    view.rerender(<PopoverSurface isOpen={false} role="region" />);
    expect(surface).toBeInTheDocument();
    view.rerender(<PopoverSurface animated={false} isOpen={false} role="region" />);
    expect(surface).not.toBeInTheDocument();
    view.rerender(<PopoverSurface isOpen={false} role="region" />);
    expect(screen.queryByRole("region", { hidden: true })).toBeNull();
  });

  it("reverses a quick close and cancels the old completion callback", () => {
    vi.useFakeTimers();
    const onAfterClose = vi.fn();
    const view = render(<PopoverSurface isOpen onAfterClose={onAfterClose} role="region" />);
    const surface = screen.getByRole("region");
    view.rerender(<PopoverSurface isOpen={false} onAfterClose={onAfterClose} role="region" />);
    view.rerender(<PopoverSurface isOpen onAfterClose={onAfterClose} role="region" />);
    expect(screen.getByRole("region")).toBe(surface);
    expect(surface).not.toHaveAttribute("inert");
    finishTransition(surface);
    act(() => vi.advanceTimersByTime(1000));
    expect(surface).toBeInTheDocument();
    expect(onAfterClose).not.toHaveBeenCalled();
  });

  it("respects reduced motion and still completes closing exactly once", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
    const onAfterClose = vi.fn();
    const view = render(<PopoverSurface isOpen onAfterClose={onAfterClose} role="region" />);
    const surface = screen.getByRole("region");
    view.rerender(<PopoverSurface isOpen={false} onAfterClose={onAfterClose} role="region" />);
    expect(surface).not.toBeInTheDocument();
    expect(onAfterClose).toHaveBeenCalledOnce();
  });
});
