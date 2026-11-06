import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
// Original public-template fixtures explicitly use the development view.
vi.mock("@/features/release/server", () => ({ getRequestReleaseState: async () => ({ view: "dev", canSwitch: true, localSimulation: false }) }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ view: "dev", canSwitch: true, localSimulation: false, enabled: () => true, allowsPath: () => true, switching: false, error: false, changeView: async () => true }) }));
import { getDictionary } from "@/i18n/dictionary";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request = vi.hoisted(() => ({ locale: "ru" as "ru" | "en" }));
vi.mock("server-only", () => ({}));
vi.mock("next/server", () => ({ connection: vi.fn(async () => undefined) }));
vi.mock("next/navigation", () => ({ useRouter: vi.fn(), usePathname: () => "/", useSearchParams: () => new URLSearchParams() }));
vi.mock("@/i18n/server", () => ({
  getRequestLocale: async () => request.locale,
  getRequestDictionary: async () => getDictionary(request.locale),
  redirectLocalized: vi.fn((path, locale) => { throw new Error(`REDIRECT:/${locale}${path}`); }),
}));
vi.mock("@/features/session/home-entry.server", () => ({ loadHomeEntrySession: vi.fn() }));
vi.mock("@/features/session/HomeSessionRecovery", () => ({ HomeSessionRecovery: () => <i data-testid="home-session-recovery" /> }));
import { loadHomeEntrySession } from "@/features/session/home-entry.server";
import { redirectLocalized } from "@/i18n/server";

import HomePage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

beforeEach(() => { vi.mocked(loadHomeEntrySession).mockResolvedValue("guest"); });
afterEach(() => { request.locale = "ru"; vi.clearAllMocks(); });

describe("homepage retirement boundary", () => {
  it("sends a verified signed-in visitor straight to the localized workspace", async () => {
    vi.mocked(loadHomeEntrySession).mockResolvedValue("authenticated");
    await expect(HomePage()).rejects.toThrow("REDIRECT:/ru/app");
    expect(redirectLocalized).toHaveBeenCalledWith("/app", "ru");
  });
  it("preserves the landing while a recoverable session is checked in the browser", async () => {
    vi.mocked(loadHomeEntrySession).mockResolvedValue("refresh_required");
    render(await HomePage());
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.getByTestId("home-session-recovery")).toBeInTheDocument();
    expect(redirectLocalized).not.toHaveBeenCalled();
  });
  it("renders Russian content and localized links", async () => {
    request.locale = "ru";
    const { container } = render(await HomePage());
    expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});

  it("serves the rebuilt homepage only on the English route", async () => {
    request.locale = "en";
    render(<LocaleProvider locale={request.locale}>{await HomePage()}</LocaleProvider>);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("The creative platform to direct your best work");
  });

  it.each(["ru", "en"] as const)("keeps %s rebuilding home outside search", async locale => {
    request.locale = locale;
    expect((await generateMetadata()).robots).toEqual({ index: false, follow: false });
    expect(publicSitemap()).toEqual([]);
  });
});
