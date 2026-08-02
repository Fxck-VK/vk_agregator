import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { usePathname, useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { SidebarConversations } from "./SidebarConversations";

const conversations: ConversationItem[] = [
  {
    id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
    title: "Подготовить макет",
    created_at: "2026-07-31T09:00:00Z",
    updated_at: "2026-07-31T09:05:00Z",
  },
  {
    id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
    title: "   ",
    created_at: "2026-07-31T09:10:00Z",
    updated_at: "2026-07-31T09:15:00Z",
  },
];

describe("SidebarConversations", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("marks only the exact current conversation without rendering a duplicate create action", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("link", { name: conversations[0].title })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: ru.conversations.unnamed })).not.toHaveAttribute("aria-current");
    expect(screen.queryByRole("button", { name: ru.conversations.createLabel })).not.toBeInTheDocument();
  });

  it("does not mark a nested conversation route as active", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906/child");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("link", { name: conversations[0].title })).not.toHaveAttribute("aria-current");
  });

  it("renders safe conversation titles as local chat links and uses the unnamed fallback", () => {
    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("heading", { name: ru.conversations.recentHeading })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Подготовить макет" })).toHaveAttribute(
      "href",
      "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906",
    );
    expect(screen.getByRole("link", { name: ru.conversations.unnamed })).toHaveAttribute(
      "href",
      "/app/chat/a2a006fc-4457-4bb5-bc4d-4f553d51766b",
    );
    expect(screen.queryByText(conversations[0].created_at)).not.toBeInTheDocument();
    expect(screen.queryByText(conversations[0].updated_at)).not.toBeInTheDocument();
  });

  it("renders each recent chat through an independently labelled action control", () => {
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) })).toHaveLength(conversations.length);
  });

  it("keeps B focused while a background rename failure on A remains visible and restores A's draft when chosen", async () => {
    let settleRename: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRename = resolve; }));

    render(<SidebarConversations conversations={conversations} />);

    const [firstActions, secondActions] = screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    const draft = "Keep this draft";
    fireEvent.change(titleInput, { target: { value: draft } });
    fireEvent.keyDown(titleInput, { key: "Enter" });
    fireEvent.click(secondActions);
    secondActions.focus();

    settleRename(new Response(null, { status: 500 }));

    await vi.waitFor(() => expect(firstActions).toBeEnabled());
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.renameFailure);
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(secondActions).toHaveFocus();
    expect(firstActions).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(firstActions);

    expect(secondActions).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel })).toHaveValue(draft);
  });

  it("keeps B focused while a background archive failure on A restores its confirmation when chosen", async () => {
    let settleArchive: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleArchive = resolve; }));

    render(<SidebarConversations conversations={conversations} />);

    const [firstActions, secondActions] = screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.archivePending })).toBeDisabled());
    fireEvent.click(secondActions);
    secondActions.focus();

    settleArchive(new Response(null, { status: 500 }));

    await vi.waitFor(() => expect(firstActions).toBeEnabled());
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.archiveFailure);
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(secondActions).toHaveFocus();

    fireEvent.click(firstActions);

    expect(secondActions).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText(ru.conversations.archiveConfirmation)).toBeVisible();
    expect(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel })).toBeEnabled();
  });

  it("focuses the next chat after archiving a non-active row", async () => {
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));

    render(<SidebarConversations conversations={conversations} />);

    fireEvent.click(screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) })[0]);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    const successor = await screen.findByRole("link", { name: ru.conversations.unnamed });
    await vi.waitFor(() => expect(screen.queryByRole("link", { name: conversations[0].title })).not.toBeInTheDocument());
    expect(successor).toHaveFocus();
  });

  it("renders an explicit empty recent-chat state", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={[]} />);

    expect(screen.getByText(ru.conversations.empty)).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: ru.conversations.createLabel })).not.toBeInTheDocument();
  });
});
