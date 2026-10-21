import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoginShowcase } from "./LoginShowcase";

let wide: boolean;
let reduced: boolean;
let visible: boolean;
const listeners = new Set<() => void>();
function notify() { act(() => { listeners.forEach(callback => callback()); document.dispatchEvent(new Event("visibilitychange")); }); }

beforeEach(() => {
  vi.useFakeTimers(); wide = true; reduced = false; visible = true;
  vi.stubGlobal("matchMedia", (query: string) => ({
    get matches() { return query.includes("reduced-motion") ? reduced : wide; },
    addEventListener: (_: string, callback: () => void) => listeners.add(callback),
    removeEventListener: (_: string, callback: () => void) => listeners.delete(callback),
  }));
  vi.spyOn(document, "visibilityState", "get").mockImplementation(() => visible ? "visible" : "hidden");
  vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(undefined);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
});
afterEach(() => { cleanup(); listeners.clear(); vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("LoginShowcase", () => {
  it("supports manual tabs and arrow-key navigation with associated content", () => {
    render(<LoginShowcase />);
    fireEvent.click(screen.getByRole("tab", { name: "Музыка" }));
    expect(screen.getByRole("tabpanel", { name: "Музыка" })).toHaveTextContent("Ваши идеи звучат");
    fireEvent.keyDown(screen.getByRole("tab", { name: "Музыка" }), { key: "End" });
    expect(screen.getByRole("tab", { name: "Текст" })).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: "Текст" })).toHaveTextContent("Найдите нужные слова");
    fireEvent.keyDown(screen.getByRole("tab", { name: "Текст" }), { key: "Home" });
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
  });

  it("rotates automatically, honors pause, and releases timers when removed", () => {
    const { unmount } = render(<LoginShowcase />);
    act(() => vi.advanceTimersByTime(6500));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("button", { name: "Приостановить слайды" }));
    act(() => vi.advanceTimersByTime(20000));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("button", { name: "Продолжить слайды" }));
    act(() => vi.advanceTimersByTime(6500));
    expect(screen.getByRole("tab", { name: "Музыка" })).toHaveAttribute("aria-selected", "true");
    unmount(); expect(vi.getTimerCount()).toBe(0);
  });

  it.each(["narrow", "reduced", "hidden"])("avoids automatic media and rotation when %s", condition => {
    wide = condition !== "narrow"; reduced = condition === "reduced"; visible = condition !== "hidden";
    const { container } = render(<LoginShowcase />);
    act(() => vi.advanceTimersByTime(10000));
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("tab", { name: "Видео" }));
    expect(container.querySelector("video")).not.toHaveAttribute("src");
    expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();
  });

  it("stops an active video and removes its source when the page becomes hidden", () => {
    const { container } = render(<LoginShowcase />);
    fireEvent.click(screen.getByRole("tab", { name: "Видео" }));
    expect(container.querySelector("video")).toHaveAttribute("src");
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    visible = false; notify();
    expect(container.querySelector("video")).not.toHaveAttribute("src");
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(10000));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
  });
});
