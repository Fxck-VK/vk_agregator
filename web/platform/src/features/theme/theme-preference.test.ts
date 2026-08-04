import { afterEach, describe, expect, it, vi } from "vitest";

import {
  applyThemePreference,
  readThemePreference,
  themeStorageKey,
} from "./theme-preference";

describe("theme preference", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("falls back to system for absent or malformed storage", () => {
    expect(readThemePreference()).toBe("system");

    localStorage.setItem(themeStorageKey, "unknown");

    expect(readThemePreference()).toBe("system");
  });

  it.each(["system", "light", "dark"] as const)("applies and persists %s", (preference) => {
    applyThemePreference(preference);

    expect(document.documentElement).toHaveAttribute("data-theme", preference);
    expect(localStorage.getItem(themeStorageKey)).toBe(preference);
    expect(readThemePreference()).toBe(preference);
  });

  it("falls back safely when browser storage is unavailable", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementationOnce(() => {
      throw new DOMException("Storage unavailable");
    });

    expect(readThemePreference()).toBe("system");

    vi.spyOn(Storage.prototype, "setItem").mockImplementationOnce(() => {
      throw new DOMException("Storage unavailable");
    });

    expect(() => applyThemePreference("light")).not.toThrow();
    expect(document.documentElement).toHaveAttribute("data-theme", "light");
  });
});
