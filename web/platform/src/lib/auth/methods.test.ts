import { describe, expect, it } from "vitest";
import { authMethodsSchema, safeAuthorizationURL } from "./methods";

describe("browser authentication boundary", () => {
  it("keeps registration disabled for the older backend contract", () => {
    expect(authMethodsSchema.parse({ password: true, recovery: false, email_link: false, phone_link: false, providers: [] }).registration).toBe(false);
  });
  it("accepts only known providers and no server secrets", () => {
    const valid = { registration: false, password: true, recovery: true, email_link: true, phone_link: false, providers: ["google"] };
    expect(authMethodsSchema.safeParse(valid).success).toBe(true);
    expect(authMethodsSchema.safeParse({ ...valid, providers: ["untrusted"] }).success).toBe(false);
    expect(authMethodsSchema.safeParse({ ...valid, client_secret: "synthetic" }).success).toBe(false);
  });
  it.each(["http://accounts.google.com/o/oauth2/v2/auth", "https://accounts.google.com.evil.example/o/oauth2/v2/auth", "https://user@accounts.google.com/o/oauth2/v2/auth", "https://accounts.google.com/other", "javascript:alert(1)"])("rejects a substituted authorization target: %s", value => {
    expect(safeAuthorizationURL(value, "google")).toBeNull();
  });
  it("accepts a pinned provider authorize endpoint", () => {
    expect(safeAuthorizationURL("https://accounts.google.com/o/oauth2/v2/auth?state=synthetic", "google")).toContain("accounts.google.com");
  });
});
