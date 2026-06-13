import { describe, expect, test } from "vitest";
import { safeExternalHttpsUrl } from "./openExternalUrl";

describe("safeExternalHttpsUrl", () => {
  test("allows only absolute https URLs", () => {
    expect(safeExternalHttpsUrl("https://yookassa.example/pay?id=1")).toBe("https://yookassa.example/pay?id=1");
    expect(safeExternalHttpsUrl("http://yookassa.example/pay")).toBeNull();
    expect(safeExternalHttpsUrl("javascript:alert(1)")).toBeNull();
    expect(safeExternalHttpsUrl("/payments/return")).toBeNull();
    expect(safeExternalHttpsUrl("")).toBeNull();
  });
});
