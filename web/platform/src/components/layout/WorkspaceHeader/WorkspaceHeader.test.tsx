import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
}));

vi.mock("@/features/models/WorkspaceModelSelector/WorkspaceModelSelector", () => ({
  WorkspaceModelSelector: () => <button type="button">Nano Banana 2</button>,
}));

import { usePathname } from "next/navigation";

import { loadPaymentProducts } from "@/features/payments/payments";
vi.mock("@/features/payments/payments", async (original) => ({ ...await original<typeof import("@/features/payments/payments")>(), loadPaymentProducts: vi.fn() }));

import { WorkspaceHeader } from "./WorkspaceHeader";

describe("WorkspaceHeader", () => {
  beforeEach(() => {
    vi.mocked(usePathname).mockReturnValue("/app/profile");
    sessionStorage.clear();
    vi.mocked(loadPaymentProducts).mockResolvedValue({ checkout_available: true, items: [
      { code: "20000", credits: 20000, amount: 920000, currency: "RUB" },
      { code: "10000", credits: 10000, amount: 470000, currency: "RUB" },
      { code: "3000", credits: 3000, amount: 150000, currency: "RUB" },
      { code: "1500", credits: 1500, amount: 75000, currency: "RUB" },
      { code: "800", credits: 800, amount: 40000, currency: "RUB" },
    ] });
  });

  afterEach(() => {
    cleanup();
  });

  it("names the fixed header after the profile route", () => {
    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("banner", { name: "Профиль" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Nano Banana 2" })).toBeInTheDocument();
    expect(screen.queryByText("Профиль")).toBeNull();
  });

  it("keeps the model selector visible throughout the Inspiration section", () => {
    vi.mocked(usePathname).mockReturnValue("/app/inspiration/example");

    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("banner", { name: "Вдохновение" })).toBeInTheDocument();
    expect(screen.queryByText("Вдохновение")).toBeNull();
    expect(screen.getByRole("button", { name: "Nano Banana 2" })).toBeInTheDocument();
  });

  it("does not treat a similarly prefixed route as the Inspiration section", () => {
    vi.mocked(usePathname).mockReturnValue("/app/inspiration-tools");

    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("button", { name: "Nano Banana 2" })).toBeInTheDocument();
  });

  it("shows an explicit guest action instead of a balance", () => {
    render(
      <WorkspaceHeader balance={null} trailingAction={<a href="/login">Войти</a>} />,
    );

    expect(screen.getByRole("link", { name: "Войти" })).toHaveAttribute("href", "/login");
    expect(screen.queryByTestId("workspace-balance")).not.toBeInTheDocument();
  });

  it("renders the shared credit-star artwork instead of a text glyph", () => {
    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByTestId("workspace-balance")).toHaveAttribute(
      "aria-label",
      "Пополнить баланс токенов. Текущий баланс: 104 звезды",
    );
    expect(screen.getByTestId("credit-star-icon")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-balance")).not.toHaveTextContent("★");
  });

  it("opens token top-up from the balance and selects a package", async () => {
    render(<WorkspaceHeader balance={104} />);

    const balanceButton = screen.getByRole("button", {
      name: "Пополнить баланс токенов. Текущий баланс: 104 звезды",
    });
    expect(balanceButton).toHaveAttribute("aria-haspopup", "dialog");
    expect(balanceButton).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(balanceButton);

    const dialog = screen.getByRole("dialog", { name: "Пополнить баланс токенов" });
    expect(balanceButton).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Выберите подходящий пакет токенов.")).toBeInTheDocument();
    expect(await screen.findAllByRole("radio")).toHaveLength(5);
    expect(screen.getByRole("radio", { name: "20 000 токенов за 9 200 ₽" })).toBeChecked();
    expect(screen.getByRole("button", { name: "Купить за 9 200 ₽" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "10 000 токенов за 4 700 ₽" }));

    expect(screen.getByRole("radio", { name: "10 000 токенов за 4 700 ₽" })).toBeChecked();
    expect(screen.getByRole("button", { name: "Купить за 4 700 ₽" })).toBeInTheDocument();
    expect(dialog).toBeInTheDocument();
  });

  it("closes token top-up from its close control", () => {
    render(<WorkspaceHeader balance={104} />);

    fireEvent.click(screen.getByTestId("workspace-balance"));
    fireEvent.click(screen.getByRole("button", { name: "Закрыть пополнение баланса" }));

    const backdrop = screen.getByTestId("token-top-up-backdrop");
    expect(backdrop).toHaveAttribute("data-state", "closing");
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", { name: "Пополнить баланс токенов" })).toBeNull();
  });

  it("shows a non-navigating tariff button beside the authenticated balance", () => {
    render(<WorkspaceHeader balance={104} />);

    const tariffButton = screen.getByRole("button", { name: "Выбрать тариф" });
    expect(tariffButton).toHaveAttribute("type", "button");
    expect(tariffButton).not.toHaveAttribute("href");
  });

  it("opens the subscription plans dialog and closes it with Escape", () => {
    render(<WorkspaceHeader balance={104} />);

    fireEvent.click(screen.getByRole("button", { name: "Выбрать тариф" }));

    expect(screen.getByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeInTheDocument();
    for (const planName of ["Lite", "Start+", "Pro", "Ultima", "Elite", "Командный тариф"]) {
      expect(screen.getByRole("heading", { name: planName })).toBeInTheDocument();
    }
    expect(screen.getAllByRole("button", { name: "Активировать подписку" })).toHaveLength(5);
    expect(screen.getByRole("button", { name: "Связаться с менеджером" })).not.toHaveAttribute("href");

    fireEvent.keyDown(window, { key: "Escape" });

    const escapeBackdrop = screen.getByTestId("subscription-plans-backdrop");
    expect(escapeBackdrop).toHaveAttribute("data-state", "closing");
    fireEvent.animationEnd(escapeBackdrop, { animationName: "modalBackdropOut" });

    expect(screen.queryByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeNull();
  });

  it("closes the subscription plans dialog from its close control and backdrop", () => {
    render(<WorkspaceHeader balance={104} />);

    const tariffButton = screen.getByRole("button", { name: "Выбрать тариф" });
    fireEvent.click(tariffButton);
    fireEvent.click(screen.getByRole("button", { name: "Закрыть тарифы" }));
    fireEvent.animationEnd(screen.getByTestId("subscription-plans-backdrop"), { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeNull();

    fireEvent.click(tariffButton);
    const backdrop = screen.getByTestId("subscription-plans-backdrop");
    fireEvent.mouseDown(backdrop);
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeNull();
  });
});
