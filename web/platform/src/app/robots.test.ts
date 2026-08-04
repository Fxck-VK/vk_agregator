import { describe, expect, it } from "vitest";

import robots from "./robots";

describe("robots metadata route", () => {
  it("indexes the public site and excludes private and BFF routes", () => {
    expect(robots()).toEqual({
      rules: {
        userAgent: "*",
        allow: "/",
        disallow: ["/app/", "/login", "/web/"],
      },
      sitemap: "https://neiirohub.ru/sitemap.xml",
      host: "https://neiirohub.ru",
    });
  });
});
