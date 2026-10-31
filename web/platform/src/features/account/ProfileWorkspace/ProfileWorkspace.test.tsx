import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { WorkspaceAccountProvider } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { ru } from "@/i18n/ru";
import type { AccountProfile } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";

import { ProfileWorkspace } from "./ProfileWorkspace";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
vi.mock("@/features/subscriptions/subscriptions", () => ({ loadSubscription: async () => ({ subscription: null, balances: [], checkout_available: false, recurring_available: false }), cancelSubscription: vi.fn() }));

const profile: AccountProfile = {
  account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
  identity_refs: [
    {
      id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
      account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
      provider: "email",
      label: "m***@example.com",
      verified: true,
      created_at: "2026-07-31T09:00:00Z",
    },
  ],
};

describe("ProfileWorkspace", () => {
  afterEach(() => {
    cleanup();
    endBrowserSession();
    vi.resetAllMocks();
    Reflect.deleteProperty(navigator, "clipboard");
  });

  it("uses the session snapshot for the verified identity and star balance without inventing billing data", () => {
    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    expect(screen.getByRole("heading", { name: "Профиль" })).toBeInTheDocument();
    expect(screen.getAllByText("m***@example.com")).toHaveLength(2);
    expect(screen.queryByText("member@example.com")).not.toBeInTheDocument();
    expect(screen.getByLabelText("104 звезды")).toBeInTheDocument();
    expect(screen.getByText("Электронная почта")).toBeInTheDocument();
    expect(screen.getByText("История покупок и списаний появится здесь после подключения биллинга.")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Реферальная программа" })).toHaveAttribute("aria-selected", "false");
    expect(screen.getByRole("button", { name: "Ввести промокод" })).toBeDisabled();
    expect(screen.queryByText(profile.account_id)).not.toBeInTheDocument();
    expect(screen.queryByText("Lite")).not.toBeInTheDocument();
    expect(screen.queryByText("NH")).not.toBeInTheDocument();
  });

  it("does not replace an unavailable balance with a made-up zero", () => {
    render(
      <WorkspaceAccountProvider snapshot={{ balance: null, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    expect(screen.getByLabelText("Баланс временно недоступен")).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByLabelText("0 звёзд")).not.toBeInTheDocument();
  });
  it("does not claim a verified identity when no verified login method exists", () => {
    const profileWithoutVerifiedIdentity: AccountProfile = {
      ...profile,
      identity_refs: profile.identity_refs.map((identity) => ({ ...identity, verified: false })),
    };

    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile: profileWithoutVerifiedIdentity }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    expect(screen.getByText("Мой профиль")).toBeInTheDocument();
    expect(screen.getByText(ru.profile.noVerifiedIdentity)).toBeInTheDocument();
    expect(screen.queryByText(ru.profile.verifiedIdentity)).not.toBeInTheDocument();
  });

  it("keeps the profile title on the primary verified email when Google is linked first", () => {
    render(
      <WorkspaceAccountProvider
        snapshot={{
          balance: 104,
          profile: {
            ...profile,
            identity_refs: [
              {
                ...profile.identity_refs[0],
                id: "d7c979f5-24e5-4f88-924b-a592d6e5a907",
                provider: "google",
                label: "google-member@example.com",
              },
              {
                ...profile.identity_refs[0],
                email_role: "primary",
                label: "primary@example.com",
              },
            ],
          },
        }}
      >
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    const identityCard = screen.getByLabelText(ru.profile.identityCardLabel);
    expect(within(identityCard).getByText("primary@example.com")).toBeInTheDocument();
    expect(within(identityCard).queryByText("google-member@example.com")).not.toBeInTheDocument();
  });

  it("replaces the general panel with the referral panel content", () => {
    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    const overviewTab = screen.getByRole("tab", { name: ru.profile.overviewTabLabel });
    const referralTab = screen.getByRole("tab", { name: ru.profile.referralTabLabel });
    const panelId = overviewTab.getAttribute("aria-controls");

    expect(panelId).not.toBeNull();
    expect(referralTab).toHaveAttribute("aria-controls", panelId ?? "");
    expect(document.querySelectorAll('[role="tabpanel"]')).toHaveLength(1);
    expect(screen.getByText(ru.profile.billingPlaceholder)).toBeInTheDocument();
    expect(screen.queryByText(ru.profile.referralLaunchTitle)).not.toBeInTheDocument();

    fireEvent.click(referralTab);

    expect(screen.getByRole("tabpanel", { name: ru.profile.referralTabLabel })).toBeInTheDocument();
    expect(document.querySelectorAll('[role="tabpanel"]')).toHaveLength(1);
    expect(screen.getByRole("heading", { name: ru.profile.referralLaunchTitle })).toBeInTheDocument();
    expect(screen.queryByText(ru.profile.billingPlaceholder)).not.toBeInTheDocument();

    fireEvent.keyDown(referralTab, { key: "ArrowLeft" });

    expect(overviewTab).toHaveAttribute("aria-selected", "true");
    expect(overviewTab).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: ru.profile.overviewTabLabel })).toBeInTheDocument();
    expect(screen.getByText(ru.profile.billingPlaceholder)).toBeInTheDocument();
    expect(screen.queryByText(ru.profile.referralLaunchTitle)).not.toBeInTheDocument();
  });

  it("reuses the invitation across actual profile tab switches and keeps copying available when statistics fail", async () => {
    beginBrowserSession(profile.account_id);
    const summary = { code: "INVITE1234", visits: 7, registered: 3, activated: 0, rewarded: 0, rewards_enabled: false };
    let finishStatistics!: (response: Response) => void;
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json(summary))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finishStatistics = resolve; }))
      .mockResolvedValueOnce(Response.json({ visits: 9, registered: 4, activated: 0, rewarded: 0, rewards_enabled: false }));
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );
    fireEvent.click(screen.getByRole("tab", { name: ru.profile.referralTabLabel }));
    const inviteURL = `${window.location.origin}/ru/invite/INVITE1234`;
    expect(await screen.findByRole("textbox", { name: ru.profile.referralLinkLabel })).toHaveValue(inviteURL);

    fireEvent.click(screen.getByRole("tab", { name: ru.profile.overviewTabLabel }));
    expect(screen.queryByRole("textbox", { name: ru.profile.referralLinkLabel })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: ru.profile.referralTabLabel }));
    expect(screen.getByRole("textbox", { name: ru.profile.referralLinkLabel })).toHaveValue(inviteURL);
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/referrals/stats", expect.objectContaining({ cache: "no-store" }));
    fireEvent.click(screen.getByRole("button", { name: ru.profile.referralCopy }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(inviteURL));

    await act(async () => { finishStatistics(new Response(null, { status: 503 })); });
    expect(screen.getByRole("alert")).toHaveTextContent(ru.profile.referralStatisticsFailure);
    expect(screen.getByRole("textbox", { name: ru.profile.referralLinkLabel })).toHaveValue(inviteURL);
    expect(screen.getByRole("button", { name: ru.profile.referralCopy })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: ru.profile.referralRetry }));
    expect(await screen.findByText("9")).toBeInTheDocument();
    expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => path === "/web/v1/referrals")).toHaveLength(1);
    expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => path === "/web/v1/referrals/stats")).toHaveLength(2);
  });
});
