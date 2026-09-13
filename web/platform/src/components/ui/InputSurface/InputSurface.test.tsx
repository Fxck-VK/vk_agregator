import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { InputSurface } from "./InputSurface";

afterEach(cleanup);

describe("InputSurface", () => {
  it("owns the shared surface hook while preserving consumer classes and attributes", () => {
    render(
      <InputSurface aria-label="Общее поле" className="consumer-layout" data-testid="surface">
        Содержимое
      </InputSurface>,
    );

    const surface = screen.getByTestId("surface");

    expect(surface).toHaveAttribute("data-ui", "input-surface");
    expect(surface).toHaveAccessibleName("Общее поле");
    expect(surface.className).toContain("consumer-layout");
    expect(surface).toHaveTextContent("Содержимое");
  });
});
