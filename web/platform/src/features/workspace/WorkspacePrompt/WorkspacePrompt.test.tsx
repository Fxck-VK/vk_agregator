import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { usePathname, useRouter } from "next/navigation";

import { readPendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { SidebarConversations } from "@/features/conversations/SidebarConversations/SidebarConversations";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { WorkspacePrompt } from "./WorkspacePrompt";

const push = vi.fn();
const conversationKey = "c7c979f5-24e5-4f88-924b-a592d6e5a906";
const messageKey = "e7c979f5-24e5-4f88-924b-a592d6e5a906";
const workspaceAccountId = "0ce06a6a-16d8-4b16-b9df-5e63175a4a0c";
const selectedTextModel = {
  id: "default_text_model",
  name: "Default Text Model",
  description: "Каталожная текстовая модель",
  categories: ["popular", "text", "study-work", "free"],
  estimate_credits: 0,
};

function renderPrompt({ access = "authenticated", variant = "workspace", ...props }: Parameters<typeof WorkspacePrompt>[0] = {}) {
  return render(
    <WorkspaceConversationListProvider accountId={workspaceAccountId} initialConversations={[]}>
      <WorkspacePrompt {...props} access={access} variant={variant} />
      <SidebarConversations />
    </WorkspaceConversationListProvider>,
  );
}

describe("WorkspacePrompt", () => {
  beforeEach(() => {
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ push } as never);
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn().mockReturnValueOnce(conversationKey).mockReturnValueOnce(messageKey),
    });
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
    window.sessionStorage.clear();
  });

  it("keeps the animated hero prompt visible on focus and hides it only after typing", () => {
    vi.useFakeTimers();
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    render(<WorkspacePrompt access="guest" variant="hero" />);

    const textarea = screen.getByLabelText("Задайте вопрос NeiroHub");
    expect(textarea).toHaveAttribute("placeholder", "Спросите NeiroHub или ");

    fireEvent.focus(textarea);
    expect(textarea).toHaveAttribute("placeholder", "Спросите NeiroHub или ");

    act(() => vi.advanceTimersByTime(80));
    expect(textarea).toHaveAttribute("placeholder", "Спросите NeiroHub или с");

    fireEvent.change(textarea, { target: { value: "Мой вопрос" } });
    expect(textarea).toHaveAttribute("placeholder", "");

    fireEvent.change(textarea, { target: { value: "" } });
    expect(textarea).toHaveAttribute("placeholder", "Спросите NeiroHub или ");
  });

  it("shows a static complete hero suggestion when reduced motion is requested", () => {
    vi.useFakeTimers();
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    render(<WorkspacePrompt access="guest" variant="hero" />);

    const textarea = screen.getByLabelText("Задайте вопрос NeiroHub");
    expect(textarea).toHaveAttribute(
      "placeholder",
      "Спросите NeiroHub или составьте план проекта",
    );

    act(() => vi.advanceTimersByTime(10_000));
    expect(textarea).toHaveAttribute(
      "placeholder",
      "Спросите NeiroHub или составьте план проекта",
    );
  });

  it("deletes the completed suggestion and types the next one", () => {
    vi.useFakeTimers();
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    render(<WorkspacePrompt access="guest" variant="hero" />);

    const textarea = screen.getByLabelText("Задайте вопрос NeiroHub");
    const prefix = "Спросите NeiroHub или ";
    const firstSuggestion = "составьте план проекта";

    for (let index = 0; index < firstSuggestion.length; index += 1) {
      act(() => vi.advanceTimersByTime(80));
    }
    expect(textarea).toHaveAttribute("placeholder", `${prefix}${firstSuggestion}`);

    act(() => vi.advanceTimersByTime(1_600));
    act(() => vi.advanceTimersByTime(45));
    expect(textarea).toHaveAttribute(
      "placeholder",
      `${prefix}${firstSuggestion.slice(0, -1)}`,
    );

    for (let index = 1; index < firstSuggestion.length; index += 1) {
      act(() => vi.advanceTimersByTime(45));
    }
    act(() => vi.advanceTimersByTime(350));
    act(() => vi.advanceTimersByTime(80));
    expect(textarea).toHaveAttribute("placeholder", `${prefix}п`);
  });

  it("keeps the dedicated new-chat copy", () => {
    renderPrompt({ variant: "newChat" });

    expect(screen.getByLabelText(ru.conversations.composerPlaceholder)).toHaveAttribute(
      "placeholder",
      ru.conversations.composerPlaceholder,
    );
    expect(screen.getByRole("button", { name: ru.conversations.composerMediaUpload })).toBeEnabled();
    expect(screen.getByRole("button", { name: ru.workspace.promptSubmit }).querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/send-message-white.svg",
    );
  });

  it("routes a guest prompt to login without creating private data", () => {
    render(<WorkspacePrompt access="guest" selectedChatModel={selectedTextModel} variant="hero" />);

    fireEvent.change(screen.getByLabelText("Задайте вопрос NeiroHub"), { target: { value: "Помоги составить план" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).toHaveBeenCalledWith("/login");
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("opens the temporary conversation immediately without waiting for the server", () => {
    renderPrompt({ selectedChatModel: selectedTextModel });

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "  Мгновенный диалог  " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).toHaveBeenCalledWith(`/app/chat/${conversationKey}?pending=1`);
    expect(textarea).toHaveValue("");
    expect(webBrowserMutation).not.toHaveBeenCalled();
    expect(readPendingConversationBootstrap(conversationKey)).toMatchObject({
      conversationKey,
      messageKey,
      modelId: selectedTextModel.id,
      prompt: "Мгновенный диалог",
    });
  });

  it("captures the selected chat model in the first-message intent", () => {
    renderPrompt({ selectedChatModel: { id: "gpt_5_5", name: "GPT-5.5", description: "Каталожное описание GPT-5.5", categories: ["popular", "text", "study-work"], estimate_credits: 20, max_prompt_bytes: 32 } });
    const input = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(input, { target: { value: "я".repeat(17) } });
    fireEvent.submit(input.closest("form")!);
    expect(push).not.toHaveBeenCalled();
    expect(input).toHaveValue("я".repeat(17));
    fireEvent.change(input, { target: { value: "Вопрос" } });
    fireEvent.submit(input.closest("form")!);
    expect(readPendingConversationBootstrap(conversationKey)).toMatchObject({ modelId: "gpt_5_5", prompt: "Вопрос" });
  });

  it("shows the temporary chat in the sidebar before navigation", () => {
    renderPrompt({ selectedChatModel: selectedTextModel });

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Временное название" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    const sidebar = screen.getByRole("heading", { name: ru.conversations.recentHeading }).closest("section");
    expect(sidebar).not.toBeNull();
    expect(within(sidebar as HTMLElement).getByText("Временное название")).toBeVisible();
  });

  it("creates only one temporary conversation when submit fires twice in the same frame", () => {
    renderPrompt({ selectedChatModel: selectedTextModel });

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    const form = textarea.closest("form");
    expect(form).not.toBeNull();
    fireEvent.change(textarea, { target: { value: "Один диалог" } });

    act(() => {
      fireEvent.submit(form as HTMLFormElement);
      fireEvent.submit(form as HTMLFormElement);
    });

    expect(push).toHaveBeenCalledTimes(1);
  });

  it("does not submit an empty prompt", () => {
    renderPrompt({ selectedChatModel: selectedTextModel });

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).not.toHaveBeenCalled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("keeps submission disabled when no selected model record is available", () => {
    renderPrompt();

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Есть текст" } });

    expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    expect(push).not.toHaveBeenCalled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
});
