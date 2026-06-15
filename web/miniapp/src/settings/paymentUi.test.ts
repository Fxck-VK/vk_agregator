import { describe, expect, it } from "vitest";
import type { PaymentProduct } from "../api/client";
import { paymentHistoryCountLabel, selectedPaymentProduct } from "./paymentUi";

const products: PaymentProduct[] = [
  { id: "1", code: "starter", title: "Starter", amount: 9900, currency: "RUB", credits: 100, price_version: 1 },
  { id: "2", code: "pro", title: "Pro", amount: 19900, currency: "RUB", credits: 250, price_version: 1 },
];

describe("payment UI helpers", () => {
  it("selects product by code and falls back to the first catalog item", () => {
    expect(selectedPaymentProduct(products, "pro")?.code).toBe("pro");
    expect(selectedPaymentProduct(products, "missing")?.code).toBe("starter");
    expect(selectedPaymentProduct([], "pro")).toBeNull();
  });

  it("formats payment history counters", () => {
    expect(paymentHistoryCountLabel(0)).toBe("Нет платежей");
    expect(paymentHistoryCountLabel(1)).toBe("1 платеж");
    expect(paymentHistoryCountLabel(2)).toBe("2 платежа");
    expect(paymentHistoryCountLabel(5)).toBe("5 платежей");
    expect(paymentHistoryCountLabel(21)).toBe("21 платеж");
    expect(paymentHistoryCountLabel(112)).toBe("112 платежей");
  });
});
