import { afterEach, expect, it, vi } from "vitest";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn() }));
import { webBrowserMutation } from "@/lib/web-api/browser";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { parsePromotionQuote, requestPromotionQuote } from "./promotions";

export const quoteFixture = { code: "FIRST15", revision: 1, percent: 15, plans: [{ plan_code: "start", base_amount: 54900, amount: 46665, currency: "RUB", stars: 1250 }] };
afterEach(() => { vi.resetAllMocks(); endBrowserSession(); });
it("normalizes the code and requests only an account-owned quote, never a client discount or price", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(quoteFixture));
  expect(await requestPromotionQuote(" first15 ")).toEqual(quoteFixture);
  expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/subscription/promo", expect.objectContaining({ method: "POST", body: JSON.stringify({ code: "FIRST15" }), cache: "no-store" }));
});
it.each(["x", "bad code", "скидка", "x".repeat(65)])("rejects invalid input without a network call: %s", async code => {
  await expect(requestPromotionQuote(code)).rejects.toThrow(); expect(webBrowserMutation).not.toHaveBeenCalled();
});
it.each([
  { percent: 100 }, { revision: 0 }, { plans: [] }, { plans: [{ ...quoteFixture.plans[0], amount: 1 }] },
  { plans: [{ ...quoteFixture.plans[0], stars: 999999 }] }, { plans: [{ ...quoteFixture.plans[0], base_amount: 50000 }] },
  { plans: [quoteFixture.plans[0], quoteFixture.plans[0]] }, { private_value: "not allowed" },
])("rejects inconsistent server quote data: %j", patch => { expect(() => parsePromotionQuote({ ...quoteFixture, ...patch })).toThrow(); });
it("rejects a different code returned by the server", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ ...quoteFixture, code: "ANOTHER" }));
  await expect(requestPromotionQuote("FIRST15")).rejects.toThrow();
});
it("discards a quote after an account change while JSON is still loading", async () => {
  beginBrowserSession("account-one"); let finish!: (value: unknown) => void;
  vi.mocked(webBrowserMutation).mockResolvedValue({ ok: true, headers: new Headers(), json: () => new Promise(resolve => { finish = resolve; }) } as Response);
  const pending = requestPromotionQuote("FIRST15"); await Promise.resolve(); await Promise.resolve();
  endBrowserSession(); beginBrowserSession("account-two"); finish(quoteFixture);
  await expect(pending).rejects.toThrow();
});
