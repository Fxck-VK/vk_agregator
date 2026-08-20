import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const logout = vi.hoisted(() => vi.fn());

vi.mock("@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary", () => ({
  useWorkspaceLogout: () => ({ logout }),
}));

import type { AccountProfile } from "@/lib/web-api/contracts";
import { ru } from "@/i18n/ru";

import { AccountControl } from "./AccountControl";

const profile: AccountProfile = {
  account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
  identity_refs: [
    {
      id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
      account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
      provider: "email",
      label: "  member@example.com  ",
      verified: true,
      created_at: "2026-07-31T09:00:00Z",
    },
  ],
};

describe("AccountControl", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("opens a compact account menu with a profile route and placeholder actions", () => {
    const { container } = render(<AccountControl profile={profile} />);
    const trigger = screen.getByRole("button", { name: "Открыть меню аккаунта" });

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveAttribute("data-sidebar-account-trigger", "true");
    expect(trigger).toHaveAttribute("data-sidebar-tooltip", ru.account.profileLabel);
    expect(screen.getByText("member@example.com")).toBeInTheDocument();
    expect(screen.queryByText("ME")).not.toBeInTheDocument();
    expect(screen.queryByText(profile.account_id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].provider)).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    const avatar = container.querySelector('[data-account-avatar="true"]');
    expect(avatar).toHaveTextContent("NH");
    expect(avatar).toHaveAttribute("aria-hidden", "true");

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const menu = screen.getByRole("region", { name: "Меню аккаунта" });
    expect(menu).toHaveFocus();
    const profileAction = screen.getByRole("link", { name: "Профиль" });
    const supportAction = screen.getByRole("button", { name: "Поддержка" });
    const updatesAction = screen.getByRole("button", { name: "Что нового?" });

    expect(profileAction).toHaveAttribute("href", "/app/profile");
    expect(profileAction.querySelector('[data-icon="profile"]')).toBeInTheDocument();
    expect(supportAction).toBeDisabled();
    expect(supportAction.querySelector('[data-icon="support"]')).toBeInTheDocument();
    expect(updatesAction).toBeDisabled();
    expect(updatesAction.querySelector('[data-icon="megaphone"]')).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Системная тема" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Светлая тема" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: "Тёмная тема" })).toHaveAttribute("aria-pressed", "false");
    expect(container.querySelectorAll("a")).toHaveLength(1);
  });

  it("uses a generic unavailable label when no verified safe label exists", () => {
    const unavailableProfile: AccountProfile = {
      ...profile,
      identity_refs: [{ ...profile.identity_refs[0], verified: false }],
    };
    render(<AccountControl profile={unavailableProfile} />);

    expect(screen.getByText(ru.account.unavailableLabel)).toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].label.trim())).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("switches and persists the selected appearance without closing the menu", () => {
    render(<AccountControl profile={profile} />);
    fireEvent.click(screen.getByRole("button", { name: ru.account.openMenuLabel }));

    const systemTheme = screen.getByRole("button", { name: ru.account.systemThemeLabel });
    const lightTheme = screen.getByRole("button", { name: ru.account.lightThemeLabel });
    const darkTheme = screen.getByRole("button", { name: ru.account.darkThemeLabel });

    expect(systemTheme).toBeEnabled();
    expect(lightTheme).toBeEnabled();
    expect(darkTheme).toBeEnabled();

    fireEvent.click(lightTheme);

    expect(lightTheme).toHaveAttribute("aria-pressed", "true");
    expect(systemTheme).toHaveAttribute("aria-pressed", "false");
    expect(darkTheme).toHaveAttribute("aria-pressed", "false");
    expect(document.documentElement).toHaveAttribute("data-theme", "light");
    expect(localStorage.getItem("neirohub.theme")).toBe("light");
    expect(screen.getByRole("region", { name: ru.account.menuLabel })).toBeInTheDocument();
  });

  it("renders the approved shared artwork for every theme option", () => {
    render(<AccountControl profile={profile} />);
    fireEvent.click(screen.getByRole("button", { name: ru.account.openMenuLabel }));

    const systemTheme = screen.getByRole("button", { name: ru.account.systemThemeLabel });
    const lightTheme = screen.getByRole("button", { name: ru.account.lightThemeLabel });
    const darkTheme = screen.getByRole("button", { name: ru.account.darkThemeLabel });

    expect(systemTheme.querySelector('[data-icon="monitor"]')).toBeInTheDocument();
    expect(lightTheme.querySelector('[data-icon="sun"]')).toBeInTheDocument();
    expect(darkTheme.querySelector('[data-icon="moon"]')).toBeInTheDocument();
  });

  it("delegates logout to the workspace session boundary", () => {
    render(<AccountControl profile={profile} />);

    fireEvent.click(screen.getByRole("button", { name: "Открыть меню аккаунта" }));
    fireEvent.click(screen.getByRole("button", { name: ru.account.logoutLabel }));

    expect(logout).toHaveBeenCalledOnce();
  });

  it("closes the account menu with Escape and returns focus to the account trigger", () => {
    render(<AccountControl profile={profile} />);
    const trigger = screen.getByRole("button", { name: "Открыть меню аккаунта" });

    fireEvent.click(trigger);
    fireEvent.keyDown(document.activeElement as HTMLElement, { key: "Escape" });

    expect(screen.queryByRole("region", { name: "Меню аккаунта" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
});
