import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MarketingCarousel } from "./MarketingCarousel";
import Link from "@/i18n/Link";

afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals(); });

function arrange(reduced = false) {
  vi.useFakeTimers();
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: reduced, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  render(<MarketingCarousel label="Модели" className="track" auto><Link href="/app/models">Модель</Link></MarketingCarousel>);
  const region = screen.getByRole("region", { name: "Модели" });
  Object.defineProperties(region, { clientWidth: { value: 500 }, scrollWidth: { value: 1600 }, scrollLeft: { value: 0, writable: true } });
  const scrollBy = vi.fn(), scrollTo = vi.fn();
  region.scrollBy = scrollBy; region.scrollTo = scrollTo;
  act(() => vi.advanceTimersByTime(20));
  return { region, scrollBy, scrollTo };
}

describe("marketing carousels", () => {
  it("supports next/previous controls and arrow keys while disabling unavailable directions", () => {
    const { region, scrollBy } = arrange();
    expect(screen.getByRole("button", { name: "Назад: Модели" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Далее: Модели" }));
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 400, behavior: "smooth" });
    region.scrollLeft = 1100; fireEvent.scroll(region);
    expect(screen.getByRole("button", { name: "Далее: Модели" })).toBeDisabled();
    fireEvent.keyDown(region, { key: "ArrowLeft" });
    expect(scrollBy).toHaveBeenLastCalledWith({ left: -400, behavior: "smooth" });
  });

  it("pauses automatic advancement while a visitor focuses or hovers over its content", () => {
    const { region, scrollTo } = arrange();
    act(() => vi.advanceTimersByTime(4500));
    expect(scrollTo).toHaveBeenCalledTimes(1);
    fireEvent.mouseEnter(region.parentElement!);
    screen.getByRole("link", { name: "Модель" }).focus();
    fireEvent.mouseLeave(region.parentElement!);
    act(() => vi.advanceTimersByTime(9000));
    expect(scrollTo).toHaveBeenCalledTimes(1);
  });

  it("does not auto-advance when reduced motion is requested", () => {
    const { scrollTo } = arrange(true);
    act(() => vi.advanceTimersByTime(15000));
    expect(scrollTo).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Далее: Модели" }));
    expect(screen.getByRole("region", { name: "Модели" }).scrollBy).toHaveBeenLastCalledWith({ left: 400, behavior: "instant" });
  });
});
