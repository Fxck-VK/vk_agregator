// @vitest-environment node
import { createHash } from "node:crypto";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
import { GET } from "./route";

const fixture = "dev-user:$2y$12$" + "a".repeat(53);
const origin = "https://dev-web.neiirohub.ru";
const proof = createHash("sha256").update(`neirohub-dev-web-issuer-v1:${fixture}`).digest("hex");
const issuer = { "x-dev-web-proof": proof, "x-dev-web-user": "dev-user" };

function call(action: string, headers: Record<string, string> = {}, query = "") {
  return GET(new Request(`${origin}/web/dev-access/${action}${query}`, { headers }), { params: Promise.resolve({ action }) });
}

beforeEach(() => {
  vi.stubEnv("WEB_ORIGIN", origin);
  vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", fixture);
});
afterEach(() => vi.unstubAllEnvs());

describe("internal DEV access handler", () => {
  it("issues after the Nginx password check and accepts only a valid cookie thereafter", async () => {
    const login = await call("issue", issuer, "?returnTo=%2Fen%2Fapp");
    expect(login.status).toBe(303);
    expect(login.headers.get("location")).toBe("/en/app");
    const cookie = login.headers.get("set-cookie")!;
    expect(cookie).toContain("HttpOnly");
    const allowed = await call("check", { cookie });
    expect(allowed.status).toBe(204);
    expect(allowed.headers.get("set-cookie")).toBeNull(); // No sliding renewal.
    expect(allowed.headers.get("cache-control")).toBe("private, no-store");
  });

  it("never turns a Basic header or a forged user header into access", async () => {
    expect((await call("issue", { "x-dev-web-user": "dev-user" })).status).toBe(403);
    const denied = await call("check", { authorization: "Basic ZGV2OmZha2U=" });
    expect(denied.status).toBe(401);
    expect(denied.headers.get("www-authenticate")).toBeNull();
  });

  it("redirects only document navigation, preserving the deep link", async () => {
    const denied = await call("required", { "sec-fetch-mode": "navigate", "sec-fetch-dest": "document", "x-dev-web-return-to": "/ru/app/chats?model=example" });
    expect(denied.status).toBe(303);
    expect(denied.headers.get("location")).toBe("/__dev/login?returnTo=%2Fru%2Fapp%2Fchats%3Fmodel%3Dexample");
    expect(denied.headers.get("www-authenticate")).toBeNull();
  });

  it.each([{}, { "accept": "text/html" }, { "sec-fetch-mode": "cors", "sec-fetch-dest": "empty" }])("does not send background requests to the browser password prompt", async headers => {
    const denied = await call("required", headers as Record<string, string>);
    expect(denied.status).toBe(401);
    expect(denied.headers.get("location")).toBeNull();
    expect(denied.headers.get("www-authenticate")).toBeNull();
    expect(await denied.json()).toEqual({ error: { code: "DEV_ACCESS_REQUIRED", message: "DEV access has expired. Reload the page to sign in." } });
  });

  it("is unavailable without DEV configuration and rejects unknown actions", async () => {
    expect((await call("unknown", issuer)).status).toBe(404);
    vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", "");
    expect((await call("issue", issuer)).status).toBe(404);
    expect((await call("check")).status).toBe(404);
  });
});
