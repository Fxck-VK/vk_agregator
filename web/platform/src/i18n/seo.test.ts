import { afterEach, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
import { getPublicOrigin, publicPageMetadata, publicSitemap } from "./seo";

afterEach(() => vi.unstubAllEnvs());
it("uses configured origin, self canonicals and reciprocal language links", () => {
  vi.stubEnv("WEB_ORIGIN", "https://platform.example");
  for (const locale of ["ru", "en"] as const) {
    expect(publicPageMetadata(locale, "/").alternates).toEqual({
      canonical: `https://platform.example/${locale}`,
      languages: { ru: "https://platform.example/ru", en: "https://platform.example/en", "x-default": "https://platform.example/ru" },
    });
  }
});
it("lists only existing public pages, without private, login or tracking URLs", () => {
  vi.stubEnv("WEB_ORIGIN", "https://platform.example");
  expect(publicSitemap().map(entry => entry.url)).toEqual(["https://platform.example/ru", "https://platform.example/en"]);
});
it("requires a trusted origin in production rather than guessing from Host", () => {
  vi.stubEnv("NODE_ENV", "production");
  vi.stubEnv("WEB_ORIGIN", "");
  expect(() => getPublicOrigin()).toThrow("WEB_ORIGIN");
});
it.each(["javascript:alert(1)", "https://user:password@example.com", "https://example.com/path", "https://example.com?q=1"])("rejects invalid public origins", origin => {
  vi.stubEnv("WEB_ORIGIN", origin);
  expect(() => getPublicOrigin()).toThrow("WEB_ORIGIN");
});
