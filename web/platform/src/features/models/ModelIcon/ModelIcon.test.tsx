import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ModelIcon } from "./ModelIcon";

describe("ModelIcon", () => {
  afterEach(() => {
    cleanup();
  });

  it("uses the supplied chip silhouette as the default artwork", () => {
    render(<ModelIcon />);

    const fallback = screen.getByTestId("model-icon-fallback");

    expect(fallback).toHaveStyle({
      backgroundImage: 'url("/assets/images/models/chip-silhouette.svg")',
    });
    expect(fallback.querySelector("svg")).not.toBeInTheDocument();
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

    expect(screen.getByTestId("model-icon-fallback")).toHaveStyle({
      backgroundImage: 'url("/assets/images/models/chip-silhouette.svg")',
    });
    expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
  });
});
