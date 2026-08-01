import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChatTextInput } from "./ChatTextInput";

describe("ChatTextInput", () => {
  afterEach(() => {
    cleanup();
  });

  it("sends once and prevents the default action when Enter is pressed", () => {
    const onSend = vi.fn();
    render(
      <ChatTextInput
        appearance="plain"
        disabled={false}
        onChange={vi.fn()}
        onSend={onSend}
        placeholder="Ask a question"
        rows={5}
        size="expanded"
        value="Draft"
      />,
    );

    const textarea = screen.getByRole("textbox");
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter" });
    fireEvent(textarea, event);

    expect(event.defaultPrevented).toBe(true);
    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("leaves Shift+Enter to the textarea without sending", () => {
    const onSend = vi.fn();
    render(
      <ChatTextInput
        appearance="inset"
        disabled={false}
        onChange={vi.fn()}
        onSend={onSend}
        placeholder="Ask a question"
        rows={3}
        size="compact"
        value="Draft"
      />,
    );

    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter", shiftKey: true });
    fireEvent(screen.getByRole("textbox"), event);

    expect(event.defaultPrevented).toBe(false);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("does not send or prevent Enter while an IME composition is active", () => {
    const onSend = vi.fn();
    render(
      <ChatTextInput
        appearance="plain"
        disabled={false}
        onChange={vi.fn()}
        onSend={onSend}
        placeholder="Ask a question"
        rows={5}
        size="expanded"
        value="Draft"
      />,
    );

    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter" });
    Object.defineProperty(event, "isComposing", { value: true });
    fireEvent(screen.getByRole("textbox"), event);

    expect(event.defaultPrevented).toBe(false);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("does not send from a disabled textarea", () => {
    const onSend = vi.fn();
    render(
      <ChatTextInput
        appearance="inset"
        disabled
        onChange={vi.fn()}
        onSend={onSend}
        placeholder="Ask a question"
        rows={3}
        size="compact"
        value="Draft"
      />,
    );

    fireEvent.keyDown(screen.getByRole("textbox"), { key: "Enter" });

    expect(onSend).not.toHaveBeenCalled();
  });
});
