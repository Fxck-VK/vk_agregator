import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileReferralFaq } from "./ProfileReferralFaq";

describe("ProfileReferralFaq", () => {
  it("opens one answer with an accessible disclosure control", () => {
    render(<ProfileReferralFaq />);

    const question = screen.getByText(ru.profile.referralFaqItems[0].question).closest("summary")!;
    const disclosure = question.closest("details")!;

    expect(disclosure).not.toHaveAttribute("open");
    expect(disclosure).toHaveAttribute("name", "profile-referral-faq");

    fireEvent.click(question);

    expect(disclosure).toHaveAttribute("open");
    expect(screen.getByText(ru.profile.referralFaqItems[0].answer)).toBeVisible();

    fireEvent.click(question);

    expect(disclosure).not.toHaveAttribute("open");
  });
});
