import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SubscriptionPlansDialog } from "./SubscriptionPlansDialog";

afterEach(cleanup);

function openPromo() {
  const onClose = vi.fn();
  render(<SubscriptionPlansDialog onClose={onClose} />);
  const trigger = screen.getByRole("button", { name: "Активировать промокод" });
  trigger.focus();
  fireEvent.click(trigger);
  return { onClose, trigger };
}

describe("subscription promo dialog", () => {
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
