import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { homeEn } from "@/i18n/public/home-en";
import { LandingFooter } from "./LandingFooter";

it("keeps the existing footer layout and legal destinations by default", () => {
  render(<LocaleProvider locale="en"><LandingFooter {...homeEn.footer} /></LocaleProvider>);
  expect(screen.getByRole("contentinfo")).toHaveAttribute("data-layout", "standard");
  expect(screen.getByRole("link", { name: "Privacy policy" })).toHaveAttribute("href", "/en/legal/privacy-policy");
});
it("opts into the centered reference layout without changing existing defaults", () => {
  render(<LocaleProvider locale="en"><LandingFooter {...homeEn.footer} layout="centered" /></LocaleProvider>);
  expect(screen.getByRole("contentinfo")).toHaveAttribute("data-layout", "centered");
  expect(screen.getByRole("link", { name: "User agreement" })).toHaveAttribute("href", "/en/legal/user-agreement");
});
