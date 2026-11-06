import { render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { HomeLoop } from "./HomeMedia";

afterEach(() => vi.unstubAllGlobals());

it("keeps decorative video paused when the visitor requests reduced motion", () => {
  vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
    matches: query === "(prefers-reduced-motion: reduce)",
    addEventListener: vi.fn(), removeEventListener: vi.fn(),
  })));
  const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
  const { container } = render(<HomeLoop file="creative-suite-image.webm" poster="product-shots.webp" paused={false} />);
  expect(container.querySelector("video")).not.toHaveAttribute("autoplay");
  expect(pause).toHaveBeenCalled();
});
