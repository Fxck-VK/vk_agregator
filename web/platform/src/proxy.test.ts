import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";

import { config, proxy } from "./proxy";

const returnCookieName = "__Host-nh-return-to";
const privatePath = "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906";

describe("platform return-path proxy", () => {
  it("matches rendered UI requests and leaves BFF and health routes alone", () => {
    expect(config).toEqual({ matcher: ["/", "/login", "/app/:path*"] });
  });

  it.each(["/", "/login", privatePath])("sets a fresh strict nonce CSP for the rendered UI route %s", (pathname) => {
    const response = proxy(new NextRequest(`https://platform.example${pathname}`));
    const csp = response.headers.get("Content-Security-Policy");
    const nonce = response.headers.get("x-middleware-request-x-nonce");

    expect(nonce).toMatch(/^[A-Za-z0-9+/]+={0,2}$/);
    expect(response.headers.get("x-nonce")).toBeNull();
    expect(csp).toContain(`script-src 'self' 'nonce-${nonce}' 'strict-dynamic'`);
    expect(csp).toContain(`style-src 'self' 'nonce-${nonce}'`);
    expect(csp).toContain("default-src 'self'");
    expect(csp).toContain("img-src 'self' data: blob:");
    expect(csp).toContain("font-src 'self'");
    expect(csp).toContain("connect-src 'self'");
    expect(csp).toContain("media-src 'self' blob:");
    expect(csp).toContain("object-src 'none'");
    expect(csp).toContain("base-uri 'none'");
    expect(csp).toContain("form-action 'self'");
    expect(csp).toContain("frame-src 'none'");
    expect(csp).toContain("frame-ancestors 'none'");
    expect(csp).toContain("upgrade-insecure-requests");
    expect(csp).not.toContain("unsafe-inline");
    expect(csp).not.toContain("unsafe-eval");
    expect(response.headers.get("x-middleware-request-content-security-policy")).toBe(csp);
  });

  it("generates a different nonce for each rendered UI response", () => {
    const first = proxy(new NextRequest("https://platform.example/login"));
    const second = proxy(new NextRequest("https://platform.example/login"));

    expect(first.headers.get("x-middleware-request-x-nonce")).not.toBe(
      second.headers.get("x-middleware-request-x-nonce"),
    );
  });

  it("continues a safe private request and sets the exact short-lived server cookie", async () => {
    const request = new NextRequest(`https://platform.example${privatePath}?ignored=query`);

    const response = proxy(request);
    const cookie = response.cookies.get(returnCookieName);

    expect(response.headers.get("Location")).toBeNull();
    expect(response.headers.get("x-middleware-rewrite")).toBeNull();
    expect(await response.text()).toBe("");
    expect(cookie).toMatchObject({
      name: returnCookieName,
      value: privatePath,
      httpOnly: true,
      secure: true,
      sameSite: "lax",
      path: "/",
      maxAge: 300,
    });
    const browserCookies = response.headers.getSetCookie();
    expect(browserCookies).toHaveLength(1);
    expect(browserCookies[0]).toContain("__Host-nh-return-to=%2Fapp%2Fchat%2F");
    expect(browserCookies[0]).toContain("Path=/");
    expect(browserCookies[0]).toContain("Max-Age=300");
    expect(browserCookies[0]).toContain("Secure");
    expect(browserCookies[0]).toContain("HttpOnly");
    expect(browserCookies[0]).toContain("SameSite=lax");
    expect(browserCookies[0]).not.toContain("Domain=");
    expect(browserCookies[0]).not.toContain("ignored=query");
    const middlewareCookie = response.headers.get("x-middleware-set-cookie");
    expect(middlewareCookie).toContain("__Host-nh-return-to=%2Fapp%2Fchat%2F");
    expect(middlewareCookie).toContain("Path=/");
    expect(middlewareCookie).toContain("Max-Age=300");
    expect(middlewareCookie).toContain("Secure");
    expect(middlewareCookie).toContain("HttpOnly");
    expect(middlewareCookie).toContain("SameSite=lax");
    expect(middlewareCookie).not.toContain("Domain=");
    expect(middlewareCookie).not.toContain("ignored=query");
  });

  it.each([
    "//attacker.example/app",
    "/app//chat",
    "/app/%2fchat",
    "/app\\chat",
  ])("does not set a return cookie for an unsafe path: %s", (pathname) => {
    const request = { nextUrl: { pathname } } as NextRequest;

    const response = proxy(request);

    expect(response.cookies.get(returnCookieName)).toBeUndefined();
    expect(response.headers.getSetCookie()).toEqual([]);
  });
});
