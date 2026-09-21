import { NextRequest } from "next/server";
import { expect, it } from "vitest";
import { proxy } from "../proxy";

it("rewrites prefixed pages, overrides spoofed locale, and preserves queries", () => {
  const response = proxy(new NextRequest("https://platform.example/en/app/chats?model=test", {
    headers: { cookie: "neirohub-locale=ru", "x-neirohub-page-locale": "ru" },
  }));
  expect(response.headers.get("x-middleware-rewrite")).toBe("https://platform.example/app/chats?model=test");
  expect(response.headers.get("x-middleware-request-x-neirohub-page-locale")).toBe("en");
  expect(response.headers.get("location")).toBeNull();
});
it("redirects an old page using preference, defaulting to Russian", () => {
  for (const [cookie, expected] of [["neirohub-locale=en", "en"], ["", "ru"], ["neirohub-locale=fr", "ru"]]) {
    const response = proxy(new NextRequest("https://platform.example/app/payment-return?payment=123", { headers: { cookie } }));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(`https://platform.example/${expected}/app/payment-return?payment=123`);
    expect(response.headers.get("cache-control")).toBe("private, no-store");
  }
});
it.each(["/fr/app", "/EN/app", "/ru/en/app", "/en/web/v1/models", "/ru/health"])("rejects invalid localized routes: %s", path => {
  expect(proxy(new NextRequest(`https://platform.example${path}`)).status).toBe(404);
});
it.each(["/web/v1/models", "/assets/icon.svg", "/health", "/api/callback", "/sitemap.xml", "/robots.txt"])("does not route technical paths: %s", path => {
  const response = proxy(new NextRequest(`https://platform.example${path}`));
  expect(response.headers.get("location")).toBeNull();
  expect(response.headers.get("x-middleware-rewrite")).toBeNull();
  expect(response.headers.getSetCookie()).toEqual([]);
});
it("stores only validated localized return path and allowed workspace parameters", () => {
  const response = proxy(new NextRequest("https://platform.example/en/app/chats?model=image&prompt=secret&token=secret"));
  expect(response.cookies.get("__Host-nh-return-to")?.value).toBe("/en/app/chats?model=image");
});
it("does not change cookies on a speculative link prefetch", () => {
  const response = proxy(new NextRequest("https://platform.example/en/app/files", { headers: { purpose: "prefetch" } }));
  expect(response.headers.getSetCookie()).toEqual([]);
});
