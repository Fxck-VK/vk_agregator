import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ children, href, prefetch, ...props }: { children: ReactNode; href: string; prefetch?: boolean }) => (
    <a data-prefetch={String(prefetch)} href={href} {...props}>
      {children}
    </a>
  ),
}));

import { usePathname, useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { ConversationRow } from "./ConversationRow";
import {
  useWorkspaceConversationList,
  WorkspaceConversationListProvider,
} from "../WorkspaceConversationList/WorkspaceConversationList";

const conversation: ConversationItem = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  title: "Подготовить макет",
  created_at: "2026-07-31T09:00:00Z",
  updated_at: "2026-07-31T09:05:00Z",
};

const refresh = vi.fn();
const replace = vi.fn();

function actionsLabel(item: ConversationItem = conversation) {
  return `${ru.conversations.actionsLabel}: ${item.title.trim() || ru.conversations.unnamed}`;
}

function renderRow(isActive = false, item: ConversationItem = conversation) {
  return render(<ConversationRow conversation={item} isActive={isActive} />);
}

function ContextConversationRow() {
  const conversationList = useWorkspaceConversationList();
  const currentConversation = conversationList.conversations.find((item) => item.id === conversation.id);

  if (currentConversation === undefined) return null;
  return <ConversationRow conversation={currentConversation} isActive={false} />;
}

function renderContextRow() {
  return render(
    <WorkspaceConversationListProvider accountId="account-1" initialConversations={[conversation]}>
      <ContextConversationRow />
    </WorkspaceConversationListProvider>,
  );
}

