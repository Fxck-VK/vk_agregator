import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ChatSubmitButton } from "./ChatSubmitButton";

afterEach(cleanup);

describe("ChatSubmitButton", () => {
  it("owns the accessible submit action, shared send icon, tooltip, and disabled state", () => {
    const { rerender } = render(
      <ChatSubmitButton disabled={false} label="Отправить" />,
    );

    const button = screen.getByRole("button", { name: "Отправить" });

    expect(button).toHaveAttribute("type", "submit");
    expect(button).toHaveAttribute("data-ui", "chat-submit-button");
    expect(button.querySelector("svg")).toBeNull();
    expect(button.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/send-message-white.svg",
    );
    expect(screen.getByText("Отправить", { selector: '[role="tooltip"]' })).toBeInTheDocument();

    rerender(<ChatSubmitButton disabled label="Отправить" />);
    expect(screen.getByRole("button", { name: "Отправить" })).toBeDisabled();
  });
});
