import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import Link from "next/link";
import { renderToStaticMarkup } from "react-dom/server";
import type { ReactNode } from "react";
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
import { SidebarConversations } from "@/features/conversations/SidebarConversations/SidebarConversations";
import { webBrowserMutation } from "@/lib/web-api/browser";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { Sidebar } from "./Sidebar";
import sidebarStyles from "./Sidebar.module.css";

const recentConversations: ConversationItem[] = Array.from({ length: 20 }, (_, index) => ({
  id: `d7c979f5-24e5-4f88-924b-a592d6e5a${String(index).padStart(3, "0")}`,
  title: `Recent chat ${index + 1}`,
  created_at: "2026-07-31T09:00:00Z",
  updated_at: "2026-07-31T09:05:00Z",
}));

function mockNarrowViewport() {
  vi.stubGlobal("matchMedia", (query: string): MediaQueryList => ({
    matches: query === "(max-width: 47.99rem)",
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
  }));
}

function mockWideViewport() {
  vi.stubGlobal("matchMedia", (query: string): MediaQueryList => ({
    matches: true,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
  }));
}

function mockResponsiveViewport(initialDesktop: boolean) {
  let isDesktop = initialDesktop;
  const listeners = new Set<(event: MediaQueryListEvent) => void>();
  vi.stubGlobal("matchMedia", (): MediaQueryList => ({
    get matches() { return isDesktop; },
    media: "(min-width: 48rem)",
    onchange: null,
    addEventListener: (_type: string, listener: EventListenerOrEventListenerObject) => { listeners.add(listener as (event: MediaQueryListEvent) => void); },
    removeEventListener: (_type: string, listener: EventListenerOrEventListenerObject) => { listeners.delete(listener as (event: MediaQueryListEvent) => void); },
    dispatchEvent: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
  }));

  return {
    setDesktop(nextIsDesktop: boolean) {
      isDesktop = nextIsDesktop;
      for (const listener of listeners) listener({ matches: isDesktop } as MediaQueryListEvent);
    },
  };
}

function renderNarrowSidebar({
  account,
  conversations,
}: {
  account?: ReactNode;
  conversations?: ReactNode;
} = {}) {
  mockNarrowViewport();
  render(<Sidebar account={account} conversations={conversations} />);

  const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
  const panel = screen.getByTestId("sidebar-panel");

  return { panel, trigger };
}

function openNavigation(trigger: HTMLElement) {
  fireEvent.click(trigger);

  return screen.getByRole("link", { name: ru.navigation.workspace });
}

