import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("./workspace-logout-request", () => ({
  requestWorkspaceLogout: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { savePendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
import { savePendingConversationTitleSync } from "@/features/conversations/pending-conversation-title-sync";

import {
  useWorkspaceLogout,
  WorkspaceLogoutBoundary,
} from "./WorkspaceLogoutBoundary";
import { requestWorkspaceLogout } from "./workspace-logout-request";

const push = vi.fn();
const refresh = vi.fn();
const replace = vi.fn();

class FakeBroadcastChannel {
  static instances: FakeBroadcastChannel[] = [];

  readonly close = vi.fn();
  readonly name: string;
  readonly postMessage = vi.fn();
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(name: string) {
    this.name = name;
    FakeBroadcastChannel.instances.push(this);
  }

  emit(data: unknown) {
    this.onmessage?.({ data } as MessageEvent);
  }
}

function LogoutTrigger() {
  const { logout } = useWorkspaceLogout();

  return <button onClick={logout} type="button">Logout now</button>;
}

function LoginIntentTrigger() {
  const { requestLogin } = useWorkspaceLogout();

  return <button onClick={requestLogin} type="button">Login now</button>;
}

function renderBoundary() {
  return render(
    <WorkspaceLogoutBoundary
      guest={(
        <div>
          <p>Guest workspace</p>
          <LoginIntentTrigger />
        </div>
      )}
    >
      <div>
        <p>Private conversation</p>
        <LogoutTrigger />
      </div>
    </WorkspaceLogoutBoundary>,
  );
}

describe("WorkspaceLogoutBoundary", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ push, refresh, replace } as never);
    vi.stubGlobal("BroadcastChannel", FakeBroadcastChannel);
    FakeBroadcastChannel.instances = [];
  });

  afterEach(() => {
    cleanup();
    window.sessionStorage.clear();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("removes the private subtree synchronously while logout is still pending", async () => {
    let resolveLogout!: () => void;
    vi.mocked(requestWorkspaceLogout).mockReturnValue(new Promise((resolve) => {
      resolveLogout = resolve;
    }));
    savePendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906", "Private prompt");
    savePendingConversationTitleSync("d7c979f5-24e5-4f88-924b-a592d6e5a906", "Private title");
    renderBoundary();

    fireEvent.click(screen.getByRole("button", { name: "Logout now" }));

    expect(screen.queryByText("Private conversation")).not.toBeInTheDocument();
    expect(screen.getByText("Guest workspace")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-logout-transition")).toBeInTheDocument();
    expect(window.sessionStorage.length).toBe(0);
    expect(replace).not.toHaveBeenCalled();

    resolveLogout();
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/app"));
    expect(refresh).toHaveBeenCalledOnce();
  });

  it("keeps private content locked after failure and recovers through the explicit retry", async () => {
    vi.mocked(requestWorkspaceLogout)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(undefined);
    renderBoundary();

    fireEvent.click(screen.getByRole("button", { name: "Logout now" }));

    expect(await screen.findByRole("status")).toHaveTextContent(ru.account.logoutServerFailure);
    expect(screen.queryByText("Private conversation")).not.toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: ru.account.logoutRetryLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/app"));
    expect(requestWorkspaceLogout).toHaveBeenCalledTimes(2);
  });

  it("waits for logout confirmation before honoring a login intent", async () => {
    let resolveLogout!: () => void;
    vi.mocked(requestWorkspaceLogout).mockReturnValue(new Promise((resolve) => {
      resolveLogout = resolve;
    }));
    renderBoundary();

    fireEvent.click(screen.getByRole("button", { name: "Logout now" }));
    fireEvent.click(screen.getByRole("button", { name: "Login now" }));

    expect(replace).not.toHaveBeenCalled();

    resolveLogout();
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
    expect(refresh).toHaveBeenCalledOnce();
  });

  it("mirrors logout state received from another browser tab", () => {
    vi.mocked(requestWorkspaceLogout).mockResolvedValue(undefined);
    renderBoundary();
    const channel = FakeBroadcastChannel.instances[0];

    act(() => channel.emit({ type: "logout-started" }));
    expect(screen.queryByText("Private conversation")).not.toBeInTheDocument();
    expect(screen.getByText("Guest workspace")).toBeInTheDocument();
    expect(requestWorkspaceLogout).not.toHaveBeenCalled();

    act(() => channel.emit({ type: "logout-failed" }));
    expect(screen.getByRole("status")).toHaveTextContent(ru.account.logoutServerFailure);

    act(() => channel.emit({ type: "logout-confirmed" }));
    expect(replace).toHaveBeenCalledWith("/app");
    expect(refresh).toHaveBeenCalledOnce();
  });
});

