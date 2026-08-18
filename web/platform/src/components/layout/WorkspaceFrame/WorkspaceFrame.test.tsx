import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { usePathname } from "next/navigation";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/app"),
  useRouter: vi.fn(() => ({ push: vi.fn(), refresh: vi.fn(), replace: vi.fn() })),
}));

vi.mock("@/features/models/WorkspaceModelSelector/WorkspaceModelSelector", () => ({
  WorkspaceModelSelector: () => <button type="button">Model selector</button>,
}));

import { ru } from "@/i18n/ru";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { useWorkspaceDataCache, type WorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import type { AccountProfile, ConversationItem } from "@/lib/web-api/contracts";

import { GuestWorkspaceFrame, WorkspaceFrame } from "./WorkspaceFrame";

const storageKey = "neirohub.desktop-sidebar-collapsed";
const workspaceProfile: AccountProfile = {
  account_id: "0ce06a6a-16d8-4b16-b9df-5e63175a4a0c",
  identity_refs: [],
};
const workspaceProps: { accountId: string; conversations: ConversationItem[]; profile: AccountProfile } = {
  accountId: "0ce06a6a-16d8-4b16-b9df-5e63175a4a0c",
  conversations: [],
  profile: workspaceProfile,
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

function WorkspaceAccountProbe() {
  const { balance, profile } = useWorkspaceAccountSnapshot();

  return <output>{`${profile.account_id}:${balance}`}</output>;
}

function WorkspaceDataCacheProbe({ onCache }: { onCache: (cache: WorkspaceDataCache) => void }) {
  onCache(useWorkspaceDataCache());

  return null;
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

  it("provides the already loaded account snapshot to workspace children", () => {
    mockWideViewport();

    render(
      <WorkspaceFrame {...workspaceProps} balance={42}>
        <WorkspaceAccountProbe />
      </WorkspaceFrame>,
    );

    expect(screen.getByRole("status")).toHaveTextContent(`${workspaceProfile.account_id}:42`);
  });

  it("recreates the workspace cache when the account changes", () => {
    const observedCaches: WorkspaceDataCache[] = [];
    const onCache = (cache: WorkspaceDataCache) => observedCaches.push(cache);
    mockWideViewport();
    const rendered = render(
      <WorkspaceFrame {...workspaceProps}>
        <WorkspaceDataCacheProbe onCache={onCache} />
      </WorkspaceFrame>,
    );
    const firstCache = observedCaches[0];
    const firstPage = { items: [], has_more: false, next_cursor: null };

    firstCache.setImageFilesFirstPage(firstPage);
    rendered.rerender(
      <WorkspaceFrame {...workspaceProps} accountId="c0d90c5e-9da6-45d6-9868-ffbf25b48d4d">
        <WorkspaceDataCacheProbe onCache={onCache} />
      </WorkspaceFrame>,
    );

    const secondCache = observedCaches[1];
    expect(secondCache).not.toBe(firstCache);
    expect(secondCache.getImageFilesFirstPage()).toBeUndefined();
  });

  it.each([
    ["/app", ru.navigation.workspace],
    ["/app/chat/f9712bca-8d98-448d-b595-2a80bc9c2b1a", ru.navigation.workspace],
    ["/app/chats", ru.navigation.chats],
    ["/app/files", ru.navigation.files],
    ["/app/models", ru.navigation.models],
    ["/app/inspiration", ru.navigation.inspiration],
    ["/app/image", ru.navigation.workspace],
    ["/app/profile", ru.navigation.profile],
  ])("keeps the persistent header route-based for %s", (pathname, expectedTitle) => {
    vi.mocked(usePathname).mockReturnValue(pathname);
    mockWideViewport();

    render(
      <WorkspaceFrame {...workspaceProps} balance={42}>
        Workspace
      </WorkspaceFrame>,
    );

    const header = screen.getByTestId("workspace-header");
    expect(header).toHaveAccessibleName(expectedTitle);
    if (pathname === "/app/inspiration") {
      expect(header).toHaveTextContent(expectedTitle);
      expect(screen.queryByRole("button", { name: "Model selector" })).toBeNull();
    } else {
      expect(screen.getByRole("button", { name: "Model selector" })).toBeInTheDocument();
    }
    expect(header).toHaveTextContent("42 ★");
    expect(screen.queryByRole("button", { name: "Выбрать тариф" })).toBeNull();
  });

  it("shows a neutral balance state instead of a made-up zero while the value is unavailable", () => {
    vi.mocked(usePathname).mockReturnValue("/app");
    mockWideViewport();

    render(<WorkspaceFrame {...workspaceProps}>Workspace</WorkspaceFrame>);

    expect(screen.getByTestId("workspace-balance")).toHaveAttribute("aria-busy", "true");
    expect(screen.getByTestId("workspace-balance")).toHaveAttribute("aria-label", "Загружаем баланс…");
    expect(screen.getByTestId("workspace-balance")).not.toHaveTextContent("0");
  });

  it("composes a guest shell with two login actions and no private workspace data", () => {
    mockWideViewport();

    render(
      <GuestWorkspaceFrame>
        <p>Guest landing</p>
      </GuestWorkspaceFrame>,
    );

    const loginActions = screen.getAllByRole("link", { name: ru.login.submitLabel });
    expect(loginActions).toHaveLength(2);
    loginActions.forEach((link) => expect(link).toHaveAttribute("href", "/login"));
    expect(screen.getByText("Guest landing")).toBeInTheDocument();
    expect(screen.queryByTestId("workspace-balance")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: ru.conversations.recentHeading })).not.toBeInTheDocument();
  });
});
