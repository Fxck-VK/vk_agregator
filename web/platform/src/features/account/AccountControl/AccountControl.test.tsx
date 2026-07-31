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
  });

  it("shows only the first verified, non-empty identity label", () => {
    render(<AccountControl profile={profile} />);

    expect(screen.getByRole("heading", { name: ru.account.heading })).toBeInTheDocument();
    expect(screen.getByText("member@example.com")).toBeInTheDocument();
    expect(screen.queryByText(profile.account_id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].id)).not.toBeInTheDocument();
    expect(screen.queryByText(profile.identity_refs[0].provider)).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
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

  it("uses the mutation boundary for logout and redirects only after 204", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountControl profile={profile} />);

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

    fireEvent.click(screen.getByRole("button", { name: ru.account.logoutLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.account.logoutFailure);
    expect(replace).not.toHaveBeenCalled();
    expect(screen.queryByText("untrusted detail")).not.toBeInTheDocument();
  });
});
