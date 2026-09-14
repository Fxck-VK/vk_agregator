import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadPaymentProducts, createPayment, getPayment, readPendingPayment, savePendingPayment } from "./payments";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
const id = "12345678-1234-4000-8000-123456789012";
const payment = { id, status: "waiting_for_user", amount: 470000, currency: "RUB", credits: 10000, confirmation_url: "https://yoomoney.ru/checkout/payments/v2/contract?orderId=test" };
describe("web payments", () => {
 beforeEach(() => { vi.clearAllMocks(); sessionStorage.clear(); });
 it("loads actual server prices without hardcoded packages", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [{ code: "server-package", amount: 12345, currency: "RUB", credits: 777 }], checkout_available: true }));
  expect((await loadPaymentProducts()).items[0]).toEqual({ code: "server-package", amount: 12345, currency: "RUB", credits: 777 });
 });
 it("sends only the product with a stable idempotency key", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(payment));
  await createPayment({ product_code: "server-package" }, id);
  const [, init] = vi.mocked(webBrowserMutation).mock.calls[0];
  expect(new Headers(init.headers).get("X-Idempotency-Key")).toBe(id);
  expect(JSON.parse(init.body as string)).toEqual({ product_code: "server-package" });
 });
 it.each(["https://evil.test/pay", "javascript:alert(1)", "https://yoomoney.ru.evil.test/", "https://user:pass@yoomoney.ru/", "http://yoomoney.ru/"])("rejects untrusted checkout %s", async (url) => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ ...payment, confirmation_url: url }));
  await expect(getPayment(id)).rejects.toThrow();
 });
 it("persists only the local payment identifier for return and refresh", () => {
  savePendingPayment(id);
  expect(readPendingPayment()).toBe(id);
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBe(id);
  sessionStorage.setItem("neirohub:pending-payment", "untrusted");
  expect(readPendingPayment()).toBeNull();
 });
});