describe("ConversationRow", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ refresh, replace } as never);
    vi.mocked(usePathname).mockReturnValue("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906");
  });

  afterEach(() => {
    cleanup();
    refresh.mockReset();
    replace.mockReset();
    vi.clearAllMocks();
  });

  it("marks the active chat link as the current page", () => {
    renderRow(true);

    expect(screen.getByRole("link", { name: conversation.title })).toHaveAttribute("aria-current", "page");
  });

  it("does not prefetch the conversation-specific route", () => {
    renderRow();

    expect(screen.getByRole("link", { name: conversation.title })).toHaveAttribute("data-prefetch", "false");
  });

  it("renders the labelled actions menu in an overlay and closes it on outside click or Escape", () => {
    const rendered = renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    const menu = screen.getByRole("menu");
    expect(screen.getByRole("button", { name: ru.conversations.renameLabel })).toBeVisible();
    expect(rendered.container).not.toContainElement(menu);

    fireEvent.pointerDown(document.body);
    expect(screen.queryByRole("button", { name: ru.conversations.renameLabel })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));

    fireEvent.keyDown(screen.getByRole("button", { name: actionsLabel() }), { key: "Escape" });
    expect(screen.queryByRole("button", { name: ru.conversations.renameLabel })).not.toBeInTheDocument();
  });

  it("starts rename with the unnamed fallback and submits a parsed PATCH response on Enter", async () => {
    const unnamedConversation = { ...conversation, title: "   " };
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(unnamedConversation, { status: 200 }));
    renderRow(false, unnamedConversation);

    fireEvent.click(screen.getByRole("button", { name: actionsLabel(unnamedConversation) }));
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

  it("shows a renamed chat before the PATCH request settles", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    renderContextRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.change(titleInput, { target: { value: "Оптимистичное название" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    expect(screen.getByRole("link", { name: "Оптимистичное название" })).toBeVisible();

    settleRequest(Response.json({ ...conversation, title: "Оптимистичное название" }, { status: 200 }));
    await vi.waitFor(() => expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument());
  });

  it("rolls a failed rename back and offers retry without losing the proposed title", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    renderContextRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.change(titleInput, { target: { value: "Не потерять это название" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    expect(screen.getByRole("link", { name: "Не потерять это название" })).toBeVisible();
    settleRequest(new Response(null, { status: 500 }));

    expect(await screen.findByRole("link", { name: conversation.title })).toBeVisible();
    expect(titleInput).toHaveValue("Не потерять это название");
    expect(screen.getByRole("button", { name: "Повторить" })).toBeEnabled();
  });

  it("restores action-toggle focus before refreshing after a successful rename", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(conversation, { status: 200 }));
    renderRow();
    const actions = screen.getByRole("button", { name: actionsLabel() });
    refresh.mockImplementation(() => {
      expect(actions).toBeEnabled();
      expect(actions).toHaveFocus();
    });

    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel }), { key: "Enter" });

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("keeps the typed title and shows neutral feedback after a bad rename response", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response("backend detail", { status: 409 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.change(titleInput, { target: { value: "Не потерять" } });
    fireEvent.keyDown(titleInput, { key: "Enter" });

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.renameFailure);
    expect(titleInput).toHaveValue("Не потерять");
    expect(screen.queryByText("backend detail")).not.toBeInTheDocument();
    expect(refresh).not.toHaveBeenCalled();
  });

  it("returns focus to the rename input after a failed rename mutation", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 500 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    const submit = screen.getByRole("button", { name: ru.conversations.renameSubmitLabel });
    submit.focus();
    fireEvent.click(submit);

    await screen.findByRole("alert");
    await vi.waitFor(() => expect(titleInput).toHaveFocus());
  });

  it.each([
    ["a 201 response", () => new Response(null, { status: 201 })],
    ["a malformed 200 DTO", () => Response.json({ id: "not-a-uuid" }, { status: 200 })],
  ])("keeps rename open after %s", async (_caseName, response) => {
    vi.mocked(webBrowserMutation).mockResolvedValue(response());
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
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

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    const dialog = screen.getByRole("dialog", { name: ru.conversations.archiveDialogTitle });
    expect(dialog).toBeVisible();
    expect(within(dialog).getByText(conversation.title, { exact: false })).toBeVisible();
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

  it("closes delete confirmation from cancel, backdrop, or Escape without deleting", () => {
    renderRow();

    const openDialog = () => {
      fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
      fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
      return screen.getByRole("dialog", { name: ru.conversations.archiveDialogTitle });
    };

    openDialog();
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.cancelLabel }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const backdropDialog = openDialog();
    fireEvent.mouseDown(backdropDialog.parentElement!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const escapeDialog = openDialog();
    fireEvent.keyDown(escapeDialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("hides a deleted chat before the DELETE request settles and restores it on failure", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    expect(screen.queryByRole("link", { name: conversation.title })).not.toBeInTheDocument();
    settleRequest(new Response(null, { status: 500 }));

    expect(await screen.findByRole("link", { name: conversation.title })).toBeVisible();
    expect(screen.getByRole("alert")).toHaveTextContent(ru.conversations.archiveFailure);
  });

  it("redirects to the workspace after deleting the active chat", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    renderRow(true);

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/app"));
    expect(refresh).not.toHaveBeenCalled();
  });

  it("keeps the row and shows neutral feedback after a failed delete", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response("backend detail", { status: 500 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.archiveFailure);
    expect(screen.getByRole("link", { name: conversation.title })).toBeInTheDocument();
    expect(screen.queryByText("backend detail")).not.toBeInTheDocument();
  });

  it("returns focus to the archive confirmation after a failed archive mutation", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 500 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    const archiveConfirm = screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel });
    const cancel = screen.getByRole("button", { name: ru.conversations.cancelLabel });
    cancel.focus();
    fireEvent.click(archiveConfirm);

    await screen.findByRole("alert");
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel })).toHaveFocus());
  });

  it("describes archive confirmation to its confirm control", () => {
    renderRow();
    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));

    const confirmation = screen.getByText(ru.conversations.archiveConfirmation);
    expect(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }).getAttribute("aria-describedby"))
      .toContain(confirmation.id);
  });

  it("reconciles a successful deferred rename after navigation changes without restoring stale focus", async () => {
    let pathname = "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906";
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    const rendered = render(
      <>
        <button type="button">Elsewhere</button>
        <ConversationRow conversation={conversation} isActive={false} />
      </>,
    );

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel }), { key: "Enter" });
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));

    pathname = "/app";
    rendered.rerender(
      <>
        <button type="button">Elsewhere</button>
        <ConversationRow conversation={conversation} isActive={false} />
      </>,
    );
    const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
    elsewhere.focus();
    settleRequest(Response.json(conversation, { status: 200 }));

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(replace).not.toHaveBeenCalled();
    expect(elsewhere).toHaveFocus();
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it("refreshes the current route when validated rename completion is followed by a pathname transition", async () => {
    let pathname = "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906";
    let rerenderRow: (ui: ReactNode) => void = () => {};
    let refreshedPathname: string | null = null;
    vi.mocked(usePathname).mockImplementation(() => pathname);
    refresh.mockImplementation(() => { refreshedPathname = pathname; });
    vi.mocked(webBrowserMutation).mockResolvedValue({
      json: () => new Promise((resolve) => {
        resolve(conversation);
        // Let the successful handler set its refresh/focus intent, then change
        // the pathname before React runs that intent's passive effect.
        queueMicrotask(() => queueMicrotask(() => {
          pathname = "/app";
          rerenderRow(<ConversationRow conversation={conversation} isActive={false} />);
        }));
      }),
      status: 200,
    } as Response);
    const rendered = renderRow();
    rerenderRow = rendered.rerender;
    const actions = screen.getByRole("button", { name: actionsLabel() });

    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel }), { key: "Enter" });

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(refreshedPathname).toBe("/app");
    expect(replace).not.toHaveBeenCalled();
  });

  it("reconciles a successful deferred archive after navigation changes without a stale replacement", async () => {
    let pathname = "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906";
    let settleRequest: (response: Response) => void = () => {};
    const onArchived = vi.fn();
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    const rendered = render(
      <>
        <button type="button">Elsewhere</button>
        <ConversationRow conversation={conversation} isActive onArchived={onArchived} />
      </>,
    );

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));

    pathname = "/app";
    rendered.rerender(
      <>
        <button type="button">Elsewhere</button>
        <ConversationRow conversation={conversation} isActive={false} onArchived={onArchived} />
      </>,
    );
    const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
    elsewhere.focus();
    settleRequest(new Response(null, { status: 204 }));

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(replace).not.toHaveBeenCalled();
    expect(onArchived).not.toHaveBeenCalled();
    expect(elsewhere).toHaveFocus();
    expect(screen.queryByText(ru.conversations.archiveConfirmation)).not.toBeInTheDocument();
  });

  it("ignores a settled archive after its row unmounts while pending", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));
    const rendered = renderRow(true);

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));

    rendered.unmount();
    settleRequest(new Response(null, { status: 204 }));

    await Promise.resolve();
    expect(refresh).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
  });

  it("keeps the row after a 200 delete response instead of treating it as archived", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 200 }));
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
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

    const actionButtons = screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) });
    fireEvent.click(actionButtons[0]);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));

    await vi.waitFor(() => expect(actionButtons[0]).toBeDisabled());
    expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled();
    expect(actionButtons[1]).toBeEnabled();

    settleRequest(new Response(null, { status: 500 }));
    await screen.findByRole("alert");
  });

  it("registers a pending panel in layout before its final Escape callback update", () => {
    const transitions: string[] = [];
    const onPendingPanelChange = vi.fn((_conversationId: string, isPending: boolean) => {
      transitions.push(`pending:${isPending}`);
    });
    const onVisiblePanelChange = vi.fn((_conversationId: string, closePanel: (() => void) | null) => {
      transitions.push(closePanel === null ? "visible:clear" : "visible:set");
    });
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>(() => {}));
    render(
      <ConversationRow
        conversation={conversation}
        isActive={false}
        onPendingPanelChange={onPendingPanelChange}
        onVisiblePanelChange={onVisiblePanelChange}
        sidebarSession={1}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: actionsLabel() }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    transitions.length = 0;

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));

    expect(onPendingPanelChange).toHaveBeenCalledWith(conversation.id, true, 1);
    expect(onVisiblePanelChange).toHaveBeenCalledWith(conversation.id, null, 1);
    expect(transitions.indexOf("pending:true")).toBeLessThan(transitions.lastIndexOf("visible:clear"));
  });

  it("names action toggles with the chat title or fallback", () => {
    const unnamedConversation = { ...conversation, title: " " };
    render(
      <>
        <ConversationRow conversation={conversation} isActive={false} />
        <ConversationRow conversation={unnamedConversation} isActive={false} />
      </>,
    );

    expect(screen.getByRole("button", { name: actionsLabel() })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: actionsLabel(unnamedConversation) })).toBeInTheDocument();
  });

  it("moves focus into each inner panel and restores it to actions after cancel or Escape", () => {
    renderRow();
    const actions = screen.getByRole("button", { name: actionsLabel() });

    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    expect(titleInput).toHaveFocus();

    fireEvent.keyDown(titleInput, { key: "Escape" });
    expect(actions).toHaveFocus();

    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    const archiveConfirm = screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel });
    expect(archiveConfirm).toHaveFocus();

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.cancelLabel }));
    expect(actions).toHaveFocus();
  });

  it("uses archive copy that explains removal from the visible chat lists", () => {
    expect(ru.conversations.archiveConfirmation).toContain("списка");
    expect(ru.conversations.archiveConfirmation).not.toContain("без возможности восстановления");
  });
});
