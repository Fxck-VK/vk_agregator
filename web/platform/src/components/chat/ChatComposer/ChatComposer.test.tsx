import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChatComposer } from "./ChatComposer";
import type { ChatAttachments } from "@/features/conversations/use-chat-attachments";

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

    const textarea = screen.getByLabelText("Задайте вопрос NeiroHub");

    expect(textarea).toHaveAttribute("placeholder", "Напишите вопрос");
    expect(textarea.closest('[data-ui="input-surface"]')).not.toBeNull();
    expect(screen.getByRole("button", { name: "Загрузить медиа" })).toBeEnabled();
    const submitButton = screen.getByRole("button", { name: "Отправить" });
    expect(submitButton).toBeDisabled();
    expect(submitButton).toHaveAttribute("data-ui", "chat-submit-button");
    expect(submitButton).not.toHaveAttribute("title");
    expect(screen.getByText("Отправить", { selector: '[role="tooltip"]' })).toHaveAttribute(
      "data-ui",
      "tooltip-bubble",
    );
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

  it.each(["hero", "newChat"] as const)("uses a one-row textarea in the %s composer", (variant) => {
    render(
      <ChatComposer
        canSubmit={false}
        disabled={false}
        label="Главный запрос"
        mediaLabel="Загрузить медиа"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Спросите NeiroHub"
        submitLabel="Отправить"
        value=""
        variant={variant}
      />,
    );

    expect(screen.getByLabelText("Главный запрос")).toHaveAttribute("rows", "1");
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

  it("keeps shared control styling component-owned without a radius variant", () => {
    const { container } = render(
      <ChatComposer
        canSubmit
        disabled={false}
        label="Генерация изображения"
        mediaLabel="Загрузить медиа"
        onChange={vi.fn()}
        onSend={vi.fn()}
        placeholder="Опишите изображение"
        submitLabel="Сгенерировать"
        value="Город после дождя"
        variant="hero"
      />,
    );

    expect(container.querySelector("[data-control-radius]")).not.toBeInTheDocument();
  });

  it("rejects attachments when no supported model controller is connected", () => {
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

    expect(screen.getByRole("alert")).toHaveTextContent("Выбранная модель не поддерживает вложения");
    expect(screen.queryByText("reference.png")).not.toBeInTheDocument();
  });
});

describe("attachment previews", () => {
  afterEach(cleanup);
  const remove = vi.fn(); const retry = vi.fn();
  function composer(status: "uploading" | "ready" | "failed", progress: number | null = 40) {
    const attachments: ChatAttachments = {
      enabled: true, items: [{ id: "photo", fingerprint: "test", name: "diagram.png", mimeType: "image/png", source: "uploaded", previewUrl: "blob:photo", status, progress, error: status === "failed" ? "Не удалось загрузить файл." : undefined }],
      duplicateNotice: false, dismissDuplicateNotice: vi.fn(),
      networkNotice: false, dismissNetworkNotice: vi.fn(),
      add: vi.fn(), remove, retry, clear: vi.fn(), error: null, blocked: status !== "ready", ids: status === "ready" ? ["photo"] : [],
    };
    return <ChatComposer attachmentController={attachments} canSubmit disabled={false} label="Диалог" mediaLabel="Загрузить медиа" onChange={vi.fn()} onSend={vi.fn()} placeholder="Вопрос" submitLabel="Отправить" value="Черновик" variant="conversation" />;
  }
  it("places a photo before the draft, offers a remove tooltip and removes the exact file", () => {
    render(composer("ready"));
    const image = screen.getByRole("img", { name: "diagram.png" });
    fireEvent.load(image);
    expect(image.compareDocumentPosition(screen.getByRole("textbox")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.queryByText("diagram.png")).toBeNull();
    expect(screen.getByRole("tooltip", { name: "Удалить файл", hidden: true })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Удалить файл: diagram.png" }));
    expect(remove).toHaveBeenCalledWith("photo");
    expect(screen.queryByRole("progressbar")).toBeNull();
  });
  it("shows measured progress until ready and keeps retry available on failure", () => {
    const { rerender } = render(composer("uploading"));
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "40");
    expect(screen.getByRole("button", { name: "Отправить" })).toBeDisabled();
    rerender(composer("uploading", null));
    expect(screen.getByRole("progressbar")).not.toHaveAttribute("aria-valuenow");
    rerender(composer("uploading", 100));
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuetext", "Обработка файла…");
    expect(screen.getByRole("button", { name: "Отправить" })).toBeDisabled();
    rerender(composer("failed"));
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить файл.");
    const retryButton = screen.getByRole("button", { name: "Повторить загрузку: diagram.png" });
    expect(retryButton.textContent).toBe("");
    expect(retryButton).toHaveAccessibleDescription("Не удалось загрузить файл.");
    expect(screen.getByRole("tooltip", { name: "Повторить", hidden: true })).toBeInTheDocument();
    fireEvent.click(retryButton);
    expect(retry).toHaveBeenCalledWith("photo");
    rerender(composer("ready"));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Отправить" })).toBeEnabled();
    expect(screen.getByRole("textbox")).toHaveValue("Черновик");
  });
  it("keeps the local photo open across upload states and preserves the draft", () => {
    const view = render(composer("uploading"));
    const trigger = screen.getByRole("button", { name: "Просмотр файла: diagram.png" });
    trigger.focus();
    fireEvent.click(trigger);
    expect(screen.getByRole("dialog", { name: "Просмотр файла" })).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(screen.queryByRole("button", { name: "Следующий файл" })).toBeNull();
    expect(screen.getByRole("dialog").querySelector("img")).toHaveAttribute("src", "blob:photo");
    for (const status of ["failed", "uploading", "ready"] as const) {
      view.rerender(composer(status));
      expect(screen.getByRole("dialog", { name: "Просмотр файла" })).toBeInTheDocument();
      expect(screen.getByRole("dialog").querySelector("img")).toHaveAttribute("src", "blob:photo");
    }
    fireEvent.click(screen.getByRole("button", { name: "Закрыть предпросмотр" }));
    fireEvent.animationEnd(screen.getByTestId("attachment-preview-backdrop"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(trigger).toHaveFocus();
    expect(screen.getByRole("textbox")).toHaveValue("Черновик");
    fireEvent.click(screen.getByRole("button", { name: "Удалить файл: diagram.png" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
