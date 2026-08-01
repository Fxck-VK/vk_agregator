import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { AssistantTypingIndicator } from "./AssistantTypingIndicator";

describe("AssistantTypingIndicator", () => {
  afterEach(() => {
    cleanup();
  });

  it("announces a polite three-dot waiting status", () => {
    render(<AssistantTypingIndicator label="NeiroHub is typing" />);

    const status = screen.getByRole("status", { name: "NeiroHub is typing" });
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(status.querySelectorAll('[aria-hidden="true"]')).toHaveLength(3);
  });
});
