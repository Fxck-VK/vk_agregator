import { createRef, useRef, useState } from "react";
import { act, cleanup, createEvent, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PopoverPanel } from "./PopoverPanel";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function InteractivePanel({ animated, onClose }: { animated?: boolean; onClose: () => void }) {
  const [isOpen, setIsOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const close = () => { onClose(); setIsOpen(false); };
  return (
    <>
      <button aria-expanded={isOpen} onClick={() => setIsOpen((open) => !open)} ref={anchorRef}>Настройки</button>
      <button>Снаружи</button>
      <PopoverPanel animated={animated} anchorRef={anchorRef} isOpen={isOpen} label="Настройки" onClose={close} width={352}>
        <button onClick={close}>Выбрать</button>
      </PopoverPanel>
    </>
  );
}

function finishTransition(element: Element) {
  const event = createEvent.transitionEnd(element);
  Object.defineProperty(event, "propertyName", { value: "opacity" });
  fireEvent(element, event);
}

function renderPanel({ isOpen = true, align = "start", width = 352 }:
  { isOpen?: boolean; align?: "start" | "end"; width?: number } = {}) {
  const anchorRef = createRef<HTMLButtonElement>();
  const onClose = vi.fn();
  const view = render(
    <div>
      <button ref={anchorRef} type="button"><span>Открыть настройки</span></button>
      <button type="button">Снаружи</button>
      <PopoverPanel align={align} anchorRef={anchorRef} isOpen={isOpen} label="Настройки" onClose={onClose} width={width}>
        <button type="button">Вариант</button>
      </PopoverPanel>
    </div>,
  );
  return { ...view, onClose, trigger: screen.getByRole("button", { name: "Открыть настройки" }) };
}

describe("PopoverPanel", () => {
  it("allows its consumer to opt out of animation", () => {
    render(<InteractivePanel animated={false} onClose={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Настройки" });
    fireEvent.click(trigger);
    const panel = screen.getByRole("dialog");
    expect(panel).not.toHaveAttribute("data-motion-state");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(panel).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    fireEvent.click(trigger);
    expect(screen.getByRole("dialog")).toBeVisible();
  });

  it.each(["escape", "trigger", "outside", "selection"])("keeps an inert panel until its %s close finishes", (method) => {
    const onClose = vi.fn();
    render(<InteractivePanel onClose={onClose} />);
    const trigger = screen.getByRole("button", { name: "Настройки" });
    fireEvent.click(trigger);
    const panel = screen.getByRole("dialog");
    expect(panel).toHaveAttribute("data-motion-state", "open");

    if (method === "escape") fireEvent.keyDown(document, { key: "Escape" });
    else if (method === "trigger") fireEvent.click(trigger);
    else if (method === "selection") fireEvent.click(screen.getByRole("button", { name: "Выбрать" }));
    else {
      const outside = screen.getByRole("button", { name: "Снаружи" });
      outside.focus();
      fireEvent.mouseDown(outside);
      expect(outside).toHaveFocus();
    }

    expect(panel).toBeInTheDocument();
    expect(panel).toHaveAttribute("data-motion-state", "closing");
    expect(panel).toHaveAttribute("inert");
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    if (method === "escape") expect(trigger).toHaveFocus();
    const closeCount = onClose.mock.calls.length;
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.mouseDown(document.body);
    expect(onClose).toHaveBeenCalledTimes(closeCount);
    finishTransition(panel.querySelector("button")!);
    expect(panel).toBeInTheDocument();
    finishTransition(panel);
    expect(panel).not.toBeInTheDocument();
  });

  it("reverses a quick close without losing the panel or focus to an old timer", () => {
    vi.useFakeTimers();
    render(<InteractivePanel onClose={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Настройки" });
    fireEvent.click(trigger);
    const panel = screen.getByRole("dialog");
    fireEvent.click(trigger);
    fireEvent.click(trigger);
    expect(screen.getByRole("dialog")).toBe(panel);
    expect(panel).not.toHaveAttribute("inert");
    expect(screen.getByRole("button", { name: "Выбрать" })).toHaveFocus();
    finishTransition(panel);
    act(() => vi.advanceTimersByTime(1000));
    expect(panel).toBeInTheDocument();
  });

  it("does not retain a closed panel when reduced motion is enabled", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
    render(<InteractivePanel onClose={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Настройки" });
    fireEvent.click(trigger);
    const panel = screen.getByRole("dialog");
    fireEvent.click(trigger);
    expect(panel).not.toBeInTheDocument();
  });

  it("mounts the named dialog in the document body only while open", () => {
    const closed = renderPanel({ isOpen: false });
    expect(screen.queryByRole("dialog")).toBeNull();
    closed.unmount();

    renderPanel();
    expect(screen.getByRole("dialog", { name: "Настройки" }).parentElement).toBe(document.body);
    expect(screen.getByRole("button", { name: "Вариант" })).toBeVisible();
  });

  it("ignores presses within the panel and trigger but closes outside without moving focus", () => {
    const { onClose } = renderPanel();
    fireEvent.mouseDown(screen.getByText("Открыть настройки"));
    fireEvent.mouseDown(screen.getByRole("button", { name: "Вариант" }));
    expect(onClose).not.toHaveBeenCalled();

    const outside = screen.getByRole("button", { name: "Снаружи" });
    outside.focus();
    fireEvent.mouseDown(outside);
    expect(onClose).toHaveBeenCalledOnce();
    expect(outside).toHaveFocus();
  });

  it("closes on Escape and restores focus to the trigger", () => {
    const { onClose, trigger } = renderPanel();
    screen.getByRole("button", { name: "Вариант" }).focus();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
    expect(trigger).toHaveFocus();
  });

  it("removes closing handlers on unmount", () => {
    const { onClose, unmount } = renderPanel();
    unmount();
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.mouseDown(document.body);
    expect(onClose).not.toHaveBeenCalled();
  });

  it.each([false, true])("returns focus to the trigger when Tab leaves the panel (shift: %s)", (shiftKey) => {
    const { onClose, trigger } = renderPanel();
    const option = screen.getByRole("button", { name: "Вариант" });
    expect(option).toHaveFocus();
    fireEvent.keyDown(option, { key: "Tab", shiftKey });
    expect(onClose).toHaveBeenCalledOnce();
    expect(trigger).toHaveFocus();
  });

  it.each([
    { name: "below the start edge when both sides have room", align: "start" as const, vw: 1024, vh: 768, left: 100, top: 300, triggerWidth: 200, panelHeight: 200, width: 544, expectedLeft: 100, expectedTop: 352, expectedWidth: 544 },
    { name: "below the end edge when both sides have room", align: "end" as const, vw: 1024, vh: 768, left: 700, top: 300, triggerWidth: 80, panelHeight: 200, width: 352, expectedLeft: 428, expectedTop: 352, expectedWidth: 352 },
    { name: "above the start edge when there is no room below", align: "start" as const, vw: 1024, vh: 768, left: 100, top: 600, triggerWidth: 200, panelHeight: 200, width: 544, expectedLeft: 100, expectedTop: 388, expectedWidth: 544 },
    { name: "above the end edge when there is no room below", align: "end" as const, vw: 1024, vh: 768, left: 700, top: 600, triggerWidth: 80, panelHeight: 200, width: 352, expectedLeft: 428, expectedTop: 388, expectedWidth: 352 },
    { name: "below when there is no room above", align: "start" as const, vw: 1024, vh: 768, left: 100, top: 20, triggerWidth: 80, panelHeight: 200, width: 352, expectedLeft: 100, expectedTop: 72, expectedWidth: 352 },
    { name: "within view when the trigger scrolls above the viewport", align: "start" as const, vw: 1024, vh: 768, left: 100, top: -400, triggerWidth: 80, panelHeight: 200, width: 352, expectedLeft: 100, expectedTop: 16, expectedWidth: 352 },
    { name: "within a narrow and short viewport", align: "start" as const, vw: 320, vh: 240, left: 290, top: 200, triggerWidth: 20, panelHeight: 400, width: 544, expectedLeft: 16, expectedTop: 16, expectedWidth: 288 },
  ])("positions $name", ({ align, vw, vh, left, top, triggerWidth, panelHeight, width, expectedLeft, expectedTop, expectedWidth }) => {
    vi.stubGlobal("innerWidth", vw);
    vi.stubGlobal("innerHeight", vh);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
      new DOMRect(left, top, triggerWidth, 40),
    );
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(panelHeight);
    renderPanel({ align, width });

    expect(screen.getByRole("dialog", { name: "Настройки" })).toHaveStyle({
      left: `${expectedLeft}px`, top: `${expectedTop}px`, width: `${expectedWidth}px`, maxHeight: `${vh - 32}px`,
    });
    expect(screen.getByRole("dialog")).toHaveAttribute("data-motion-origin", expectedTop < top ? "bottom" : "top");
  });

  it("follows its trigger when scrolling and stays in bounds after resizing", () => {
    vi.stubGlobal("innerWidth", 1024);
    vi.stubGlobal("innerHeight", 768);
    const bounds = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockReturnValue(new DOMRect(400, 500, 80, 40));
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(200);
    renderPanel();
    const panel = screen.getByRole("dialog", { name: "Настройки" });

    bounds.mockReturnValue(new DOMRect(400, 350, 80, 40));
    fireEvent.scroll(window);
    expect(panel).toHaveStyle({ left: "400px", top: "402px" });

    vi.stubGlobal("innerWidth", 320);
    vi.stubGlobal("innerHeight", 240);
    fireEvent.resize(window);
    expect(panel).toHaveStyle({ left: "16px", top: "24px", width: "288px", maxHeight: "208px" });
  });
});
