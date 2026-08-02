import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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
