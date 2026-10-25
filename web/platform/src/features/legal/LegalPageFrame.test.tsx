import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { LegalPageFrame } from "./LegalPageFrame";

const documents = [
  { slug: "user-agreement", title: "User agreement" },
  { slug: "privacy-policy", title: "Privacy policy" },
  { slug: "paid-access", title: "Payment and refund terms" },
  { slug: "payment-details", title: "Payment details" },
  { slug: "marketing-consent", title: "Email marketing consent" },
] as const;
function page() {
  return render(<LocaleProvider locale="en"><LegalPageFrame locale="en" currentSlug="privacy-policy" documents={documents}><h1>Privacy policy</h1></LegalPageFrame></LocaleProvider>);
}

describe("legal navigation", () => {
  it("expands the current document group and keeps the active link localized", () => {
    page();
    const navigation = screen.getByRole("navigation", { name: "Legal pages" });
    const current = within(navigation).getByRole("link", { name: "Privacy policy" });
    expect(current).toHaveAttribute("aria-current", "page");
    expect(current.closest("details")).toHaveAttribute("open");
    expect(current).toHaveAttribute("href", "/en/legal/privacy-policy");
    expect(screen.queryByRole("button", { name: /theme/i })).not.toBeInTheDocument();
  });

  it("opens a document drawer, traps focus and returns focus on Escape", () => {
    const { container } = page();
    const trigger = screen.getByRole("button", { name: "Open legal menu" });
    trigger.focus(); fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Legal pages" });
    const close = within(dialog).getByRole("button", { name: "Close menu" });
    expect(close).toHaveFocus();
    expect(container.querySelector("[data-legal-page]")).toHaveProperty("inert", true);
    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(within(dialog).getAllByRole("link", { name: "Contact support" }).at(-1)).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: "Tab" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("legal-navigation-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(container.querySelector("[data-legal-page]")).toHaveProperty("inert", false);
  });

  it("provides real app destinations in the mobile global navigation", () => {
    page();
    fireEvent.click(screen.getByRole("button", { name: "Open navigation" }));
    const dialog = screen.getByRole("dialog", { name: "Navigation" });
    expect(within(dialog).getByRole("link", { name: "New chat" })).toHaveAttribute("href", "/en/app");
    expect(within(dialog).getByRole("link", { name: "All AI models" })).toHaveAttribute("href", "/en/app/models");
    expect(within(dialog).getByRole("link", { name: "Legal pages" })).toHaveAttribute("href", "/en/legal");
  });
});
