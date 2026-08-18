import { describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { defaultLocale, locales, parseLocale } from "../locales";
import { publicDictionaryEn } from "./en";
import { getPublicDictionary } from "./get-public-dictionary";
import { publicDictionaryRu } from "./ru";

function flattenKeys(value: unknown, prefix = ""): string[] {
  if (typeof value === "string") {
    return [prefix];
  }

  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(`Public dictionary value at "${prefix}" must be a string or object.`);
  }

  return Object.entries(value)
    .flatMap(([key, child]) => flattenKeys(child, prefix ? `${prefix}.${key}` : key))
    .sort();
}

describe("public localization contract", () => {
  it("defines Russian as the default and accepts only supported locales", () => {
    expect(locales).toEqual(["ru", "en"]);
    expect(defaultLocale).toBe("ru");
    expect(parseLocale("ru")).toBe("ru");
    expect(parseLocale("en")).toBe("en");
    expect(() => parseLocale("de")).toThrow("Unsupported locale: de");
    expect(() => parseLocale(null)).toThrow("Unsupported locale: null");
  });

  it("keeps Russian and English public dictionary keys identical", () => {
    expect(flattenKeys(publicDictionaryEn)).toEqual(flattenKeys(publicDictionaryRu));
  });

  it("loads the requested public dictionary and rejects unsupported input", async () => {
    await expect(getPublicDictionary("ru")).resolves.toEqual(publicDictionaryRu);
    await expect(getPublicDictionary("en")).resolves.toEqual(publicDictionaryEn);
    await expect(getPublicDictionary("de")).rejects.toThrow("Unsupported locale: de");
  });
});
