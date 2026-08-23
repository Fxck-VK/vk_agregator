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
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Напишите вопрос"
        submitLabel="Отправить"
        value=""
        variant="conversation"
      />,
    );

    expect(screen.getByLabelText("Задайте вопрос NeiroHub")).toHaveAttribute("placeholder", "Напишите вопрос");
    expect(screen.getByRole("button", { name: "Загрузить медиа" })).toBeEnabled();
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

  it("renders optional domain controls alongside the shared media and submit actions", () => {
    render(
      <ChatComposer
        leadingControls={<button type="button">1K</button>}
        canSubmit
        disabled={false}
        label="Генерация изображения"
        mediaLabel="Загрузить медиа"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Опишите изображение"
        submitLabel="Сгенерировать"
        value="Город после дождя"
        variant="conversation"
      />,
    );

    expect(screen.getByRole("button", { name: "Загрузить медиа" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "1K" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Сгенерировать" })).toBeEnabled();
    expect(screen.getByRole("group", { name: "Медиа и настройки" })).toContainElement(
      screen.getByRole("button", { name: "Загрузить медиа" }),
    );
    expect(screen.getByRole("group", { name: "Медиа и настройки" })).toContainElement(
      screen.getByRole("button", { name: "1K" }),
    );
    expect(screen.getByRole("group", { name: "Медиа и настройки" })).not.toContainElement(
      screen.getByRole("button", { name: "Сгенерировать" }),
    );
  });

  it("shows a selected local file in the composer and lets the user remove it", () => {
    const { container } = render(
      <ChatComposer
        canSubmit
        disabled={false}
        label="Диалог"
        mediaLabel="Загрузить медиа"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Напишите вопрос"
        submitLabel="Отправить"
        value="Вопрос"
        variant="conversation"
      />,
    );
    const file = new File(["image"], "reference.png", { type: "image/png" });

    fireEvent.click(screen.getByRole("button", { name: "Загрузить медиа" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Загрузить файл" }));
    const input = container.querySelector('input[type="file"]');
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    expect(screen.getByText("reference.png")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Убрать reference.png" }));
    expect(screen.queryByText("reference.png")).not.toBeInTheDocument();
  });
});
