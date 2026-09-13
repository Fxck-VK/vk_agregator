import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChatTextInput } from "./ChatTextInput";

describe("ChatTextInput", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
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

  it("renders the textarea as a vertical ScrollArea viewport", () => {
    render(
      <ChatTextInput
        appearance="composer"
        disabled={false}
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Ask a question"
        rows={3}
        size="expanded"
        value={"Первая строка\nВторая строка"}
      />,
    );

    const textarea = screen.getByRole("textbox");
    const scrollArea = textarea.closest('[data-ui="chat-text-input-scroll-area"]');

    expect(textarea).toHaveAttribute("data-scroll-area-viewport", "true");
    expect(scrollArea).toHaveAttribute("data-orientation", "vertical");
    expect(scrollArea).toHaveAttribute("data-track-placement", "inset");
    expect(scrollArea).toContainElement(screen.getByTestId("floating-scrollbar-track"));
  });

  it("clamps automatic growth to nine text rows", () => {
    vi.spyOn(HTMLTextAreaElement.prototype, "scrollHeight", "get").mockReturnValue(500);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      bottom: 52,
      height: 52,
      left: 0,
      right: 600,
      top: 0,
      width: 600,
      x: 0,
      y: 0,
      toJSON: () => undefined,
    });
    vi.spyOn(window, "getComputedStyle").mockReturnValue({
      borderBottomWidth: "0px",
      borderTopWidth: "0px",
      fontSize: "16px",
      lineHeight: "20px",
      paddingBottom: "0px",
      paddingTop: "0px",
    } as CSSStyleDeclaration);

    render(
      <ChatTextInput
        appearance="composer"
        disabled={false}
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Ask a question"
        rows={1}
        size="compact"
        value={Array.from({ length: 12 }, (_, index) => `Строка ${index + 1}`).join("\n")}
      />,
    );

    const scrollArea = screen.getByRole("textbox").closest(
      '[data-ui="chat-text-input-scroll-area"]',
    ) as HTMLElement;

    expect(scrollArea.style.getPropertyValue("--chat-text-input-height")).toBe("180px");
    expect(scrollArea).toHaveAttribute("data-max-auto-rows", "9");
  });

  it("manually expands and collapses without changing the draft or caret", () => {
    render(
      <ChatTextInput
        appearance="composer"
        disabled={false}
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Ask a question"
        rows={1}
        size="compact"
        value={"Первая строка\nВторая строка"}
      />,
    );

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    textarea.focus();
    textarea.setSelectionRange(4, 4);

    const expandButton = screen.getByRole("button", { name: "Развернуть поле ввода" });
    expandButton.focus();
    fireEvent.click(expandButton);

    const inputRoot = textarea.closest('[data-ui="chat-text-input"]');
    expect(inputRoot).toHaveAttribute("data-manually-expanded", "true");
    expect(screen.getByRole("button", { name: "Свернуть поле ввода" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(textarea).toHaveValue("Первая строка\nВторая строка");
    expect(textarea).toHaveFocus();
    expect(textarea.selectionStart).toBe(4);

    fireEvent.click(screen.getByRole("button", { name: "Свернуть поле ввода" }));

    expect(inputRoot).toHaveAttribute("data-manually-expanded", "false");
    expect(screen.getByRole("button", { name: "Развернуть поле ввода" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(textarea).toHaveValue("Первая строка\nВторая строка");
  });

  it("shows the expand action only after text reaches a second visual row", () => {
    vi.spyOn(HTMLTextAreaElement.prototype, "scrollHeight", "get").mockImplementation(
      function scrollHeight(this: HTMLTextAreaElement) {
        return this.value.includes("\n") ? 48 : 24;
      },
    );
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      bottom: 31.2,
      height: 31.2,
      left: 0,
      right: 600,
      top: 0,
      width: 600,
      x: 0,
      y: 0,
      toJSON: () => undefined,
    });
    vi.spyOn(window, "getComputedStyle").mockReturnValue({
      borderBottomWidth: "0px",
      borderTopWidth: "0px",
      fontSize: "16px",
      lineHeight: "24px",
      paddingBottom: "0px",
      paddingTop: "0px",
    } as CSSStyleDeclaration);

    const input = {
      appearance: "composer" as const,
      disabled: false,
      onChange: vi.fn(),
      onSend: vi.fn(),
      placeholder: "Ask a question",
      rows: 1,
      size: "compact" as const,
    };
    const { container, rerender } = render(<ChatTextInput {...input} value="Одна строка" />);

    expect(container.querySelector('button[aria-label="Развернуть поле ввода"]')).not.toBeInTheDocument();

    rerender(<ChatTextInput {...input} value={"Первая строка\nВторая строка"} />);

    expect(container.querySelector('button[aria-label="Развернуть поле ввода"]')).toBeVisible();
  });

  it("limits smooth resizing to the manual transition window", () => {
    vi.useFakeTimers();
    render(
      <ChatTextInput
        appearance="composer"
        disabled={false}
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Ask a question"
        rows={1}
        size="compact"
        value={"Первая строка\nВторая строка"}
      />,
    );

    const inputRoot = screen.getByRole("textbox").closest('[data-ui="chat-text-input"]');
    fireEvent.click(screen.getByRole("button", { name: "Развернуть поле ввода" }));

    expect(inputRoot).toHaveAttribute("data-manual-transitioning", "true");

    act(() => vi.advanceTimersByTime(240));

    expect(inputRoot).toHaveAttribute("data-manual-transitioning", "true");

    act(() => vi.advanceTimersByTime(120));

    expect(inputRoot).toHaveAttribute("data-manual-transitioning", "false");
  });
});
