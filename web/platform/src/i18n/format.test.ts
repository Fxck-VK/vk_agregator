import { describe, expect, it } from "vitest";

import { formatMessage, formatPlural, formatNumber, formatCurrency, formatDate } from "./format";
import { resolveLocale, getLocaleDirection } from "./locales";

describe("locale formatting", () => {
  it("validates preferences and falls back to Russian", () => {
    expect(resolveLocale("en")).toBe("en");
    expect(resolveLocale("../../en")).toBe("ru");
    expect(resolveLocale(undefined)).toBe("ru");
    expect(getLocaleDirection("en")).toBe("ltr");
    expect(getLocaleDirection("ar")).toBe("rtl");
  });

  it("substitutes named parameters without treating user values as templates", () => {
    expect(formatMessage("Selected: {name}", { name: "{count}<script>" })).toBe("Selected: {count}<script>");
    expect(() => formatMessage("Selected: {name}", {})).toThrow("name");
  });

  it("supports locale-specific plural forms including zero and fractions", () => {
    const forms = { one: "{count} файл", few: "{count} файла", many: "{count} файлов", other: "{count} файла" };
    expect([0, 1, 2, 5, 21].map(count => formatPlural("ru", count, forms)))
      .toEqual(["0 файлов", "1 файл", "2 файла", "5 файлов", "21 файл"]);
    expect(formatPlural("en", 1, { one: "{count} file", other: "{count} files" })).toBe("1 file");
    expect(formatPlural("en", 1.5, { one: "{count} file", other: "{count} files" })).toBe("1.5 files");
  });

  it("formats dates and values per request without converting currency or changing numbers", () => {
    expect(formatNumber("en", 1234.5)).toBe("1,234.5");
    expect(formatNumber("ru", 1234.5)).toMatch(/1\s234,5/);
    expect(formatCurrency("en", 199, "RUB")).toContain("199");
    expect(formatDate("en", "2026-09-16T00:00:00Z", { timeZone: "UTC", month: "long" })).toBe("September");
    expect(formatDate("ru", "2026-09-16T00:00:00Z", { timeZone: "UTC", month: "long" })).toBe("сентябрь");
  });
});
