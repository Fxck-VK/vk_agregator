import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CapabilitiesCarousel } from "./CapabilitiesCarousel";

describe("CapabilitiesCarousel", () => {
  it("renders every local capability and exposes navigation controls", () => {
    render(<CapabilitiesCarousel />);

    expect(screen.getAllByTestId("capability-card")).toHaveLength(4);
    fireEvent.click(screen.getByRole("button", { name: "Следующая возможность" }));
    expect(screen.getByRole("button", { name: "Предыдущая возможность" })).toBeEnabled();
  });
});
