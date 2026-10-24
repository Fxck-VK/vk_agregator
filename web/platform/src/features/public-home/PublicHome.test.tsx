import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { homeEn } from "@/i18n/public/home-en";
import { PublicHome } from "./PublicHome";

vi.mock("next/navigation", () => ({ useRouter: vi.fn() }));
vi.mock("@/features/models/model-catalog-cache", () => ({
  loadModelCatalog: vi.fn(async () => ({ items: [{ id: "nano_banana_pro", name: "Nano Banana Pro" }, { id: "seedream_5_0_lite", name: "Seedream 5.0 Lite" }] })),
  resetModelCatalogCacheForTests: vi.fn(),
}));

function home() { render(<LocaleProvider locale="en"><PublicHome copy={homeEn} /></LocaleProvider>); }
beforeEach(() => { vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined); });

describe("rebuilt English public home", () => {
  it("renders one English homepage with existing localized workspace destinations", () => {
    home();
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(homeEn.hero.title);
    expect(screen.getAllByRole("link", { name: homeEn.start })[0]).toHaveAttribute("href", "/en/app");
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });

  it("switches the suite using arrow keys and exposes honest upcoming panels", () => {
    home();
    const tabs = within(screen.getByRole("tablist", { name: homeEn.suite.tabsLabel }));
    fireEvent.keyDown(tabs.getByRole("tab", { name: "Creative Suite" }), { key: "ArrowRight" });
    expect(tabs.getByRole("tab", { name: "Workspace" })).toHaveAttribute("aria-selected", "true");
    expect(tabs.getByRole("tab", { name: "Workspace" })).toHaveFocus();
    fireEvent.click(tabs.getByRole("tab", { name: "MCP & API" }));
    expect(screen.getByRole("tabpanel", { name: "MCP & API" })).toHaveTextContent("Coming soon");
  });

  it("filters tool cards without navigation or generation requests", () => {
    home();
    fireEvent.click(within(screen.getByRole("tablist", { name: homeEn.tools.filtersLabel })).getByRole("tab", { name: "Audio" }));
    const panel = screen.getByRole("tabpanel", { name: "Audio" });
    expect(within(panel).getByRole("link", { name: /Music Generator/ })).toHaveAttribute("href", "/en/app/music");
    expect(within(panel).queryByRole("link", { name: /AI Image Generator/ })).not.toBeInTheDocument();
  });

  it("pauses motion and lets visitors choose a specific showcase slide", () => {
    home();
    fireEvent.click(screen.getByRole("button", { name: homeEn.controls.pause }));
    expect(screen.getByRole("button", { name: homeEn.controls.play })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Go to slide 3" }));
    expect(screen.getByRole("heading", { name: homeEn.originals.slides[2].title })).toBeInTheDocument();
  });

  it("searches the shared model catalog including Russian aliases", async () => {
    home();
    fireEvent.click(screen.getByRole("button", { name: homeEn.controls.search }));
    const dialog = screen.getByRole("dialog", { name: homeEn.controls.search });
    const input = within(dialog).getByRole("searchbox");
    expect(input).toHaveFocus();
    fireEvent.change(input, { target: { value: "нана банана про" } });
    const result = await within(dialog).findByRole("link", { name: /Nano Banana Pro/ });
    expect(result).toHaveAttribute("href", "/en/app?model=nano_banana_pro");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(result).toHaveFocus();
  });

  it("closes its dialog with Escape, restores scrolling and returns focus", () => {
    home();
    const trigger = screen.getByRole("button", { name: homeEn.controls.search });
    trigger.focus(); fireEvent.click(trigger);
    expect(document.body.style.overflow).toBe("hidden");
    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("landing-dialog-backdrop"), { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.body.style.overflow).not.toBe("hidden");
    expect(trigger).toHaveFocus();
  });
});
