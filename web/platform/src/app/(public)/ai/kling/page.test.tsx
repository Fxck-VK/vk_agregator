import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request = vi.hoisted(() => ({ locale: "en" as "en" | "ru" }));
vi.mock("server-only", () => ({}));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => request.locale }));
import KlingPage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

beforeEach(() => { vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(); vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {}); });
afterEach(() => { cleanup(); request.locale = "en"; vi.restoreAllMocks(); });
async function page() { return render(<LocaleProvider locale={request.locale}>{await KlingPage()}</LocaleProvider>); }

describe("Kling model-family landing", () => {
  it.each(["en", "ru"] as const)("keeps %s draft and outside sitemap", async locale => {
    request.locale = locale;
    expect((await generateMetadata()).robots).toEqual({ index:false, follow:false });
    expect(publicSitemap().some(item => /\/(en|ru)\/ai\/kling\/?$/.test(item.url))).toBe(false);
  });
  it("renders Russian content and localized links", async () => {
    request.locale = "ru";
    const {container} = await page();
    expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders the complete model-family layout with one owned shell", async () => {
    const {container} = await page();
    expect(screen.getAllByRole("heading",{level:1})).toHaveLength(1);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    for(const id of ["examples","versions","choose","whats-new","compare","in-epifera","numbers","plans","questions"]) expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(container.querySelectorAll("#versions tbody tr")).toHaveLength(9);
    expect(container.querySelectorAll("#choose article")).toHaveLength(4);
    expect(container.querySelectorAll("#compare [data-comparison-clip]")).toHaveLength(3);
    expect(container.querySelectorAll("#questions details")).toHaveLength(12);
    expect(screen.queryByText(/ISO 27001|Trustpilot|Unlimited, all year|Premium\+/)).not.toBeInTheDocument();
  });
  it("shows application limits and keeps reference versions out of launch links", async () => {
    const {container} = await page();
    const row = within(screen.getByRole("table",{name:"Kling versions in Epifera"})).getByRole("link",{name:"View Kling 3.0"}).closest("tr")!;
    expect(row).toHaveTextContent("Not selectable");
    expect(within(row).getByRole("link",{name:"View Kling 3.0"})).toHaveAttribute("href","/en/app?model=video_kling_v3");
    expect(container.querySelector('a[href*="modelId="]')).toBeNull();
    expect(container.querySelector('a[href*="kling-40"]')).toBeNull();
    const fetch = vi.spyOn(globalThis,"fetch");
    fireEvent.click(screen.getByRole("button",{name:"About Kling 4.0"}));
    expect(screen.getByRole("dialog",{name:"Kling 4.0"})).toHaveTextContent("Not listed in the Epifera catalog");
    expect(fetch).not.toHaveBeenCalled();
  });
  it("opens the gallery with local media, pauses motion and restores focus on Escape", async () => {
    const {container} = await page();
    const trigger = screen.getAllByRole("button",{name:/Open example/})[0];
    trigger.focus(); fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog",{name:"Kling reference example"});
    expect(dialog).toHaveTextContent("Magnific reference");
    expect(dialog.querySelector("video")).toHaveAttribute("src",expect.stringContaining("/assets/videos/model-family/"));
    expect(container.querySelector("[data-model-family-page]")).toHaveAttribute("data-paused","true");
    fireEvent.keyDown(window,{key:"Escape"});
    fireEvent.animationEnd(screen.getByTestId("family-gallery-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
  it("offers an explicit motion pause and never claims reference research as Epifera data", async () => {
    const {container} = await page();
    fireEvent.click(screen.getByRole("button",{name:/Pause motion/}));
    expect(screen.getByRole("button",{name:/Resume motion/})).toHaveAttribute("aria-pressed","true");
    expect(container.querySelector("#numbers")).toHaveTextContent("Magnific usage data");
    expect(container.querySelector("#numbers")).toHaveTextContent("not Epifera usage");
    expect(container.querySelectorAll("#numbers figure")).toHaveLength(3);
    for(const anchor of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]')) expect(container.querySelector(anchor.getAttribute("href")!)).not.toBeNull();
  });
  it("starts comparison audio muted and only changes it on explicit interaction", async () => {
    const {container} = await page();
    const control=screen.getByRole("button",{name:"Sound for Kling 2.6"});
    const video=container.querySelector<HTMLVideoElement>("#compare video")!;
    expect(video.muted).toBe(true);
    fireEvent.click(control);
    expect(control).toHaveAttribute("aria-pressed","true");
    expect(video.muted).toBe(false);
    fireEvent.click(control);
    expect(video.muted).toBe(true);
  });
});
