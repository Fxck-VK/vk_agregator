import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { buildPublicSitemap, publicPageMetadata } from "@/i18n/seo";
import type { PublicPagePublication, PublicPageSource } from "./types";

const now = Date.parse("2026-10-24T12:00:00Z");
const published: PublicPagePublication = {
  id: "tool-new-page",
  status: "published",
  indexing: "index",
  paths: { ru: "/tools/novaya-stranitsa", en: "/tools/new-page" },
  publishedAt: "2026-10-23T12:00:00Z",
  updatedAt: "2026-10-24T09:00:00Z",
};
const source = (pages: readonly PublicPagePublication[], route = "/tools/[slug]"): PublicPageSource[] => [{ route, pages }];

afterEach(() => vi.unstubAllEnvs());

describe("public page publication and sitemap", () => {
  it("adds a published Russian page without exposing the retained English source path", () => {
    vi.stubEnv("WEB_ORIGIN", "https://epifera.example");
    expect(buildPublicSitemap(source([published]), now)).toEqual([
      { url: "https://epifera.example/ru/tools/novaya-stranitsa", lastModified: published.updatedAt,
        alternates: { languages: { ru: "https://epifera.example/ru/tools/novaya-stranitsa" } } },
    ]);
  });

  it.each(["draft", "review", "archived"] as const)("removes a %s page and makes its metadata noindex", status => {
    const page = { ...published, status };
    expect(buildPublicSitemap(source([page]), now)).toEqual([]);
    expect(publicPageMetadata("ru", page, now).robots).toEqual({ index: false, follow: false });
  });

  it("keeps published noindex pages and future publications outside the sitemap", () => {
    for (const page of [
      { ...published, indexing: "noindex" as const },
      { ...published, publishedAt: "2026-10-25T12:00:00Z" },
    ]) {
      expect(buildPublicSitemap(source([page]), now)).toEqual([]);
      expect(publicPageMetadata("en", page, now).robots).toEqual({ index: false, follow: false });
    }
  });

  it("does not publish an English-only source or invent a Russian translation path", () => {
    vi.stubEnv("WEB_ORIGIN", "https://epifera.example");
    const page = { ...published, paths: { en: "/tools/new-page" } };
    expect(buildPublicSitemap(source([page]), now)).toEqual([]);
    expect(() => publicPageMetadata("en", page, now)).toThrow("no translation");
  });

  it("sets the default alternate to the actual Russian path, even when slugs differ", () => {
    vi.stubEnv("WEB_ORIGIN", "https://epifera.example");
    expect(publicPageMetadata("en", published, now).alternates?.languages).toHaveProperty("x-default", "https://epifera.example/ru/tools/novaya-stranitsa");
  });

  it.each(["/app", "/app/files", "/login", "/web/v1/models", "/__dev/login", "/assets/a", "/ru/tools/new-page", "//elsewhere.example/a", "/tools/a?tracking=1", "/tools/a#part", "/tools/../app", "/tools/%2e%2e/app"])("rejects a non-public canonical: %s", path => {
    expect(() => buildPublicSitemap(source([{ ...published, paths: { ru: path } }]), now)).toThrow();
  });

  it("rejects paths that do not belong to the implemented route", () => {
    expect(() => buildPublicSitemap(source([published], "/ai/image-generator"), now)).toThrow(/route/i);
  });

  it.each([
    ["/guides/[...slug]", "/guides/images/prompts"],
    ["/guides/[[...slug]]", "/guides"],
    ["/guides/[[...slug]]", "/guides/images/prompts"],
    ["/[[...slug]]", "/"],
  ])("accepts actual catch-all route %s at %s", (route, path) => {
    expect(buildPublicSitemap(source([{ ...published, paths: { ru: path } }], route), now)).toHaveLength(1);
  });

  it("rejects duplicate canonical URLs and IDs instead of silently hiding bad content", () => {
    expect(() => buildPublicSitemap(source([published, { ...published, id: "different-id" }]), now)).toThrow(/duplicate/i);
    expect(() => buildPublicSitemap(source([published, { ...published, paths: { en: "/tools/other" } }]), now)).toThrow(/duplicate/i);
  });

  it("rejects invalid dates and never invents lastmod from request time", () => {
    expect(() => buildPublicSitemap(source([{ ...published, updatedAt: "not-a-date" }]), now)).toThrow(/date/i);
    const { updatedAt, ...page } = published;
    void updatedAt;
    expect(buildPublicSitemap(source([page]), now)[0]).not.toHaveProperty("lastModified");
  });
});
