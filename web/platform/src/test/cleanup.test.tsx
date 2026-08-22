import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

describe("React test cleanup", () => {
  it("renders a marker for the current test", () => {
    render(<div>cleanup marker</div>);

    expect(screen.getByText("cleanup marker")).toBeInTheDocument();
  });

  it("starts the next test with an empty document", () => {
    expect(screen.queryByText("cleanup marker")).not.toBeInTheDocument();
  });
});