describe("Sidebar", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("keeps the narrow drawer closed and inaccessible until its trigger opens it", () => {
    const serverMarkup = renderToStaticMarkup(
      <Sidebar
        account={<button type="button">Выйти</button>}
        conversations={<Link href="/app/chat/recent">Недавний чат</Link>}
      />,
    );
    const { panel, trigger } = renderNarrowSidebar({
      account: <button type="button">Выйти</button>,
      conversations: <Link href="/app/chat/recent">Недавний чат</Link>,
    });

    expect(serverMarkup).toContain('data-open="false"');
    expect(serverMarkup).toContain('aria-expanded="false"');
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(panel).toHaveAttribute("data-open", "false");
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(panel).toHaveAttribute("inert");
    expect(screen.queryByRole("link", { name: ru.navigation.workspace })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Выйти" })).not.toBeInTheDocument();
    expect(screen.queryByText(ru.accountPreview.title)).not.toBeInTheDocument();
  });

  it("toggles the narrow drawer and restores focus to its trigger when closed", () => {
    const { panel, trigger } = renderNarrowSidebar();

    const firstLink = openNavigation(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(panel).toHaveAttribute("data-open", "true");
    expect(firstLink).toHaveFocus();
    expect(screen.getByRole("dialog", { name: ru.navigation.label })).toHaveAttribute("aria-modal", "true");

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveFocus();
  });

  it("keeps Tab focus in the opened drawer and restores it after Escape", () => {
    const { panel, trigger } = renderNarrowSidebar({
      account: <button type="button">Выйти</button>,
      conversations: <Link href="/app/chat/recent">Недавний чат</Link>,
    });
    const firstLink = openNavigation(trigger);
    const lastControl = screen.getByRole("button", { name: "Выйти" });

    expect(panel).toContainElement(screen.getByRole("link", { name: "Недавний чат" }));
    expect(panel).toContainElement(lastControl);

    fireEvent.keyDown(firstLink, { key: "Tab", shiftKey: true });
    expect(lastControl).toHaveFocus();

    fireEvent.keyDown(lastControl, { key: "Tab" });
    expect(firstLink).toHaveFocus();

    fireEvent.keyDown(window, { key: "Escape" });

    expect(trigger).toHaveFocus();
    expect(screen.queryByRole("link", { name: ru.navigation.workspace })).not.toBeInTheDocument();
  });

  it("closes the narrow drawer after a recent chat selection without restoring trigger focus", () => {
    const { panel, trigger } = renderNarrowSidebar({
      conversations: <Link href="/app/chat/recent">Recent chat</Link>,
    });

    openNavigation(trigger);
    const recentChat = screen.getByRole("link", { name: "Recent chat" });
    recentChat.addEventListener("click", (event) => event.preventDefault(), { once: true });

    fireEvent.click(recentChat);

    expect(panel).toHaveAttribute("data-open", "false");
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(panel).toHaveAttribute("inert");
    expect(trigger).not.toHaveFocus();
  });

  it("does not close the narrow drawer when an action inside the conversations slot is clicked", () => {
    const onAction = vi.fn();
    const { panel, trigger } = renderNarrowSidebar({
      conversations: <button onClick={onAction} type="button">Chat action</button>,
    });

    openNavigation(trigger);
    fireEvent.click(screen.getByRole("button", { name: "Chat action" }));

    expect(onAction).toHaveBeenCalledTimes(1);
    expect(panel).toHaveAttribute("data-open", "true");
    expect(trigger).not.toHaveFocus();
  });

  it("closes the narrow drawer after creating a chat changes the pathname without restoring trigger focus", async () => {
    mockNarrowViewport();
    let pathname = "/app/chat/current";
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn().mockReturnValue("test-request-1"),
    });
    vi.mocked(webBrowserMutation).mockResolvedValue(
      Response.json(
        {
          id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
          title: "",
          created_at: "2026-07-31T09:00:00Z",
          updated_at: "2026-07-31T09:05:00Z",
        },
        { status: 201 },
      ),
    );

    let rerenderSidebar: (ui: ReactNode) => void = () => {};
    const push = vi.fn((nextPath: string) => {
      pathname = nextPath;
      rerenderSidebar(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    });
    vi.mocked(useRouter).mockReturnValue({ push, refresh: vi.fn() } as never);

    const rendered = render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    rerenderSidebar = rendered.rerender;
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.createLabel }));

    await vi.waitFor(() => expect(panel).toHaveAttribute("data-open", "false"));
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(panel).toHaveAttribute("inert");
    expect(trigger).not.toHaveFocus();
    expect(push).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906");
  });

  it("closes the narrow drawer after deleting the active chat changes the pathname without restoring trigger focus", async () => {
    mockNarrowViewport();
    let pathname = "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a000";
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));

    let conversations = recentConversations;
    let rerenderSidebar: (ui: ReactNode) => void = () => {};
    const refresh = vi.fn(() => {
      conversations = conversations.slice(1);
      rerenderSidebar(<Sidebar conversations={<SidebarConversations conversations={conversations} />} />);
    });
    const replace = vi.fn((nextPath: string) => {
      pathname = nextPath;
      rerenderSidebar(<Sidebar conversations={<SidebarConversations conversations={conversations} />} />);
    });
    vi.mocked(useRouter).mockReturnValue({ refresh, replace } as never);

    const rendered = render(<Sidebar conversations={<SidebarConversations conversations={conversations} />} />);
    rerenderSidebar = rendered.rerender;
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);

    fireEvent.click(screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) })[0]);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    await vi.waitFor(() => expect(panel).toHaveAttribute("data-open", "false"));
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(panel).toHaveAttribute("inert");
    expect(trigger).not.toHaveFocus();
    expect(replace).toHaveBeenCalledWith("/app");
    expect(refresh).not.toHaveBeenCalled();
    expect(screen.queryByRole("link", { name: "Recent chat 1" })).not.toBeInTheDocument();
    expect(panel.contains(document.activeElement)).toBe(false);

    fireEvent.click(trigger);
    expect(screen.queryByRole("link", { name: "Recent chat 1" })).not.toBeInTheDocument();
  });

  it("closes only an open conversation panel on Escape inside the narrow drawer", () => {
    const { panel, trigger } = renderNarrowSidebar({
      conversations: <SidebarConversations conversations={recentConversations} />,
    });

    openNavigation(trigger);
    const actions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });

    fireEvent.keyDown(titleInput, { key: "Escape" });

    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
    expect(panel).toHaveAttribute("data-open", "true");
    expect(actions).toHaveFocus();
    expect(trigger).not.toHaveFocus();
  });

  it("clears nested conversation panels while the narrow drawer is inactive", () => {
    const { trigger } = renderNarrowSidebar({
      conversations: <SidebarConversations conversations={recentConversations} />,
    });
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    expect(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel })).toBeInTheDocument();

    fireEvent.click(trigger);
    fireEvent.click(trigger);

    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it("invalidates row panels when the desktop sidebar becomes narrow and then reactivates", () => {
    const viewport = mockResponsiveViewport(true);
    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const actions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    fireEvent.click(actions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    expect(screen.getByRole("textbox", { name: ru.conversations.renameInputLabel })).toBeInTheDocument();

    act(() => viewport.setDesktop(false));
    expect(screen.getByTestId("sidebar-panel")).toHaveAttribute("data-open", "false");

    act(() => viewport.setDesktop(true));
    expect(screen.getByTestId("sidebar-panel")).toHaveAttribute("data-open", "true");
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it.each(["trigger", "backdrop", "navigation"] as const)("invalidates row panels after the narrow drawer closes through %s", (closeMethod) => {
    const { trigger } = renderNarrowSidebar({
      conversations: <SidebarConversations conversations={recentConversations} />,
    });
    openNavigation(trigger);
    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));

    if (closeMethod === "trigger") fireEvent.click(trigger);
    else if (closeMethod === "backdrop") fireEvent.click(screen.getByRole("button", { name: ru.navigation.closeMenuLabel }));
    else {
      const workspace = screen.getByRole("link", { name: ru.navigation.workspace });
      workspace.addEventListener("click", (event) => event.preventDefault(), { once: true });
      fireEvent.click(workspace);
    }

    fireEvent.click(trigger);
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it("invalidates row panels when the desktop sidebar collapses and expands", () => {
    mockWideViewport();
    const onDesktopToggle = vi.fn();
    const rendered = render(
      <Sidebar conversations={<SidebarConversations conversations={recentConversations} />} isDesktopCollapsed={false} onDesktopToggle={onDesktopToggle} />,
    );
    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));

    rendered.rerender(
      <Sidebar conversations={<SidebarConversations conversations={recentConversations} />} isDesktopCollapsed onDesktopToggle={onDesktopToggle} />,
    );
    rendered.rerender(
      <Sidebar conversations={<SidebarConversations conversations={recentConversations} />} isDesktopCollapsed={false} onDesktopToggle={onDesktopToggle} />,
    );

    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
  });

  it("does not revive or refocus a pending mutation from a closed drawer session", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());

    fireEvent.click(trigger);
    fireEvent.click(trigger);
    const focusBeforeSettlement = document.activeElement;
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();

    settleRequest(new Response(null, { status: 500 }));

    await Promise.resolve();
    await Promise.resolve();
    expect(screen.queryByRole("textbox", { name: ru.conversations.renameInputLabel })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(focusBeforeSettlement);

    fireEvent.keyDown(focusBeforeSettlement as HTMLElement, { key: "Escape" });
    expect(panel).toHaveAttribute("data-open", "false");
  });

  it("keeps the drawer open when Escape is pressed in a visible pending row", async () => {
    mockNarrowViewport();
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>(() => {}));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);
    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    const titleInput = screen.getByRole("textbox", { name: ru.conversations.renameInputLabel });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());

    fireEvent.keyDown(titleInput, { key: "Escape" });
    expect(panel).toHaveAttribute("data-open", "true");
  });

  it("keeps the drawer open when Escape is pressed on another control while a row is pending", async () => {
    mockNarrowViewport();
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>(() => {}));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    const workspace = openNavigation(trigger);
    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());

    workspace.focus();
    fireEvent.keyDown(workspace, { key: "Escape" });
    expect(panel).toHaveAttribute("data-open", "true");
  });

  it("keeps the drawer open for pending A after pending B settles", async () => {
    mockNarrowViewport();
    let settleA: (response: Response) => void = () => {};
    let settleB: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation)
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleA = resolve; }))
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleB = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    const workspace = openNavigation(trigger);
    const [firstActions, secondActions] = screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());
    fireEvent.click(secondActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));

    settleB(Response.json(recentConversations[1], { status: 200 }));
    await vi.waitFor(() => expect(screen.getAllByRole("button", { name: ru.conversations.renamePending })).toHaveLength(1));
    workspace.focus();
    fireEvent.keyDown(workspace, { key: "Escape" });
    expect(panel).toHaveAttribute("data-open", "true");
    settleA(new Response(null, { status: 500 }));
  });

  it("does not refocus the first row when a second row owns the panel after a rename settles", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    const refresh = vi.fn();
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh, replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    const firstActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    const secondActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 2` });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());
    fireEvent.click(secondActions);
    secondActions.focus();

    settleRequest(Response.json(recentConversations[0], { status: 200 }));

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(document.activeElement).toBe(secondActions);
  });

  it("keeps the second row panel and focus when a background first-row rename fails", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    const firstActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    const secondActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 2` });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());
    fireEvent.click(secondActions);
    secondActions.focus();

    settleRequest(new Response(null, { status: 500 }));

    await vi.waitFor(() => expect(screen.queryByRole("button", { name: ru.conversations.renamePending })).not.toBeInTheDocument());
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(document.activeElement).toBe(secondActions);
  });

  it("preserves the second row panel when a first-row archive settles in the background", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    const firstActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    const secondActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 2` });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.archivePending })).toBeDisabled());
    fireEvent.click(secondActions);
    secondActions.focus();

    settleRequest(new Response(null, { status: 204 }));

    await vi.waitFor(() => expect(screen.queryByRole("link", { name: "Recent chat 1" })).not.toBeInTheDocument());
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(document.activeElement).toBe(secondActions);
  });

  it("focuses create chat after two deferred archives remove their stale successor", async () => {
    mockNarrowViewport();
    let settleA: (response: Response) => void = () => {};
    let settleB: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation)
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleA = resolve; }))
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleB = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations.slice(0, 2)} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    const [firstActions, secondActions] = screen.getAllByRole("button", { name: new RegExp(ru.conversations.actionsLabel) });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    fireEvent.click(secondActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));

    settleA(new Response(null, { status: 204 }));
    await vi.waitFor(() => expect(screen.queryByRole("link", { name: "Recent chat 1" })).not.toBeInTheDocument());
    settleB(new Response(null, { status: 204 }));

    await vi.waitFor(() => expect(screen.queryByRole("link", { name: "Recent chat 2" })).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: ru.conversations.createLabel })).toHaveFocus();
  });

  it("keeps the second row panel and focus when a background first-row archive fails", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    const firstActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    const secondActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 2` });
    fireEvent.click(firstActions);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.archivePending })).toBeDisabled());
    fireEvent.click(secondActions);
    secondActions.focus();

    settleRequest(new Response(null, { status: 500 }));

    await vi.waitFor(() => expect(screen.queryByRole("button", { name: ru.conversations.archivePending })).not.toBeInTheDocument());
    expect(secondActions).toHaveAttribute("aria-expanded", "true");
    expect(document.activeElement).toBe(secondActions);
  });

  it("dismisses a non-pending row panel on an outside click without stealing the target focus", () => {
    const { trigger } = renderNarrowSidebar({
      conversations: <SidebarConversations conversations={recentConversations} />,
    });
    const workspace = openNavigation(trigger);
    const actions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    fireEvent.click(actions);
    expect(actions).toHaveAttribute("aria-expanded", "true");

    fireEvent.pointerDown(workspace);
    workspace.focus();

    expect(actions).toHaveAttribute("aria-expanded", "false");
    expect(workspace).toHaveFocus();
  });

  it("does not refocus an old drawer session after a successful pending rename", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    const refresh = vi.fn();
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh, replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    openNavigation(trigger);
    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.renameSubmitLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.renamePending })).toBeDisabled());

    fireEvent.click(trigger);
    fireEvent.click(trigger);
    const focusBeforeSettlement = document.activeElement;
    settleRequest(Response.json(recentConversations[0], { status: 200 }));

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(document.activeElement).toBe(focusBeforeSettlement);
  });

  it("does not focus a successor when a pending archive settles after the drawer closes", async () => {
    mockNarrowViewport();
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>((resolve) => { settleRequest = resolve; }));

    render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.archiveConfirmLabel }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.archivePending })).toBeDisabled());

    fireEvent.click(trigger);
    const focusBeforeSettlement = document.activeElement;
    settleRequest(new Response(null, { status: 204 }));

    await vi.waitFor(() => expect(document.getElementById("sidebar-conversation-d7c979f5-24e5-4f88-924b-a592d6e5a000")).not.toBeInTheDocument());
    expect(document.activeElement).toBe(focusBeforeSettlement);
    expect(panel.contains(document.activeElement)).toBe(false);
  });

  it("clears an unmounted conversation panel before the next Escape", () => {
    mockNarrowViewport();
    const rendered = render(<Sidebar conversations={<SidebarConversations conversations={recentConversations} />} />);
    const trigger = screen.getByRole("button", { name: ru.navigation.openMenuLabel });
    const panel = screen.getByTestId("sidebar-panel");
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` }));
    rendered.rerender(<Sidebar conversations={<SidebarConversations conversations={recentConversations.slice(1)} />} />);
    fireEvent.keyDown(window, { key: "Escape" });

    expect(panel).toHaveAttribute("data-open", "false");
    expect(trigger).toHaveFocus();
  });

  it("closes the first row panel when a second row panel opens", () => {
    const { panel, trigger } = renderNarrowSidebar({
      conversations: <SidebarConversations conversations={recentConversations} />,
    });
    openNavigation(trigger);
    const firstActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 1` });
    const secondActions = screen.getByRole("button", { name: `${ru.conversations.actionsLabel}: Recent chat 2` });

    fireEvent.click(firstActions);
    fireEvent.click(secondActions);
    fireEvent.keyDown(secondActions, { key: "Escape" });

    expect(panel).toHaveAttribute("data-open", "true");
    expect(firstActions).toHaveAttribute("aria-expanded", "false");
    expect(secondActions).toHaveAttribute("aria-expanded", "false");
    expect(trigger).not.toHaveFocus();
  });

  it("keeps twenty recent chat links reachable above a focusable account control", () => {
    mockWideViewport();
    render(
      <Sidebar
        account={<button type="button">{ru.account.logoutLabel}</button>}
        conversations={<SidebarConversations conversations={recentConversations} />}
      />,
    );

    const recentLinks = screen.getAllByRole("link", { name: /Recent chat/ });
    const finalRecentLink = screen.getByRole("link", { name: "Recent chat 20" });
    const logoutControl = screen.getByRole("button", { name: ru.account.logoutLabel });
    const conversationsSlot = screen.getByRole("heading", { name: ru.conversations.recentHeading }).closest("section")?.parentElement;

    expect(recentLinks).toHaveLength(20);
    expect(finalRecentLink).toHaveAttribute("href", "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a019");
    expect(logoutControl).toBeInTheDocument();
    logoutControl.focus();
    expect(logoutControl).toHaveFocus();
    expect(conversationsSlot).toHaveClass(sidebarStyles.conversationsSlot);
  });

  it("calls the desktop sidebar toggle from its dedicated control", () => {
    mockWideViewport();
    const onDesktopToggle = vi.fn();

    render(<Sidebar isDesktopCollapsed={false} onDesktopToggle={onDesktopToggle} />);

    const control = screen.getByRole("button", { name: ru.navigation.collapseSidebarLabel });

    expect(control).toHaveAttribute("aria-controls", "sidebar-panel");
    expect(control).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(control);

    expect(onDesktopToggle).toHaveBeenCalledTimes(1);
  });

  it("does not render a no-op desktop toggle when no toggle handler is supplied", () => {
    mockWideViewport();

    render(<Sidebar />);

    expect(screen.queryByRole("button", { name: ru.navigation.collapseSidebarLabel })).not.toBeInTheDocument();
  });

  it("derives narrow drawer behavior by inverting the exact desktop breakpoint query", () => {
    const matchMedia = vi.fn((query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
    }));
    vi.stubGlobal("matchMedia", matchMedia);

    render(<Sidebar />);

    expect(matchMedia).toHaveBeenCalledWith("(min-width: 48rem)");
  });

  it("makes a collapsed desktop panel inaccessible while keeping its restore control available", () => {
    mockWideViewport();
    const onDesktopToggle = vi.fn();

    render(<Sidebar isDesktopCollapsed onDesktopToggle={onDesktopToggle} />);

    expect(screen.getByRole("button", { name: ru.navigation.expandSidebarLabel })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("sidebar-panel")).toHaveAttribute("data-desktop-collapsed", "true");
    expect(screen.getByTestId("sidebar-panel")).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByTestId("sidebar-panel")).toHaveAttribute("inert");
  });

  it("keeps the final recent chat and account control in the narrow drawer focus trap", () => {
    const { panel, trigger } = renderNarrowSidebar({
      account: <button type="button">{ru.account.logoutLabel}</button>,
      conversations: <SidebarConversations conversations={recentConversations} />,
    });
    const firstLink = openNavigation(trigger);
    const finalRecentLink = screen.getByRole("link", { name: "Recent chat 20" });
    const logoutControl = screen.getByRole("button", { name: ru.account.logoutLabel });

    expect(panel).toContainElement(finalRecentLink);
    expect(panel).toContainElement(logoutControl);

    fireEvent.keyDown(firstLink, { key: "Tab", shiftKey: true });
    expect(logoutControl).toHaveFocus();

    fireEvent.keyDown(logoutControl, { key: "Tab" });
    expect(firstLink).toHaveFocus();
  });

  it("closes from the backdrop and navigation links", () => {
    const { panel, trigger } = renderNarrowSidebar();
    openNavigation(trigger);

    fireEvent.click(screen.getByRole("button", { name: ru.navigation.closeMenuLabel }));

    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveFocus();

    const firstLink = openNavigation(trigger);
    firstLink.addEventListener("click", (event) => event.preventDefault(), { once: true });
    fireEvent.click(firstLink);

    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveFocus();
    expect(screen.queryByRole("link", { name: ru.navigation.workspace })).not.toBeInTheDocument();
  });
});
