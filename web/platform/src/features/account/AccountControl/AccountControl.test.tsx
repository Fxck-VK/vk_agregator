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
import type { AccountProfile } from "@/lib/web-api/contracts";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { AccountControl } from "./AccountControl";

const replace = vi.fn();
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
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ replace } as never);
  });

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
    expect(screen.getByText("member@example.com")).toBeInTheDocument();
    expect(screen.queryByText("ME")).not.toBeInTheDocument();
    expect(screen.queryByText(profile.account_id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].provider)).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const menu = screen.getByRole("region", { name: "Меню аккаунта" });
    expect(menu).toHaveFocus();
    expect(screen.getByRole("link", { name: "Профиль" })).toHaveAttribute("href", "/app/profile");
    expect(screen.getByRole("button", { name: "Поддержка" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Что нового?" })).toBeDisabled();
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

  it("renders a filled moon glyph for the dark theme option", () => {
    render(<AccountControl profile={profile} />);
    fireEvent.click(screen.getByRole("button", { name: ru.account.openMenuLabel }));

    const darkTheme = screen.getByRole("button", { name: ru.account.darkThemeLabel });
    const moonPath = darkTheme.querySelector("path");

    expect(moonPath).toHaveAttribute("fill", "currentColor");
    expect(moonPath).toHaveAttribute(
      "d",
      "M9.528 1.718a.75.75 0 0 1 1.162.81 8.25 8.25 0 0 0 10.78 10.78.75.75 0 0 1 .81 1.163A9.75 9.75 0 1 1 9.528 1.718Z",
    );
  });

  it("uses the mutation boundary for logout and redirects only after 204", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountControl profile={profile} />);

    fireEvent.click(screen.getByRole("button", { name: "Открыть меню аккаунта" }));
    fireEvent.click(screen.getByRole("button", { name: ru.account.logoutLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/auth/logout", { method: "POST" });
  });

  it.each([
    ["a rejected request", () => Promise.reject(new Error("untrusted detail"))],
    ["a non-204 response", () => Promise.resolve(new Response(null, { status: 200 }))],
  ])("keeps the workspace open with neutral feedback after %s", async (_caseName, request) => {
    vi.mocked(webBrowserMutation).mockImplementationOnce(request);
    render(<AccountControl profile={profile} />);

    fireEvent.click(screen.getByRole("button", { name: "Открыть меню аккаунта" }));
    fireEvent.click(screen.getByRole("button", { name: ru.account.logoutLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.account.logoutFailure);
    expect(replace).not.toHaveBeenCalled();
    expect(screen.queryByText("untrusted detail")).not.toBeInTheDocument();
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
