import { render, screen, cleanup } from "@testing-library/react";
import { NextRequest } from "next/server";
import { afterEach, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("./actions", () => ({ setLocalePreference: vi.fn() }));

import { proxy } from "@/proxy";
import { buildPublicSitemap, publicPageMetadata } from "./seo";
import { localizeHref } from "./routing";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { LocaleProvider } from "./LocaleProvider";

afterEach(() => { cleanup(); vi.unstubAllEnvs(); });

it("retires English page addresses with a permanent Russian redirect and preserves the query", () => {
  const response = proxy(new NextRequest("https://platform.example/en/ai/image-generator?model=a%2Bb&from=old", {
    headers: { cookie: "neirohub-locale=en" },
  }));
  expect(response.status).toBe(308);
  expect(response.headers.get("location")).toBe("https://platform.example/ru/ai/image-generator?model=a%2Bb&from=old");
  expect(response.headers.get("x-middleware-rewrite")).toBeNull();
});

it("never selects an English page from an old language preference", () => {
  const response = proxy(new NextRequest("https://platform.example/app", { headers: { cookie: "neirohub-locale=en" } }));
  expect(response.headers.get("location")).toBe("https://platform.example/ru/app");
});

it("recognizes the legacy AI product root as a page rather than a language code", () => {
  const response = proxy(new NextRequest("https://platform.example/ai/image-generator?model=x"));
  expect(response.status).toBe(307);
  expect(response.headers.get("location")).toBe("https://platform.example/ru/ai/image-generator?model=x");
});

it("generates Russian page links even from legacy English language context", () => {
  expect(localizeHref("/en/ai/image-generator?model=x#examples", "en")).toBe("/ru/ai/image-generator?model=x#examples");
  expect(localizeHref("/web/v1/models", "en")).toBe("/web/v1/models");
});

it("does not offer English or an unnecessary language selector", () => {
  render(<LocaleProvider locale="ru"><LanguageSwitcher /></LocaleProvider>);
  expect(screen.queryByRole("button", { name: "English" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Русский" })).toBeNull();
});

it("emits Russian-only public sitemap and SEO alternates", () => {
  vi.stubEnv("WEB_ORIGIN", "https://platform.example");
  const page = { id: "ru-only-tool", status: "published" as const, indexing: "index" as const,
    paths: { ru: "/ai/image-generator", en: "/ai/image-generator" } };
  expect(buildPublicSitemap([{ route: "/ai/image-generator", pages: [page] }])).toEqual([
    { url: "https://platform.example/ru/ai/image-generator", alternates: { languages: { ru: "https://platform.example/ru/ai/image-generator" } } },
  ]);
  expect(publicPageMetadata("en", page).alternates).toEqual({
    canonical: "https://platform.example/ru/ai/image-generator",
    languages: { ru: "https://platform.example/ru/ai/image-generator", "x-default": "https://platform.example/ru/ai/image-generator" },
  });
});
