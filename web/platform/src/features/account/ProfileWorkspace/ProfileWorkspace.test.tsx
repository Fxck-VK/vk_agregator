import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { WorkspaceAccountProvider } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { ru } from "@/i18n/ru";
import type { AccountProfile } from "@/lib/web-api/contracts";

import { ProfileWorkspace } from "./ProfileWorkspace";

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
  afterEach(cleanup);

  it("uses the session snapshot for the verified identity and star balance without inventing billing data", () => {
    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    expect(screen.getByRole("heading", { name: "Профиль" })).toBeInTheDocument();
    expect(screen.getAllByText("m***@example.com")).toHaveLength(2);
    expect(screen.queryByText("member@example.com")).not.toBeInTheDocument();
    expect(screen.getByLabelText("104 ★")).toBeInTheDocument();
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
    expect(screen.queryByText(/^0 ★$/)).not.toBeInTheDocument();
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

    expect(screen.getByText(ru.account.unavailableLabel)).toBeInTheDocument();
    expect(screen.getByText(ru.profile.noVerifiedIdentity)).toBeInTheDocument();
    expect(screen.queryByText(ru.profile.verifiedIdentity)).not.toBeInTheDocument();
  });

  it("opens the referral launch state without showing the general billing panel", () => {
    render(
      <WorkspaceAccountProvider snapshot={{ balance: 104, profile }}>
        <ProfileWorkspace />
      </WorkspaceAccountProvider>,
    );

    const overviewTab = screen.getByRole("tab", { name: ru.profile.overviewTabLabel });
    const referralTab = screen.getByRole("tab", { name: ru.profile.referralTabLabel });

    for (const tab of [overviewTab, referralTab]) {
      const panelId = tab.getAttribute("aria-controls");

      expect(panelId).not.toBeNull();
      expect(document.getElementById(panelId ?? "")).toHaveAttribute("role", "tabpanel");
    }

    fireEvent.click(referralTab);

    expect(screen.getByRole("tabpanel", { name: ru.profile.referralTabLabel })).toBeInTheDocument();
    expect(screen.getByText(ru.profile.referralLaunchTitle)).toBeInTheDocument();
    expect(document.getElementById(overviewTab.getAttribute("aria-controls") ?? "")).toHaveAttribute("hidden");
    expect(screen.queryByRole("heading", { name: ru.profile.billingTitle })).not.toBeInTheDocument();

    fireEvent.keyDown(referralTab, { key: "ArrowLeft" });

    expect(overviewTab).toHaveAttribute("aria-selected", "true");
    expect(overviewTab).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: ru.profile.overviewTabLabel })).toBeInTheDocument();
  });
});
