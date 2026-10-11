import { afterEach, expect, it } from "vitest";
import { beginBrowserSession, browserSessionSnapshot, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { readReferralCode, rememberReferralCode } from "./referral-code-cache";

afterEach(endBrowserSession);

it("clears the code without mounted subscribers and rejects a late response from an earlier login", () => {
  beginBrowserSession("account-a");
  const firstSession = browserSessionSnapshot();
  expect(rememberReferralCode("account-a", "WEB2345678", firstSession)).toBe(true);
  expect(readReferralCode("account-a")).toBe("WEB2345678");
  expect(readReferralCode("account-b")).toBeNull();
  endBrowserSession();
  beginBrowserSession("account-a");
  expect(readReferralCode("account-a")).toBeNull();
  expect(rememberReferralCode("account-a", "WEB2345678", firstSession)).toBe(false);
  expect(readReferralCode("account-a")).toBeNull();
});

it("does not write an unverified owner or malformed code into the current account's cache", () => {
  beginBrowserSession("account-a");
  const session = browserSessionSnapshot();
  expect(rememberReferralCode("account-b", "WEB2345678", session)).toBe(false);
  expect(rememberReferralCode("account-a", "../../outside", session)).toBe(false);
  expect(readReferralCode("account-a")).toBeNull();
});
