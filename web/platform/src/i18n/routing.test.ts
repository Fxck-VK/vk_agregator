import { describe, expect, it } from "vitest";
import { localeFromPath, stripLocale, localizeHref, isServicePath } from "./routing";

describe("locale URL contract", () => {
  it("recognizes only a complete supported locale segment", () => {
    expect(localeFromPath("/en/app")).toBe("en");
    expect(localeFromPath("/ru")).toBe("ru");
    for (const path of ["/english/app", "/fr/app", "/app", "//en/app"]) {
      expect(localeFromPath(path)).toBeNull();
    }
  });
  it("maps to stable internal pages without changing the URL suffix", () => {
    expect(stripLocale("/en")).toBe("/");
    expect(stripLocale("/ru/app?model=x#draft")).toBe("/app?model=x#draft");
    expect(stripLocale("/app/chat/id")).toBe("/app/chat/id");
  });
  it("switches page language once and preserves query/fragment bytes", () => {
    expect(localizeHref("/", "en")).toBe("/en");
    expect(localizeHref("/ru/app/chat/id?model=a%2Bb&x=1#draft", "en"))
      .toBe("/en/app/chat/id?model=a%2Bb&x=1#draft");
    expect(localizeHref("/app", "ru")).toBe("/ru/app");
    expect(localizeHref("/en/app", "en")).toBe("/en/app");
  });
  it.each(["/web/v1/models", "/health", "/api/auth/callback", "/assets/icon.svg", "/_next/static/x.js", "/sitemap.xml", "/robots.txt"])
  ("leaves technical URLs unchanged: %s", path => {
    expect(isServicePath(path)).toBe(true);
    expect(localizeHref(path, "en")).toBe(path);
  });
  it.each(["https://example.com/app", "//example.com/app", "mailto:test@example.com", "#section", "?model=x"])
  ("leaves external and document-relative URLs unchanged: %s", path => {
    expect(localizeHref(path, "en")).toBe(path);
  });
});
