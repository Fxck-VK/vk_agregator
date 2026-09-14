import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ConversationComposer } from "./ConversationComposer";

const chatScrollProps = {
  contentVersion: "",
  forceScrollRequest: 0,
  scrollContainer: null,
};
const textGenerationModel = {
  category: "text" as const,
  categories: ["popular", "text", "free", "study-work"],
  description: "Text model",
  estimate_credits: 0,
  id: "chatgpt",
  name: "NeiroHub Chat",
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

    const mediaControls = screen.getByRole("group", { name: "Медиа и настройки" });
    const submitButton = screen.getByRole("button", { name: ru.conversations.composerSubmit });

    expect(within(mediaControls).queryByRole("button", { name: "Загрузить медиа" })).toBeNull();
    expect(submitButton.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/send-message-white.svg",
    );
    expect(screen.getByText("Стоимость зависит от выбранной нейросети. Нейросеть может ошибаться")).toBeVisible();
  });

  it("clears and submits a normalized draft immediately when Enter is pressed", () => {
    const onSubmit = vi.fn();
    render(<ConversationComposer {...chatScrollProps} generationModel={textGenerationModel} onSubmit={onSubmit} />);

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
    render(<ConversationComposer {...chatScrollProps} generationModel={textGenerationModel} onSubmit={onSubmit} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Продолжи диалог" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    expect(onSubmit).toHaveBeenCalledWith("Продолжи диалог");
    expect(textarea).toHaveValue("");
  });

  it("leaves Shift+Enter to the textarea without submitting", () => {
    const onSubmit = vi.fn();
    render(<ConversationComposer {...chatScrollProps} generationModel={textGenerationModel} onSubmit={onSubmit} />);

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
    const rendered = render(<ConversationComposer {...chatScrollProps} generationModel={textGenerationModel} onSubmit={onSubmit} />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
    expect(onSubmit).not.toHaveBeenCalled();

    rendered.rerender(<ConversationComposer {...chatScrollProps} disabled generationModel={textGenerationModel} initialDraft="Не отправлять" onSubmit={onSubmit} />);
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

it("keeps one input while switching text, image and video controls and submits video options", () => {
 const send = vi.fn();
 const {rerender,unmount} = render(<ConversationComposer {...chatScrollProps} onSubmit={send} />);
 const input = screen.getByLabelText(ru.conversations.composerLabel);
 fireEvent.change(input,{target:{value:"Движущийся журавль"}});
 const video = {category:"video" as const,id:"video_veo_3_1_fast",name:"Veo",description:"Video",allowed_resolutions:["720p"],allowed_durations_sec:[8],allowed_aspect_ratios:["16:9"],default_resolution:"720p",default_duration_sec:8,default_aspect_ratio:"16:9",price_by_option:{"720p:8":100}};
 rerender(<ConversationComposer {...chatScrollProps} generationModel={video} onSubmit={send} />);
 expect(screen.getByLabelText(ru.conversations.composerLabel)).toBe(input);
 expect(input).toHaveValue("Движущийся журавль");
 expect(screen.getByRole("button",{name:"Длительность: 8 с"})).toBeEnabled();
 expect(screen.getByText("Стоимость: 100 токенов")).toBeVisible();
 fireEvent.submit(input.closest("form")!);
 expect(send).toHaveBeenCalledWith("Движущийся журавль",{resolution:"720p",duration_sec:8,aspect_ratio:"16:9"});
 unmount();
});
