import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileReferralProgram } from "./ProfileReferralProgram";

describe("ProfileReferralProgram", () => {
  it("keeps the launch state free of invented referral values until the web API provides them", () => {
    const { container } = render(<ProfileReferralProgram />);

    expect(screen.getByRole("heading", { name: ru.profile.referralLaunchTitle })).toBeInTheDocument();

    const renderedText = container.textContent ?? "";

    expect(screen.queryAllByRole("link")).toHaveLength(0);
    expect(renderedText).not.toMatch(/(?:https?:\/\/\S+|\b(?:ref|referral|promo|invite)=[^\s]+)/i);
    expect(renderedText).not.toMatch(/\b[A-Z0-9]{3,}(?:-[A-Z0-9]{3,})+\b/);
    expect(renderedText).not.toMatch(
      /(?:\+?\s*\d+(?:[.,]\d+)?\s*(?:\u2605|\u0437\u0432[\u0435\u0451]\u0437\u0434(?:\u0430|\u044b)?|\u0442\u043e\u043a\u0435\u043d(?:\u043e\u0432|\u0430|\u044b)?|\u20BD)|\d+(?:[.,]\d+)?\s*%)/i,
    );
    expect(renderedText).not.toMatch(
      /(?:\u0437\u0430\u0440\u0430\u0431\u043e\u0442\u0430\u043d\u043e\s*(?:\u0432\u0441\u0435\u0433\u043e)?|\u043e\u0436\u0438\u0434\u0430\u044e\u0442\s+\u043f\u0440\u043e\u0432\u0435\u0440\u043a\u0438|\u0434\u0440\u0443\u0437(?:\u0435\u0439|\u044c\u044f).*\u043a\u0443\u043f\u0438\u043b|\u0432\u0430\u0448\s+\u0434\u043e\u0445\u043e\u0434)/i,
    );

    const statisticsSection = screen
      .getByRole("heading", { name: ru.profile.referralStatisticsTitle })
      .closest("section");

    expect(statisticsSection).not.toBeNull();
    expect(statisticsSection).not.toHaveTextContent(/\d/);
  });
});
