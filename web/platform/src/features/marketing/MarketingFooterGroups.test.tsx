import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MarketingFooterGroups } from "./MarketingFooterGroups";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("collapses mobile footer groups without hiding the product links initially", () => {
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  render(<MarketingFooterGroups />);
  expect(screen.getByRole("button", { name: "Продукт" })).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByRole("link", { name: "Музыка" })).toBeVisible();
  expect(screen.queryByRole("link", { name: "Мои файлы" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Начать" }));
  expect(screen.getByRole("link", { name: "Мои файлы" })).toHaveAttribute("href", "/ru/app/files");
  fireEvent.click(screen.getByRole("button", { name: "Начать" }));
  expect(screen.queryByRole("link", { name: "Мои файлы" })).toBeNull();
});
