import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { LandingCarousel } from "./LandingCarousel";

afterEach(() => vi.restoreAllMocks());
it("disables controls at the actual catalog carousel edges", () => {
  const { container } = render(<LandingCarousel label="Plugins" previous="Previous" next="Next" bounded><div>One</div><div>Two</div></LandingCarousel>);
  const viewport = container.querySelector('[data-landing-carousel-viewport]') as HTMLElement;
  expect(viewport).not.toBeNull();
  Object.defineProperties(viewport, { clientWidth: { configurable: true, value: 500 }, scrollWidth: { configurable: true, value: 1500 } });
  viewport.scrollBy = vi.fn();
  fireEvent(window, new Event("resize"));
  expect(screen.getByRole("button", { name: "Previous: Plugins" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Next: Plugins" })).toBeEnabled();
  fireEvent.click(screen.getByRole("button", { name: "Next: Plugins" }));
  expect(viewport.scrollBy).toHaveBeenCalledWith({ left: 400, behavior: "smooth" });
  viewport.scrollLeft = 1000; fireEvent.scroll(viewport);
  expect(screen.getByRole("button", { name: "Next: Plugins" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Previous: Plugins" })).toBeEnabled();
});
it("preserves the old unbounded behavior unless requested", () => {
  render(<LandingCarousel label="Existing" previous="Previous" next="Next"><div>One</div></LandingCarousel>);
  expect(screen.getByRole("button", { name: "Previous: Existing" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Next: Existing" })).toBeEnabled();
});
