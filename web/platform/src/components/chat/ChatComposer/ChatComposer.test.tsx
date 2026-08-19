import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChatComposer } from "./ChatComposer";

describe("ChatComposer", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the shared chat controls and accessible textarea", () => {
    render(
      <ChatComposer
        canSubmit={false}
        disabled={false}
        label="Задайте вопрос NeiroHub"
        mediaLabel="Загрузить медиа"
        mediaUnavailableLabel="Загрузка пока недоступна"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Напишите вопрос"
        submitLabel="Отправить"
        value=""
        variant="conversation"
      />,
    );

    expect(screen.getByLabelText("Задайте вопрос NeiroHub")).toHaveAttribute("placeholder", "Напишите вопрос");
    expect(screen.getByRole("button", { name: "Загрузить медиа" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Отправить" })).toBeDisabled();
  });

  it("keeps Enter submission in every visual variant", () => {
    const onSend = vi.fn();
    render(
      <ChatComposer
        canSubmit
        disabled={false}
        label="Новый чат"
        mediaLabel="Загрузить медиа"
        mediaUnavailableLabel="Загрузка пока недоступна"
        onChange={vi.fn()}
        onSend={onSend}
        placeholder="Напишите вопрос"
        submitLabel="Начать чат"
        value="Вопрос"
        variant="newChat"
      />,
    );

    fireEvent.keyDown(screen.getByLabelText("Новый чат"), { key: "Enter" });

    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("renders an optional note outside the composer surface", () => {
    render(
      <ChatComposer
        canSubmit
        disabled={false}
        label="Диалог"
        mediaLabel="Загрузить медиа"
        mediaUnavailableLabel="Загрузка пока недоступна"
        note="Стоимость зависит от выбранной нейросети"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Напишите вопрос"
        submitLabel="Отправить"
        value="Вопрос"
        variant="conversation"
      />,
    );

    expect(screen.getByText("Стоимость зависит от выбранной нейросети")).toBeVisible();
  });
});
