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

function renderPrompt({ access = "authenticated", variant = "workspace" }: Parameters<typeof WorkspacePrompt>[0] = {}) {
  return render(
    <WorkspaceConversationListProvider accountId={workspaceAccountId} initialConversations={[]}>
      <WorkspacePrompt access={access} variant={variant} />
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
    vi.clearAllMocks();
    vi.unstubAllGlobals();
    window.sessionStorage.clear();
  });

  it("keeps the dedicated new-chat copy", () => {
    renderPrompt({ variant: "newChat" });

    expect(screen.getByLabelText(ru.conversations.composerPlaceholder)).toHaveAttribute(
      "placeholder",
      ru.conversations.composerPlaceholder,
    );
    expect(screen.getByRole("button", { name: ru.conversations.composerMediaUpload })).toBeEnabled();
    expect(screen.getByRole("button", { name: ru.workspace.promptSubmit }).querySelector("svg")).not.toBeNull();
  });

  it("routes a guest prompt to login without creating private data", () => {
    render(<WorkspacePrompt access="guest" variant="hero" />);

    fireEvent.change(screen.getByLabelText("Задайте вопрос NeiroHub"), { target: { value: "Помоги составить план" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).toHaveBeenCalledWith("/login");
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("opens the temporary conversation immediately without waiting for the server", () => {
    renderPrompt();

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "  Мгновенный диалог  " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).toHaveBeenCalledWith(`/app/chat/${conversationKey}?pending=1`);
    expect(textarea).toHaveValue("");
    expect(webBrowserMutation).not.toHaveBeenCalled();
    expect(readPendingConversationBootstrap(conversationKey)).toEqual({
      conversationKey,
      messageKey,
      prompt: "Мгновенный диалог",
    });
  });

  it("shows the temporary chat in the sidebar before navigation", () => {
    renderPrompt();

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Временное название" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    const sidebar = screen.getByRole("heading", { name: ru.conversations.recentHeading }).closest("section");
    expect(sidebar).not.toBeNull();
    expect(within(sidebar as HTMLElement).getByText("Временное название")).toBeVisible();
  });

  it("creates only one temporary conversation when submit fires twice in the same frame", () => {
    renderPrompt();

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
    renderPrompt();

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).not.toHaveBeenCalled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
});
