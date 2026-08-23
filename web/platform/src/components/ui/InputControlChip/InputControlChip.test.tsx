import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { InputControlChip } from "./InputControlChip";

describe("InputControlChip", () => {
  it("renders an accessible shared button shell", () => {
    const onClick = vi.fn();

    render(
      <InputControlChip aria-expanded="false" onClick={onClick}>
        <span aria-hidden="true">icon</span>
        <span>16:9</span>
      </InputControlChip>,
    );

    const trigger = screen.getByRole("button", { name: "16:9" });
    expect(trigger).toHaveAttribute("data-ui", "input-control-chip");
    expect(trigger).toHaveAttribute("type", "button");
    fireEvent.click(trigger);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("uses the same shell for grouped controls", () => {
    render(
      <InputControlChip as="div" aria-label="Количество изображений" role="group">
        <button type="button">−</button>
        <span>1 / 4</span>
        <button type="button">+</button>
      </InputControlChip>,
    );

    expect(screen.getByRole("group", { name: "Количество изображений" })).toHaveAttribute(
      "data-ui",
      "input-control-chip",
    );
  });
});
