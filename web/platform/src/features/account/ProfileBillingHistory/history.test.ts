import { afterEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { historyPageSchema, loadBillingHistory } from "./history";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
afterEach(() => vi.resetAllMocks());
export const purchase = { id: "payment:10000000-0000-4000-8000-000000000001", kind: "topup", created_at: "2026-10-31T09:13:00Z", stars_delta: 777, status: "succeeded", money: { amount_minor: 12345, currency: "rub" } };
describe("billing history safe transport", () => {
  it("uses a bounded no-store GET and encodes the opaque cursor", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [purchase], next_cursor: null }));
    const signal = new AbortController().signal;
    expect(await loadBillingHistory("a_b-0", signal)).toEqual({ items: [purchase], next_cursor: null });
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/billing/history?limit=20&cursor=a_b-0", { cache: "no-store", signal });
  });
  it.each([
    { created_at: "bad-date" }, { created_at: "2026-02-30T09:13:00Z" },
    { stars_delta: Number.MAX_SAFE_INTEGER + 1 }, { stars_delta: 1.5 },
    { money: { amount_minor: 1, currency: "usd" } },
    { money: { amount_minor: Number.MAX_SAFE_INTEGER + 1, currency: "rub" } },
    { money: { amount_minor: 0, currency: "rub" } },
    { kind: "provider-secret" }, { status: "raw-provider-status" }, { operation: "raw-operation" },
  ])("rejects malformed financial data %j", patch => {
    expect(historyPageSchema.safeParse({ items: [{ ...purchase, ...patch }], next_cursor: null }).success).toBe(false);
  });
  it("strips undisclosed fields at every level", () => {
    const parsed = historyPageSchema.parse({ items: [{ ...purchase, metadata: { email: "private" }, provider_id: "private", reason: "private", money: { ...purchase.money, provider_payload: "private" } }], next_cursor: null, account_id: "private" });
    expect(parsed).toEqual({ items: [purchase], next_cursor: null });
  });
  it.each([{ items: [], next_cursor: "cursor" }, { items: [purchase], next_cursor: "" }, { items: [purchase], next_cursor: "a".repeat(513) }])("rejects inconsistent pagination %j", page => {
    expect(historyPageSchema.safeParse(page).success).toBe(false);
  });
  it("normalizes failed reads without consuming or exposing their body", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response("private provider failure", { status: 503 }));
    await expect(loadBillingHistory(null, new AbortController().signal)).rejects.toThrow("Billing history unavailable");
  });
});
