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
  it("shows only images, video and music, rotates in six seconds and keeps keyboard navigation", () => {
    const { container } = render(<LoginShowcase />);
    expect(screen.getAllByRole("tab").map(tab => tab.textContent)).toEqual(["Изображения", "Видео", "Музыка"]);
    expect(container.querySelector("video")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(6000));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
    expect(container.querySelector("video")).toHaveAttribute("src", "/assets/videos/auth/creative-video.webm");
    fireEvent.keyDown(screen.getByRole("tab", { name: "Видео" }), { key: "End" });
    expect(screen.getByRole("tab", { name: "Музыка" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("tab", { name: "Музыка" }), { key: "ArrowRight" });
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
  });

  it.each(["narrow", "reduced", "hidden"])("keeps the reference first slide visible without fetching video when %s", condition => {
    wide = condition !== "narrow"; reduced = condition === "reduced"; visible = condition !== "hidden";
    const { container } = render(<LoginShowcase />);
    expect(container.querySelector("img")).toHaveAttribute("src", "/assets/images/auth/creative-image.webp");
    expect(container.querySelector("video")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(12000));
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
  });

  it("keeps a manually chosen reference video playing and advances while it has focus", () => {
    render(<LoginShowcase />);
    vi.mocked(HTMLMediaElement.prototype.play).mockClear();
    const tab = screen.getByRole("tab", { name: "Видео" });
    fireEvent.focus(tab);
    fireEvent.click(tab);
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(6000));
    expect(screen.getByRole("tab", { name: "Музыка" })).toHaveAttribute("aria-selected", "true");
  });

  it.each(["hover", "focus"])("keeps reference slides cycling through every tab during %s", interaction => {
    render(<LoginShowcase />);
    if (interaction === "hover") fireEvent.mouseEnter(screen.getByRole("region", { name: "Возможности сервиса" }));
    else fireEvent.focus(screen.getByRole("tab", { name: "Изображения" }));
    for (const label of ["Видео", "Музыка", "Изображения"]) {
      act(() => vi.advanceTimersByTime(6000));
      expect(screen.getByRole("tab", { name: label })).toHaveAttribute("aria-selected", "true");
    }
  });
  it("supports manual tabs and arrow-key navigation with associated content", () => {
    render(<LoginShowcase />);
    fireEvent.click(screen.getByRole("tab", { name: "Музыка" }));
    expect(screen.getByRole("tabpanel", { name: "Музыка" })).toHaveTextContent("Ваши идеи звучат");
    fireEvent.keyDown(screen.getByRole("tab", { name: "Музыка" }), { key: "End" });
    expect(screen.getByRole("tab", { name: "Музыка" })).toHaveFocus();
    expect(screen.queryByRole("tab", { name: "Текст" })).not.toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("tab", { name: "Музыка" }), { key: "Home" });
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
  });

  it("rotates automatically without a pause control and releases timers when removed", () => {
    const { unmount } = render(<LoginShowcase />);
    act(() => vi.advanceTimersByTime(6000));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("button", { name: "Приостановить слайды" })).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(6000));
    expect(screen.getByRole("tab", { name: "Музыка" })).toHaveAttribute("aria-selected", "true");
    unmount(); expect(vi.getTimerCount()).toBe(0);
  });

  it.each(["narrow", "reduced", "hidden"])("avoids automatic media and rotation when %s", condition => {
    wide = condition !== "narrow"; reduced = condition === "reduced"; visible = condition !== "hidden";
    const { container } = render(<LoginShowcase />);
    act(() => vi.advanceTimersByTime(10000));
    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("tab", { name: "Видео" }));
    expect(container.querySelector("video")).not.toBeInTheDocument();
    expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();
  });

  it("stops an active video and removes its source when the page becomes hidden", () => {
    const { container } = render(<LoginShowcase />);
    fireEvent.click(screen.getByRole("tab", { name: "Видео" }));
    expect(container.querySelector("video")).toHaveAttribute("src");
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    visible = false; notify();
    expect(container.querySelector("video")).not.toBeInTheDocument();
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(10000));
    expect(screen.getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
  });
});
