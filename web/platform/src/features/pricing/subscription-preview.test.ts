import { expect, it } from "vitest";
import { subscriptionPlanDiscountPercent } from "./subscription-preview";

it.each([undefined, "", "0", "-1200", "invalid", "Infinity", "999", "900", "999.01"])("omits an absent or ineffective Pro discount for %s", previous => {
  expect(subscriptionPlanDiscountPercent("pro", previous)).toBeNull();
});

it("does not overstate a fractional discount and preserves an exact whole percentage", () => {
  expect(subscriptionPlanDiscountPercent("pro", "1200")).toBe(16);
  expect(subscriptionPlanDiscountPercent("pro", "1998")).toBe(50);
});
