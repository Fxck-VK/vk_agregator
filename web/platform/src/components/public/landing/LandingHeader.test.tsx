import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { homeEn } from "@/i18n/public/home-en";
import { LandingHeader } from "./LandingHeader";

vi.mock("next/navigation", () => ({ useRouter: vi.fn() }));
afterEach(() => vi.unstubAllGlobals());

function header(onSearch = vi.fn()) {
  render(<LocaleProvider locale="en"><LandingHeader navigation={homeEn.navigation} labels={{ ...homeEn.controls, login: homeEn.login, signup: homeEn.signup }} onSearch={onSearch} /></LocaleProvider>);
}

describe("reusable landing navigation", () => {
  it("opens actual localized destinations and returns focus on Escape", () => {
    header();
    const trigger = screen.getByRole("button", { name: "Creative Suite" });
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("link", { name: "Image Generator" })).toHaveAttribute("href", "/en/app/image");
    fireEvent.keyDown(window, { key: "Escape" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("keeps mobile navigation reversible and closes it after choosing a link", () => {
    header();
    fireEvent.click(screen.getByRole("button", { name: "Open navigation" }));
    expect(screen.getByRole("button", { name: "Close navigation" })).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(within(screen.getByRole("navigation", { name: "Main navigation" })).getByRole("link", { name: "Plans" }));
    expect(screen.getByRole("button", { name: "Open navigation" })).toHaveAttribute("aria-expanded", "false");
  });

  it("opens search through the button and keyboard shortcut", () => {
    const search = vi.fn(); header(search);
    fireEvent.click(screen.getByRole("button", { name: "Search or create" }));
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(search).toHaveBeenCalledTimes(2);
  });

  it("keeps a hover-opened menu open on the first click", () => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({ matches: query === "(hover: hover)", addEventListener: vi.fn(), removeEventListener: vi.fn() })));
    header();
    const trigger = screen.getByRole("button", { name: "Creative Suite" });
    fireEvent.mouseEnter(trigger.parentElement!);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});
