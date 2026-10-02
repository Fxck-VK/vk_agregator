// @vitest-environment node
import { createHash } from "node:crypto";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { getDevAccessConfig, issueDevAccessCookie, hasDevAccess, isDevAccessIssuer, safeDevReturnPath } from "./session";

const fixture = "dev-user:$2y$12$" + "a".repeat(53);
const now = new Date("2026-09-29T12:00:00Z");

function configure(entry = fixture) {
  vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", entry);
  vi.stubEnv("WEB_ORIGIN", "https://dev-web.neiirohub.ru");
  return getDevAccessConfig()!;
}

afterEach(() => vi.unstubAllEnvs());

describe("DEV access cookie", () => {
  it("is host-only, protected and remembered for exactly 30 days", () => {
    const config = configure();
    const cookie = issueDevAccessCookie(config, now);
    expect(cookie).toContain("__Host-nh-dev-access=");
    expect(cookie).toContain("Max-Age=2592000");
    expect(cookie).toContain("Expires=Thu, 29 Oct 2026 12:00:00 GMT");
    for (const flag of ["Path=/", "HttpOnly", "Secure", "SameSite=Lax"]) expect(cookie).toContain(flag);
    expect(cookie).not.toContain("Domain=");
    expect(cookie).not.toContain(fixture);
    expect(hasDevAccess(config, cookie.split(";")[0], now)).toBe(true);
    expect(hasDevAccess(config, cookie, new Date("2026-10-29T11:59:59Z"))).toBe(true);
    expect(hasDevAccess(config, cookie, new Date("2026-10-29T12:00:00Z"))).toBe(false);
    expect(hasDevAccess(config, cookie, new Date("2026-09-28T12:00:00Z"))).toBe(false);
  });

  it("rejects tampering, ambiguous cookies and account cookies", () => {
    const config = configure();
    const cookie = issueDevAccessCookie(config, now).split(";")[0];
    const modified = cookie.slice(0, -2) + "!!";
    for (const value of [null, "", modified, cookie + "; " + cookie, "nh_access=account-session", "__Host-nh-dev-access=" + "a".repeat(2048)]) {
      expect(hasDevAccess(config, value, now)).toBe(false);
    }
  });

  it("survives process recreation but is revoked by changing the credential", () => {
    const cookie = issueDevAccessCookie(configure(), now);
    expect(hasDevAccess(configure(), cookie, now)).toBe(true);
    expect(hasDevAccess(configure(fixture.replace(/a/g, "b")), cookie, now)).toBe(false);
  });

  it("requires the private Nginx issuer proof and the authenticated username", () => {
    const config = configure();
    const proof = createHash("sha256").update(`neirohub-dev-web-issuer-v1:${fixture}`).digest("hex");
    expect(isDevAccessIssuer(config, proof, "dev-user")).toBe(true);
    expect(isDevAccessIssuer(config, proof, "forged-user")).toBe(false);
    expect(isDevAccessIssuer(config, "forged", "dev-user")).toBe(false);
    expect(isDevAccessIssuer(config, null, null)).toBe(false);
  });

  it.each(["", "placeholder", "user:plaintext", "user:$2y$12$invalid"])("fails closed for invalid configuration", entry => {
    configure(entry);
    expect(getDevAccessConfig()).toBeNull();
  });

  it("only enables on the configured HTTPS DEV host; supports existing APR1 secrets", () => {
    expect(configure("dev-user:$apr1$devsalt$" + "a".repeat(22))).not.toBeNull();
    for (const origin of ["", "http://dev-web.neiirohub.ru", "https://neiirohub.ru", "https://dev-web.neiirohub.ru/path"]) {
      vi.stubEnv("WEB_ORIGIN", origin);
      expect(getDevAccessConfig()).toBeNull();
    }
  });
});

describe("DEV return path", () => {
  it("preserves a local localized deep link", () => {
    expect(safeDevReturnPath("/en/app/chats?model=seedream_5_0_lite")).toBe("/en/app/chats?model=seedream_5_0_lite");
  });
  it.each([null, "//evil.test", "https://evil.test", "/\\evil.test", "/%2f/evil.test", "/%255cevil.test", "/x\r\ny", "/__dev/login", "/web/dev-access/issue", "/ru/web/dev-access/issue", "/api/private", "/_next/x", "/x".repeat(2049)])("rejects unsafe or technical destinations", value => {
    expect(safeDevReturnPath(value)).toBe("/");
  });
});
