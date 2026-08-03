import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileReferralProgram } from "./ProfileReferralProgram";

describe("ProfileReferralProgram", () => {
  it("keeps the launch state free of invented referral values until the web API provides them", () => {
    render(<ProfileReferralProgram />);

    expect(screen.getByRole("heading", { name: ru.profile.referralLaunchTitle })).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByText(/(?:https?:\/\/|(?:\?|&)ref=)/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/(?:промокод|[A-Z]-[A-Z0-9]{4,})/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/(?:\+?\d[\d\s]*(?:★|токен(?:ов|а)?|₽)|\d+\s*%)/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/(?:заработано всего|ожидают проверки|друзей купили)/i)).not.toBeInTheDocument();

    const statisticsSection = screen
      .getByRole("heading", { name: ru.profile.referralStatisticsTitle })
      .closest("section");

    expect(statisticsSection).not.toBeNull();
    expect(statisticsSection).not.toHaveTextContent(/\d/);
  });
});
