import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileReferralFaq } from "./ProfileReferralFaq";

describe("ProfileReferralFaq", () => {
  it("opens one answer with an accessible disclosure control", () => {
    render(<ProfileReferralFaq />);

    const question = screen.getByRole("button", { name: ru.profile.referralFaqItems[0].question });

    expect(question).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(question);

    expect(question).toHaveAttribute("aria-expanded", "true");
    expect(question).toHaveAttribute("aria-controls");
    expect(screen.getByText(ru.profile.referralFaqItems[0].answer)).toBeInTheDocument();
  });
});
