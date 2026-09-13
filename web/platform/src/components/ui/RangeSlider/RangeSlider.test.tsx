import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RangeSlider } from "./RangeSlider";

function ControlledRangeSlider() {
  const [value, setValue] = useState(32);

  return (
    <RangeSlider
      aria-label="Размер кисти"
      max={80}
      min={8}
      onValueChange={setValue}
      value={value}
    />
  );
}

describe("RangeSlider", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("updates its controlled value and positions the fill at the thumb center", () => {
    render(<ControlledRangeSlider />);

    const slider = screen.getByRole("slider", { name: "Размер кисти" });
    fireEvent.change(slider, { target: { value: "64" } });

    expect(slider).toHaveValue("64");
    expect(slider.parentElement?.style.getPropertyValue("--range-slider-fill")).toBe(
      "calc(77.7778% - 37.3333px + 24px)",
    );
  });

  it("shows the current value while it changes and hides it after the delay", () => {
    vi.useFakeTimers();
    render(<ControlledRangeSlider />);

    const value = screen.getByTestId("range-slider-value");
    expect(value).toHaveAttribute("data-visible", "false");

    fireEvent.change(screen.getByRole("slider", { name: "Размер кисти" }), {
      target: { value: "64" },
    });

    expect(value).toHaveTextContent("64");
    expect(value).toHaveAttribute("data-visible", "true");

    act(() => vi.advanceTimersByTime(600));
    expect(value).toHaveAttribute("data-visible", "false");
  });
});
