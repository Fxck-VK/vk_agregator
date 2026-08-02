import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/app"),
  useRouter: vi.fn(() => ({ push: vi.fn(), refresh: vi.fn(), replace: vi.fn() })),
}));

import { ru } from "@/i18n/ru";
import { useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { WorkspaceFrame } from "./WorkspaceFrame";

const storageKey = "neirohub.desktop-sidebar-collapsed";
const workspaceProps: { accountId: string; conversations: ConversationItem[] } = {
  accountId: "0ce06a6a-16d8-4b16-b9df-5e63175a4a0c",
  conversations: [],
};
const workspaceConversation: ConversationItem = {
  id: "f9712bca-8d98-448d-b595-2a80bc9c2b1a",
  title: "Visible to workspace children",
  created_at: "2026-08-02T09:00:00Z",
  updated_at: "2026-08-02T09:00:00Z",
};

function WorkspaceConversationListProbe() {
  const { conversations } = useWorkspaceConversationList();

  return <output>{conversations.map((conversation) => conversation.title).join(",")}</output>;
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

describe("WorkspaceFrame", () => {
  afterEach(() => {
    cleanup();
    localStorage.clear();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("hydrates the open server markup, then restores only a saved collapsed preference without a hydration error or storage write", async () => {
    localStorage.setItem(storageKey, "true");
    const setItem = vi.spyOn(Storage.prototype, "setItem").mockClear();
    const serverMarkup = renderToString(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);
    const container = document.createElement("div");
    const onRecoverableError = vi.fn();

    container.innerHTML = serverMarkup;

    expect(serverMarkup).toContain(
      'data-desktop-sidebar-collapsed="false"',
    );
    expect(container.firstElementChild).toHaveAttribute("data-desktop-sidebar-collapsed", "false");

    mockWideViewport();
    const root = hydrateRoot(container, <WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>, { onRecoverableError });

    await waitFor(() => {
      expect(container.firstElementChild).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
    });
    expect(onRecoverableError).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();

    root.unmount();
  });

  it("persists the desktop preference only when its sidebar control is toggled", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    mockWideViewport();

    render(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);

    fireEvent.click(screen.getByRole("button", { name: ru.navigation.collapseSidebarLabel }));

    expect(screen.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
    expect(setItem).toHaveBeenCalledExactlyOnceWith(storageKey, "true");
  });

  it("uses the open preference when browser storage cannot be read", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("Storage blocked", "SecurityError");
    });
    mockWideViewport();

    render(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);

    expect(screen.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "false");
  });

  it("keeps the visible toggle state responsive when browser storage cannot be written", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("Storage blocked", "QuotaExceededError");
    });
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    mockWideViewport();

    render(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);

    expect(() => {
      fireEvent.click(screen.getByRole("button", { name: ru.navigation.collapseSidebarLabel }));
    }).not.toThrow();
    expect(screen.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
    expect(consoleError).not.toHaveBeenCalled();
  });

  it("writes false when an explicit restore follows a collapse", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    mockWideViewport();

    render(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);

    fireEvent.click(screen.getByRole("button", { name: ru.navigation.collapseSidebarLabel }));
    fireEvent.click(screen.getByRole("button", { name: ru.navigation.expandSidebarLabel }));

    expect(setItem).toHaveBeenNthCalledWith(1, storageKey, "true");
    expect(setItem).toHaveBeenNthCalledWith(2, storageKey, "false");
    expect(screen.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "false");
  });

  it("provides the visible conversation list to workspace children", () => {
    mockWideViewport();

    render(
      <WorkspaceFrame {...workspaceProps} conversations={[workspaceConversation]}>
        <WorkspaceConversationListProbe />
      </WorkspaceFrame>,
    );

    expect(screen.getByRole("status")).toHaveTextContent(workspaceConversation.title);
  });
});
