import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ImageOutputCountSelector } from "./ImageOutputCountSelector";

describe("ImageOutputCountSelector", () => {
  afterEach(cleanup);

  it("changes the count within the model limit", () => {
    const onChange = vi.fn();
    render(<ImageOutputCountSelector max={4} onChange={onChange} value={2} />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.increaseOutputCount }));
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.decreaseOutputCount }));

    expect(onChange).toHaveBeenNthCalledWith(1, 3);
    expect(onChange).toHaveBeenNthCalledWith(2, 1);
    expect(screen.getByText("2 / 4")).toBeVisible();
  });

  it("disables controls at their bounds", () => {
    const { rerender } = render(<ImageOutputCountSelector max={4} onChange={vi.fn()} value={1} />);
    expect(screen.getByRole("button", { name: ru.imageGeneration.decreaseOutputCount })).toBeDisabled();

    rerender(<ImageOutputCountSelector max={4} onChange={vi.fn()} value={4} />);
    expect(screen.getByRole("button", { name: ru.imageGeneration.increaseOutputCount })).toBeDisabled();
  });
});
