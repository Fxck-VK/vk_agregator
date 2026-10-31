import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request = vi.hoisted(() => ({ locale: "en" as "en" | "ru" }));
vi.mock("server-only", () => ({}));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => request.locale }));
import ToolsPage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

afterEach(() => { request.locale = "en"; vi.restoreAllMocks(); });
async function page() { return render(<LocaleProvider locale={request.locale}>{await ToolsPage()}</LocaleProvider>); }

describe("English AI tools overview", () => {
  it("links to the integration landing while preserving its planned state",async()=>{await page();expect(screen.getByRole("link",{name:"Explore Epifera MCP"})).toHaveAttribute("href","/ru/mcp");expect(screen.getAllByText("Connection details are being prepared.")).toHaveLength(8);});
  it.each(["en", "ru"] as const)("keeps %s draft and outside the sitemap", async locale => {
    request.locale = locale;
    expect((await generateMetadata()).robots).toEqual({ index: false, follow: false });
    expect(publicSitemap().some(item => /\/(en|ru)\/ai\/?$/.test(item.url))).toBe(false);
  });
  it("renders Russian content and localized links", async () => {
    request.locale = "ru";
    const { container } = await page();
    expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders the complete reference section sequence and one owned shell", async () => {
    const { container } = await page();
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("A creative platform to make anything");
    for (const id of ["tools", "plugins", "models", "connections", "teams", "docs", "explore", "start"]) expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    expect(screen.getByRole("contentinfo")).toHaveAttribute("data-layout", "centered");
    expect(container.querySelectorAll("#docs li")).toHaveLength(12);
    expect(container.querySelectorAll("#explore [data-tool-artwork]")).toHaveLength(16);
    expect(container.querySelector('a[href*="magnific.com"]')).toBeNull();
    expect(container.textContent).not.toMatch(/1 million subscribers|SOC 2|ISO 27001|20 free images/);
  });
  it("switches all eight tool categories without calling a provider", async () => {
    const fetch = vi.spyOn(globalThis, "fetch");
    await page();
    for (const [tab, title] of [["Video", "Video generation"], ["Audio", "Soundtracks"], ["Spaces", "Reusable workflows"], ["Design", "Logo Maker"], ["3D", "Image to 3D"], ["Flows", "Automations"], ["Connections", "Integrations"], ["Image", "Image Generator"]]) {
      fireEvent.click(screen.getByRole("tab", { name: tab }));
      expect(screen.getByRole("tab", { name: tab })).toHaveAttribute("aria-selected", "true");
      expect(within(screen.getByRole("tabpanel", { name: tab })).getByText(title, { exact: true })).toBeInTheDocument();
    }
    expect(fetch).not.toHaveBeenCalled();
  });
  it("wraps category keyboard navigation and focuses the selected tab", async () => {
    await page();
    const image = screen.getByRole("tab", { name: "Image" });
    image.focus(); fireEvent.keyDown(image, { key: "ArrowLeft" });
    expect(screen.getByRole("tab", { name: "Connections" })).toHaveFocus();
    expect(screen.getByRole("tab", { name: "Connections" })).toHaveAttribute("aria-selected", "true");
  });
  it("switches style and AI-client panels without inventing a live connector", async () => {
    await page();
    fireEvent.click(screen.getByRole("tab", { name: "Cinematic" }));
    expect(within(screen.getByRole("tabpanel", { name: "Cinematic" })).getByText("Kling 3.0", { exact: true })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "ChatGPT" }));
    const panel = screen.getByRole("tabpanel", { name: "ChatGPT" });
    expect(within(panel).getByText("Coming soon", { exact: true })).toBeInTheDocument();
    expect(panel.textContent).toContain("ChatGPT");
    expect(panel.textContent).not.toMatch(/https:\/\/mcp\.|mcp add/);
  });
  it("opens planned plugins in a reversible preview and restores focus", async () => {
    await page();
    const trigger = screen.getByRole("button", { name: /After Effects/ });
    trigger.focus(); fireEvent.click(trigger);
    expect(screen.getByRole("dialog", { name: "After Effects" })).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByRole("link", { name: "Explore available tools" })).toHaveAttribute("href", "/ru/ai#tools");
    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("landing-dialog-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
  it("rotates audience scenarios like the recorded source states", async () => {
    const { container } = await page();
    expect(container.querySelector("#teams [data-audience-card] h3")).toHaveTextContent("Creative spaces");
    fireEvent.click(screen.getByRole("button", { name: "Media & Broadcasting" }));
    expect(container.querySelector("#teams [data-audience-card] h3")).toHaveTextContent("Campaign kits");
    expect(screen.getByRole("button", { name: "Media & Broadcasting" })).toHaveAttribute("aria-pressed", "true");
  });
});
