import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CreditAmount } from "./CreditAmount";

describe("CreditAmount", () => {
  it.each([
    [1, "1 звезда"],
    [2, "2 звезды"],
    [5, "5 звёзд"],
    [11, "11 звёзд"],
    [22, "22 звезды"],
  ])("announces %s credits as %s", (value, accessibleLabel) => {
    render(<CreditAmount value={value} />);

    expect(screen.getByLabelText(accessibleLabel)).toBeInTheDocument();
    expect(screen.getByTestId("credit-star-icon")).toBeInTheDocument();
  });

  it("includes the optional prefix in its accessible name", () => {
    render(<CreditAmount prefix="От" value={16} />);

    expect(screen.getByLabelText("От 16 звёзд")).toBeInTheDocument();
  });
});
