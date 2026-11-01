import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileReferralFaq } from "./ProfileReferralFaq";
import { formatRewardPolicy, previewRewardPolicy } from "../ProfileReferralProgram/reward-policy";

describe("ProfileReferralFaq", () => {
  it("opens one answer with an accessible disclosure control", () => {
    render(<ProfileReferralFaq policy={previewRewardPolicy} />);

    const question = screen.getByText(ru.profile.referralFaqItems[0].question).closest("summary")!;
    const disclosure = question.closest("details")!;

    expect(disclosure).not.toHaveAttribute("open");
    expect(disclosure).toHaveAttribute("name", "profile-referral-faq");

    fireEvent.click(question);

    expect(disclosure).toHaveAttribute("open");
    expect(screen.getByText(formatRewardPolicy(ru.profile.referralFaqItems[0].answer, previewRewardPolicy, "ru"))).toBeVisible();

    fireEvent.click(question);

    expect(disclosure).not.toHaveAttribute("open");
  });
});
