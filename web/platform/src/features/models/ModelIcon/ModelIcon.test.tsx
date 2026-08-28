import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ModelIcon } from "./ModelIcon";

describe("ModelIcon", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the embedded default artwork without requesting an image", () => {
    render(<ModelIcon />);

    expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
  });

  it("renders supplied model artwork", () => {
    render(<ModelIcon src="/assets/images/models/custom.png" />);

    expect(screen.getByTestId("model-icon")).toHaveAttribute(
      "src",
      expect.stringContaining("custom.png"),
    );
  });

  it("replaces failed supplied artwork with the embedded default artwork", () => {
    render(<ModelIcon src="/assets/images/models/missing.png" />);

    fireEvent.error(screen.getByTestId("model-icon"));

    expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
  });
});
