import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ModelIcon } from "./ModelIcon";

describe("ModelIcon", () => {
  afterEach(() => {
    cleanup();
  });

  it("loads the public fallback artwork directly", () => {
    render(<ModelIcon />);

    const source = screen.getByTestId("model-icon").getAttribute("src") ?? "";

    expect(new URL(source, "http://localhost").pathname).toBe(
      "/assets/images/models/default-model-87465de8.png",
    );
    expect(source).not.toContain("/_next/image");
  });

  it("replaces a failed image with the embedded fallback artwork", () => {
    render(<ModelIcon />);

    fireEvent.error(screen.getByTestId("model-icon"));

    expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
  });
});
