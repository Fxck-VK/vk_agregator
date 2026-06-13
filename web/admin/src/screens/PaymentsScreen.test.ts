import { describe, expect, it } from "vitest";
import type { OperatorPaymentIntentDTO } from "../api/payments";
import { paymentActionEnabled, paymentActionPath } from "./PaymentsScreen";

function intent(overrides: Partial<OperatorPaymentIntentDTO> = {}): OperatorPaymentIntentDTO {
  return {
    display_id: "pay_safe_display",
    action_ref: "opact_v1_opaque/action+ref",
    user_ref: "user_safe_display",
    status: "provider_pending",
    amount: 9900,
    currency: "RUB",
    credits: 100,
    provider: "mock",
    confirmation_state: "available",
    capture_state: "open",
    cancel_state: "cancelable_by_operator_endpoint",
    refund_state: "unavailable",
    stale: false,
    created_at: "2026-06-13T00:00:00Z",
    updated_at: "2026-06-13T00:00:00Z",
    ...overrides,
  };
}

describe("payment operator action helpers", () => {
  it("uses the opaque action ref in mutation paths", () => {
    const path = paymentActionPath("opact_v1_opaque/action+ref", "refund");

    expect(path).toBe("/billing/payment-intents/opact_v1_opaque%2Faction%2Bref/refund");
    expect(path).not.toContain("pay_safe_display");
  });

  it("enables only backend-supported payment action states", () => {
    const pending = intent();
    const succeeded = intent({
      status: "succeeded",
      capture_state: "captured",
      cancel_state: "terminal",
      refund_state: "eligible_policy_check_required",
    });
    const closed = intent({
      action_ref: undefined,
      status: "canceled",
      capture_state: "closed_without_capture",
      cancel_state: "canceled",
      refund_state: "unavailable",
    });

    expect(paymentActionEnabled(pending, "sync")).toBe(true);
    expect(paymentActionEnabled(pending, "cancel")).toBe(true);
    expect(paymentActionEnabled(pending, "refund")).toBe(false);
    expect(paymentActionEnabled(succeeded, "sync")).toBe(false);
    expect(paymentActionEnabled(succeeded, "cancel")).toBe(false);
    expect(paymentActionEnabled(succeeded, "refund")).toBe(true);
    expect(paymentActionEnabled(closed, "sync")).toBe(false);
    expect(paymentActionEnabled(closed, "cancel")).toBe(false);
    expect(paymentActionEnabled(closed, "refund")).toBe(false);
  });
});
