import { afterEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { cancelSubscription, createSubscriptionCheckout, loadSubscription, subscriptionStateSchema } from "./subscriptions";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
afterEach(() => vi.resetAllMocks());
const state = { subscription: null, balances: [], checkout_available: false, recurring_available: false, trial_used: false };
const key = "10000000-0000-4000-8000-000000000001";
describe("account subscription transport", () => {
  it("accepts the Go API lowercase rub currency for subscription checkout", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ id: key, status: "waiting_for_user", amount: 19900, currency: "rub", credits: 400, confirmation_url: "https://yoomoney.ru/checkout/test" }));
    expect((await createSubscriptionCheckout({ plan_code: "lite", auto_renew: true }, key)).currency).toBe("RUB");
  });
  it("reads current account state without identity headers or caches", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(state));
    expect(await loadSubscription()).toEqual(state);
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/subscription", expect.objectContaining({ cache: "no-store" }));
  });
  it("sends only the plan and explicit recurring choice with stable idempotency", async () => {
    const payment = { id: key, status: "canceled", amount: 19900, currency: "RUB", credits: 400 };
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(payment));
    expect((await createSubscriptionCheckout({ plan_code: "lite", auto_renew: false }, key)).status).toBe("canceled");
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/subscription/checkout", expect.objectContaining({ method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": key }, body: '{"plan_code":"lite","auto_renew":false}' }));
  });
  it("sends the accepted retry policy version with explicit recurring consent", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ id: key, status: "canceled", amount: 19900, currency: "RUB", credits: 400 }));
    await createSubscriptionCheckout({ plan_code: "lite", auto_renew: true, retry_policy_version: 1 }, key);
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/subscription/checkout", expect.objectContaining({
      headers: { "Content-Type": "application/json", "X-Idempotency-Key": key },
      body: '{"plan_code":"lite","auto_renew":true,"retry_policy_version":1}',
    }));
  });
  it.each([undefined, 0])("preserves legacy recurring requests with retry version %s", async retryPolicyVersion => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ id: key, status: "canceled", amount: 19900, currency: "RUB", credits: 400 }));
    const request = { plan_code: "lite", auto_renew: true, ...(retryPolicyVersion === undefined ? {} : { retry_policy_version: retryPolicyVersion }) };
    await createSubscriptionCheckout(request, key);
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/subscription/checkout", expect.objectContaining({ body: JSON.stringify(request) }));
  });
  it.each([0, 1])("rejects retry version %s without recurring consent", async retryPolicyVersion => {
    await expect(createSubscriptionCheckout({ plan_code: "lite", auto_renew: false, retry_policy_version: retryPolicyVersion }, key)).rejects.toThrow();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
  it.each([
    { retry_policy_version: 2 }, { retry_policy_version: -1 }, { retry_policy_version: "1" },
    { retry_policy_version: 1, window_days: 30 }, { retry_policy_version: 1, interval_hours: 1 }, { retry_policy_version: 1, max_attempts: 99 },
  ])("rejects unsupported or client-controlled retry policy %j", async policy => {
    await expect(createSubscriptionCheckout({ plan_code: "lite", auto_renew: true, ...policy } as never, key)).rejects.toThrow();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
  it("cancels through the same cookie/CSRF mutation boundary", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(state));
    expect(await cancelSubscription()).toEqual(state);
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/subscription/cancel", expect.objectContaining({ method: "POST", body: "{}" }));
  });
  it("rejects hidden provider fields and negative reservations", () => {
    expect(subscriptionStateSchema.safeParse({ ...state, payment_method_id: "private" }).success).toBe(false);
    expect(subscriptionStateSchema.safeParse({ ...state, balances: [{ kind: "subscription", available: 0, reserved: -1 }] }).success).toBe(false);
  });
  it("preserves authoritative signed legacy adjustments without enabling checkout", async () => {
    const adjusted = { ...state, balances: [{ kind: "legacy", available: -25, reserved: 0 }, { kind: "purchased", available: 10, reserved: 0 }] };
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(adjusted));
    const result = await loadSubscription();
    expect(result).toEqual(adjusted);
    expect(result.balances.reduce((total, lot) => total + lot.available, 0)).toBe(-15);
    expect(result.checkout_available).toBe(false);
  });
  it("rejects a client-injected price and unsafe payment confirmation URL", async () => {
    await expect(createSubscriptionCheckout({ plan_code: "lite", auto_renew: false, amount: 1 } as never, key)).rejects.toThrow();
    expect(webBrowserMutation).not.toHaveBeenCalled();
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ id: key, status: "waiting_for_user", amount: 19900, currency: "RUB", credits: 400, confirmation_url: "https://evil.example/pay" }));
    await expect(createSubscriptionCheckout({ plan_code: "lite", auto_renew: true }, key)).rejects.toThrow();
  });
});
