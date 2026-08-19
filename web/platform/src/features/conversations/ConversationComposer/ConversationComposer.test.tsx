import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ConversationComposer } from "./ConversationComposer";

const chatScrollProps = {
  contentVersion: "",
  forceScrollRequest: 0,
  scrollContainer: null,
};

describe("ConversationComposer", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("uses the exact NeiroHub question placeholder", () => {
    render(<ConversationComposer {...chatScrollProps} onSubmit={vi.fn()} />);

    expect(screen.getByLabelText(ru.conversations.composerLabel)).toHaveAttribute(
      "placeholder",
      "Задайте вопрос NeiroHub",
    );
  });

  it("renders one compact composer surface with embedded controls and a note below", () => {
    render(<ConversationComposer {...chatScrollProps} onSubmit={vi.fn()} />);

    const mediaButton = screen.getByRole("button", { name: "Загрузить медиа" });
    const submitButton = screen.getByRole("button", { name: ru.conversations.composerSubmit });

    expect(mediaButton).toBeDisabled();
    expect(submitButton.querySelector("svg")).not.toBeNull();
    expect(screen.getByText("Стоимость зависит от выбранной нейросети. Нейросеть может ошибаться")).toBeVisible();
  });

  it("clears and submits a normalized draft immediately when Enter is pressed", () => {
    const onSubmit = vi.fn();
    render(<ConversationComposer {...chatScrollProps} onSubmit={onSubmit} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "  Вопрос с клавиатуры  " } });
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter" });
    fireEvent(textarea, event);

    expect(event.defaultPrevented).toBe(true);
    expect(onSubmit).toHaveBeenCalledWith("Вопрос с клавиатуры");
    expect(textarea).toHaveValue("");
  });

  it("clears and submits from the button without waiting for a promise", () => {
    const onSubmit = vi.fn();
    render(<ConversationComposer {...chatScrollProps} onSubmit={onSubmit} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Продолжи диалог" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    expect(onSubmit).toHaveBeenCalledWith("Продолжи диалог");
    expect(textarea).toHaveValue("");
  });

  it("leaves Shift+Enter to the textarea without submitting", () => {
    const onSubmit = vi.fn();
    render(<ConversationComposer {...chatScrollProps} onSubmit={onSubmit} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Первая строка" } });
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter", shiftKey: true });
    fireEvent(textarea, event);

    expect(event.defaultPrevented).toBe(false);
    expect(onSubmit).not.toHaveBeenCalled();
    expect(textarea).toHaveValue("Первая строка");
  });

  it("does not submit blank or disabled input", () => {
    const onSubmit = vi.fn();
    const rendered = render(<ConversationComposer {...chatScrollProps} onSubmit={onSubmit} />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
    expect(onSubmit).not.toHaveBeenCalled();

    rendered.rerender(<ConversationComposer {...chatScrollProps} disabled initialDraft="Не отправлять" onSubmit={onSubmit} />);
    fireEvent.keyDown(screen.getByLabelText(ru.conversations.composerLabel), { key: "Enter" });
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("passes the active reply state to the circular scroll control", () => {
    const scrollContainer = document.createElement("main");
    Object.defineProperties(scrollContainer, {
      clientHeight: { configurable: true, value: 400 },
      scrollHeight: { configurable: true, value: 1600 },
      scrollTo: { configurable: true, value: vi.fn() },
      scrollTop: { configurable: true, writable: true, value: 100 },
    });

    const rendered = render(
      <ConversationComposer
        {...chatScrollProps}
        isAwaitingResponse
        onSubmit={vi.fn()}
        scrollContainer={scrollContainer}
      />,
    );

    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeVisible();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();

    rendered.rerender(
      <ConversationComposer
        {...chatScrollProps}
        isAwaitingResponse={false}
        onSubmit={vi.fn()}
        scrollContainer={scrollContainer}
      />,
    );

    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
  });
});
