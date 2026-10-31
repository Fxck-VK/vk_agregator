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

  it("provides actual legal links in the existing footer sentence", () => {
    render(<SubscriptionPlansDialog onClose={() => {}} />);
    const agreement = screen.getByRole("link", { name: "пользовательским соглашением" });
    expect(agreement).toHaveAttribute("href", "/ru/legal/user-agreement");
    expect(agreement).toHaveAttribute("target", "_blank");
    expect(agreement).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByRole("link", { name: "рекуррентными платежами" })).toHaveAttribute("href", "/ru/legal/paid-access");
    expect(agreement.closest("p")).toHaveTextContent("Покупая подписку, вы соглашаетесь с пользовательским соглашением и рекуррентными платежами.");
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
  it("opens payment terms only for the chosen card and returns focus without changing the tariff layout", () => {
    const onClose = vi.fn();
    render(<SubscriptionPlansDialog onClose={onClose} />);
    const pro = screen.getByRole("heading", { name: "Pro" }).closest("article")!;
    const trigger = within(pro).getByRole("button", { name: "Активировать подписку" });
    trigger.focus();
    fireEvent.click(trigger);
    const checkout = screen.getByRole("dialog", { name: "Условия подписки" });
    expect(checkout).toHaveTextContent(/Pro.*999/);
    expect(checkout).toHaveTextContent(/2\s*100/);
    expect(within(checkout).getByRole("button", { name: "Перейти к оплате" })).toBeDisabled();
    expect(within(checkout).getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByTestId("subscription-plans-backdrop")).toHaveAttribute("data-state", "open");
    fireEvent.keyDown(window, { key: "Escape" });
    const backdrop = screen.getByTestId("subscription-checkout-backdrop");
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", { name: "Условия подписки" })).toBeNull();
    expect(trigger).toHaveFocus();
    expect(onClose).not.toHaveBeenCalled();
  });
  it("focuses the input, rejects blank codes and explains unavailable activation", () => {
    openPromo();
    const dialog = screen.getByRole("dialog", { name: "У меня есть промокод" });
    const input = within(dialog).getByRole("textbox", { name: "Промокод" });
    const submit = within(dialog).getByRole("button", { name: "Активировать" });
    expect(input).toHaveFocus();
    expect(submit).toBeDisabled();
    fireEvent.change(input, { target: { value: "   " } });
    expect(submit).toBeDisabled();
    fireEvent.change(input, { target: { value: "TEST" } });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);
    expect(within(dialog).getByRole("status")).toHaveTextContent("Активация промокодов пока недоступна.");
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
