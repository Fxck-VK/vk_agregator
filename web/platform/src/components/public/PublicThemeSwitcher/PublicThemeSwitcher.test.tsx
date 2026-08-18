import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { themeStorageKey } from "@/features/theme/theme-preference";

import { PublicThemeSwitcher } from "./PublicThemeSwitcher";

const labels = {
  group: "Тема оформления",
  system: "Системная тема",
  light: "Светлая тема",
  dark: "Тёмная тема",
};

describe("PublicThemeSwitcher", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.dataset.theme = "system";
  });

  it("applies and persists the selected theme", () => {
    render(<PublicThemeSwitcher labels={labels} />);

    fireEvent.click(screen.getByRole("button", { name: labels.dark }));

    expect(document.documentElement).toHaveAttribute("data-theme", "dark");
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark");
    expect(screen.getByRole("button", { name: labels.dark })).toHaveAttribute("aria-pressed", "true");
  });
});
