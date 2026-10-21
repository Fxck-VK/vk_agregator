import { describe, expect, it } from "vitest";

import { toBrandFilePrefix } from "./brand";

describe("brand download filename prefix", () => {
  it.each([
    ["Freesia", "freesia"],
    ["  My Brand  ", "my-brand"],
    ["Пример & Co", "пример-co"],
    ['Freesia<>:"/\\|?*\u0000', "freesia"],
    ["Cafe\u0301", "café"],
    ["", "download"],
    ["🌸 / ..", "download"],
  ])("converts %j to a safe filename prefix", (name, expected) => {
    expect(toBrandFilePrefix(name)).toBe(expected);
  });
});
