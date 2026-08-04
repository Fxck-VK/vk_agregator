import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { HowItWorks } from "./HowItWorks";

describe("HowItWorks", () => {
  it("explains the flow in three steps and provides a poster fallback", () => {
    render(<HowItWorks />);

    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.getByRole("img", { name: /интерфейс NeiroHub/i })).toHaveAttribute("loading", "lazy");
  });
});
