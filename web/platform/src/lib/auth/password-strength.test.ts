import { expect, it } from "vitest";
import { passwordStrength } from "./password-strength";

it.each([
  ["", 0], ["tiny", 1], ["1234567890", 1], ["Password123!", 1], ["пароль123!", 1], ["Aa1!Aa1!Aa1!", 1],
  ["BooksAndMaps", 2], ["Sunset#42River!", 3], ["violet meadow quiet river", 3], ["a".repeat(257), 1],
] as const)("classifies only local test input (%#)", (password, expected) => {
  expect(passwordStrength(password)).toBe(expected);
});
it("does not promote the email account name or repeated Unicode to strong", () => {
  expect(passwordStrength("OwnerName123!", "owner.name@example.test")).toBe(1);
  expect(passwordStrength("Ж".repeat(130))).toBe(1);
});
