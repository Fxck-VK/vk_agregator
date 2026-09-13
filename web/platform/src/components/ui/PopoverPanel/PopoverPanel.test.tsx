import { createRef } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PopoverPanel } from "./PopoverPanel";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

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
