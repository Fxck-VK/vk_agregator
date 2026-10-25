import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request = vi.hoisted(() => ({ locale: "en" as "ru" | "en", push: vi.fn() }));
vi.mock("server-only", () => ({}));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => request.locale }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: request.push }), usePathname: () => "/en/ai/image-generator", useSearchParams: () => new URLSearchParams() }));
import ImageGeneratorPage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

afterEach(() => { request.locale = "en"; request.push.mockClear(); });
async function page() { return render(<LocaleProvider locale={request.locale}>{await ImageGeneratorPage()}</LocaleProvider>); }

describe("rebuilt image generator landing", () => {
  it.each(["ru", "en"] as const)("keeps the %s route outside indexing and sitemap", async locale => {
    request.locale = locale;
    expect((await generateMetadata()).robots).toEqual({ index: false, follow: false });
    expect(publicSitemap().some(item => item.url.includes("/ai/image-generator"))).toBe(false);
  });
  it("renders Russian content and localized links", async () => {
    request.locale = "ru";
    const { container } = await page();
    expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("includes the complete reference section sequence with one page heading", async () => {
    const { container } = await page();
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("AI Image Generator");
    for (const id of ["create", "models", "use-cases", "workflow", "trust", "numbers", "plans", "questions"]) expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(container.querySelectorAll("#questions details")).toHaveLength(11);
    expect(screen.getByText(/Source: Magnific/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Generate now" })).toHaveAttribute("href", "/en/app/image");
    expect(screen.queryByText(/20 free images|SOC 2|ISO 27001|€47/)).not.toBeInTheDocument();
  });
  it.each([
    ["02 From a reference", "Let the picture do the talking."],
    ["03 Keep it consistent", "One idea. More than one image."],
    ["04 Edit by talking", "Say what you want to change."],
  ])("opens %s without requesting a generation", async (button, heading) => {
    const fetch = vi.spyOn(globalThis, "fetch");
    await page();
    fireEvent.click(screen.getByRole("button", { name: button }));
    expect(screen.getByRole("button", { name: button })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: /01 From a prompt/ })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled(); fetch.mockRestore();
  });
  it("opens a gallery example and returns focus to its trigger on Escape", async () => {
    await page();
    const trigger = screen.getAllByRole("button", { name: /Woman with long braids.*Open details/ })[0];
    trigger.focus(); fireEvent.click(trigger);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("image-gallery-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(); expect(trigger).toHaveFocus();
  });
  it("hands a draft to the workspace without a paid request or prompt in the URL", async () => {
    const fetch = vi.spyOn(globalThis, "fetch"); const storage = vi.spyOn(Storage.prototype, "setItem");
    await page();
    fireEvent.change(screen.getByRole("textbox", { name: "Describe the image you want to create" }), { target: { value: "A paper bird above a cloud" } });
    fireEvent.click(screen.getByRole("button", { name: "Generate" }));
    expect(request.push).toHaveBeenCalledWith("/en/app/image");
    expect(fetch).not.toHaveBeenCalled(); expect(storage).not.toHaveBeenCalled();
    fetch.mockRestore(); storage.mockRestore();
  });
});
