import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SubscriptionPlansDialog } from "./SubscriptionPlansDialog";

const { push } = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

afterEach(() => { cleanup(); vi.clearAllMocks(); });

function openPromo() {
  const onClose = vi.fn();
  render(<SubscriptionPlansDialog onClose={onClose} />);
  const trigger = screen.getByRole("button", { name: "Активировать промокод" });
  trigger.focus();
  fireEvent.click(trigger);
  return { onClose, trigger };
}

describe("subscription promo dialog", () => {
  it("opens the Russian pricing page and closes the tariff dialog", () => {
    const onClose = vi.fn();
    render(<SubscriptionPlansDialog onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: "Подробнее о тарифах" }));
    expect(onClose).toHaveBeenCalledOnce();
    expect(push).toHaveBeenCalledExactlyOnceWith("/ru/pricing");
  });

  it("provides payment and agreement links without implying automatic renewal consent", () => {
    render(<SubscriptionPlansDialog onClose={() => {}} />);
    const agreement = screen.getByRole("link", { name: "Пользовательское соглашение" });
    expect(agreement).toHaveAttribute("href", "/ru/legal/user-agreement");
    expect(agreement).toHaveAttribute("target", "_blank");
    expect(agreement).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByRole("link", { name: "Условия оплаты и автопродления" })).toHaveAttribute("href", "/ru/legal/paid-access");
    expect(agreement.closest("p")).toHaveTextContent("Пользовательское соглашение · Условия оплаты и автопродления");
  });
  it.each([["Start+", "−13%"], ["Pro", "−16%"], ["Ultima", "−23%"], ["Elite", "−28%"]])("calculates the %s badge from the displayed prices without overstating it", (name, discount) => {
    render(<SubscriptionPlansDialog onClose={() => {}} />);
    const card = screen.getByRole("heading", { name }).closest("article")!;
    expect(within(card).getByText(discount)).toBeVisible();
  });
  it("preserves the original six-card presentation with features, badges and canonical balances", () => {
    render(<SubscriptionPlansDialog onClose={() => {}} />);
    const container = screen.getByRole("dialog", { name: "С подпиской — максимум возможностей" });
    expect(container.querySelectorAll("article")).toHaveLength(6);
    expect(screen.getByRole("heading", { name: "Командный тариф" })).toBeVisible();
    expect(screen.getByText(/Популярное/)).toBeVisible();
    expect(container.querySelectorAll("del")).toHaveLength(4);
    expect(screen.getAllByRole("button", { name: "Активировать подписку" })).toHaveLength(5);
    expect(container.querySelectorAll("li").length).toBeGreaterThan(15);
    const pro = screen.getByRole("heading", { name: "Pro" }).closest("article")!;
    expect(pro).toHaveTextContent(/2\s*100/);
    expect(screen.queryByRole("dialog", { name: "Условия подписки" })).toBeNull();
  });
  it("sends a guest directly to login without opening a second payment dialog", () => {
    const onClose = vi.fn();
    render(<SubscriptionPlansDialog onClose={onClose} />);
    const pro = screen.getByRole("heading", { name: "Pro" }).closest("article")!;
    fireEvent.click(within(pro).getByRole("button", { name: "Активировать подписку" }));
    expect(push).toHaveBeenCalledExactlyOnceWith("/ru/login");
    expect(screen.queryByRole("dialog", { name: "Условия подписки" })).toBeNull();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });
  it("focuses the input and asks a guest to sign in before checking codes", () => {
    openPromo();
    const dialog = screen.getByRole("dialog", { name: "У меня есть промокод" });
    const input = within(dialog).getByRole("textbox", { name: "Промокод" });
    const submit = within(dialog).getByRole("button", { name: "Активировать" });
    expect(input).toHaveFocus();
    expect(submit).toBeDisabled();
    fireEvent.change(input, { target: { value: "   " } });
    expect(submit).toBeDisabled();
    fireEvent.change(input, { target: { value: "TEST" } });
    expect(submit).toBeDisabled();
    fireEvent.click(submit);
    expect(within(dialog).getByRole("status")).toHaveTextContent("Войдите в аккаунт");
    expect(screen.queryByText("Выберите подходящий объём возможностей NeiroHub")).toBeNull();
  });

  it.each(["escape", "backdrop", "cross"])("closes only the promo with %s and returns focus to tariffs", (method) => {
    const { onClose, trigger } = openPromo();
    const promoBackdrop = screen.getByTestId("promo-code-backdrop");
    if (method === "escape") fireEvent.keyDown(window, { key: "Escape" });
    if (method === "backdrop") fireEvent.mouseDown(promoBackdrop);
    if (method === "cross") fireEvent.click(screen.getByRole("button", { name: "Закрыть ввод промокода" }));
    expect(promoBackdrop).toHaveAttribute("data-state", "closing");
    expect(screen.getByTestId("subscription-plans-backdrop")).toHaveAttribute("data-state", "open");
    fireEvent.animationEnd(promoBackdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", { name: "У меня есть промокод" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(document.body.style.overflow).toBe("hidden");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("keeps keyboard focus inside the promo", () => {
    openPromo();
    const input = screen.getByRole("textbox", { name: "Промокод" });
    const close = screen.getByRole("button", { name: "Закрыть ввод промокода" });
    fireEvent.keyDown(input, { key: "Tab" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(input).toHaveFocus();
  });
});
