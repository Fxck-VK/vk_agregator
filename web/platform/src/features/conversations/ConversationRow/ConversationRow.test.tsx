import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { ConversationRow } from "./ConversationRow";

const conversation: ConversationItem = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  title: "Подготовить макет",
  created_at: "2026-07-31T09:00:00Z",
  updated_at: "2026-07-31T09:05:00Z",
};

const refresh = vi.fn();
const replace = vi.fn();

function renderRow(isActive = false, item: ConversationItem = conversation) {
  return render(<ConversationRow conversation={item} isActive={isActive} />);
}

describe("ConversationRow", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ refresh, replace } as never);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("marks the active chat link as the current page", () => {
    renderRow(true);

    expect(screen.getByRole("link", { name: conversation.title })).toHaveAttribute("aria-current", "page");
  });

  it("opens the labelled actions menu and closes it on cancel or Escape", () => {
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    expect(screen.getByRole("button", { name: ru.conversations.renameLabel })).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.cancelLabel }));
    expect(screen.queryByRole("button", { name: ru.conversations.renameLabel })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));

    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("button", { name: ru.conversations.renameLabel })).not.toBeInTheDocument();
  });

  it("starts rename with the unnamed fallback and submits a parsed PATCH response on Enter", async () => {
    const unnamedConversation = { ...conversation, title: "   " };
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(unnamedConversation, { status: 200 }));
    renderRow(false, unnamedConversation);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    expect(titleInput).toHaveValue(ru.conversations.unnamed);

    fireEvent.change(titleInput, { target: { value: "Новый заголовок" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    await vi.waitFor(() =>
      expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/conversations/d7c979f5-24e5-4f88-924b-a592d6e5a906", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: "Новый заголовок" }),
      }),
    );
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it("keeps the typed title and shows neutral feedback after a bad rename response", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response("backend detail", { status: 409 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.change(titleInput, { target: { value: "Не потерять" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.renameFailure);
    expect(titleInput).toHaveValue("Не потерять");
    expect(screen.queryByText("backend detail")).not.toBeInTheDocument();
    expect(refresh).not.toHaveBeenCalled();
  });

  it.each([
    ["a 201 response", () => new Response(null, { status: 201 })],
    ["a malformed 200 DTO", () => Response.json({ id: "not-a-uuid" }, { status: 200 })],
  ])("keeps rename open after %s", async (_caseName, response) => {
    vi.mocked(webBrowserMutation).mockResolvedValue(response());
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.change(titleInput, { target: { value: "Оставить открытым" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.renameFailure);
    expect(titleInput).toHaveValue("Оставить открытым");
    expect(refresh).not.toHaveBeenCalled();
  });

  it("requires confirmation and refreshes after deleting a non-active chat", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    expect(screen.getByText(ru.conversations.archiveConfirmation)).toBeVisible();
    expect(webBrowserMutation).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    await vi.waitFor(() =>
      expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/conversations/d7c979f5-24e5-4f88-924b-a592d6e5a906", {
        method: "DELETE",
      }),
    );
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(replace).not.toHaveBeenCalled();
  });

  it("redirects to the workspace after deleting the active chat", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    renderRow(true);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/app"));
    expect(refresh).not.toHaveBeenCalled();
  });

  it("keeps the row and shows neutral feedback after a failed delete", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response("backend detail", { status: 500 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.archiveFailure);
    expect(screen.getByRole("link", { name: conversation.title })).toBeInTheDocument();
    expect(screen.queryByText("backend detail")).not.toBeInTheDocument();
  });

  it("keeps the row after a 200 delete response instead of treating it as archived", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 200 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.actionsLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.archiveFailure);
    expect(screen.getByRole("link", { name: conversation.title })).toBeInTheDocument();
    expect(refresh).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
  });

  it("disables only the pending row controls", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    render(
      <>
        <ConversationRow conversation={conversation} isActive={false} />
        <ConversationRow conversation={{ ...conversation, id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b", title: "Другой чат" }} isActive={false} />
      </>,
    );

    const actionButtons = screen.getAllByRole("button", { name: ru.conversations.actionsLabel });
    fireEvent.click(actionButtons[0]);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));

    await vi.waitFor(() => expect(actionButtons[0]).toBeDisabled());
    expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled();
    expect(actionButtons[1]).toBeEnabled();

    settleRequest(new Response(null, { status: 500 }));
    await screen.findByRole("alert");
  });
});
