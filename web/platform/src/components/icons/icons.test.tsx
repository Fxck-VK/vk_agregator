import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CheckIcon } from "./CheckIcon";
import { CopyIcon } from "./CopyIcon";

describe("shared icons", () => {
  it("renders the copy icon as a decorative, theme-aware SVG", () => {
    render(<CopyIcon data-testid="copy-icon" />);

    const icon = screen.getByTestId("copy-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", "copy");
    expect(icon).toHaveAttribute("fill", "none");
  });

  it("renders the check icon as a decorative, theme-aware SVG", () => {
    render(<CheckIcon data-testid="check-icon" />);

    const icon = screen.getByTestId("check-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", "check");
    expect(icon).toHaveAttribute("fill", "none");
  });
});
