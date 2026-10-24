import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getDictionary } from "@/i18n/dictionary";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request = vi.hoisted(() => ({ locale: "ru" as "ru" | "en" }));
vi.mock("server-only", () => ({}));
vi.mock("next/server", () => ({ connection: vi.fn(async () => undefined) }));
vi.mock("next/navigation", () => ({ useRouter: vi.fn(), usePathname: () => "/", useSearchParams: () => new URLSearchParams() }));
vi.mock("@/i18n/server", () => ({
  getRequestLocale: async () => request.locale,
  getRequestDictionary: async () => getDictionary(request.locale),
}));

import HomePage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

afterEach(() => { request.locale = "ru"; });

describe("homepage retirement boundary", () => {
  it("keeps the Russian home empty after retiring its former layout", async () => {
    request.locale = "ru";
    const { container } = render(await HomePage());
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    expect(container.querySelector("header,footer,video,img")).toBeNull();
  });

  it("serves the rebuilt homepage only on the English route", async () => {
    request.locale = "en";
    render(<LocaleProvider locale="en">{await HomePage()}</LocaleProvider>);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("The creative platform to direct your best work");
  });

  it.each(["ru", "en"] as const)("keeps %s rebuilding home outside search", async locale => {
    request.locale = locale;
    expect((await generateMetadata()).robots).toEqual({ index: false, follow: false });
    expect(publicSitemap()).toEqual([]);
  });
});
