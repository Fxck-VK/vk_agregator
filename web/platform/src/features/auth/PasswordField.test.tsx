import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { PasswordField } from "./PasswordField";

it("keeps login passwords masked and reveals them only through the explicit control", () => {
  const onChange = vi.fn();
  render(<PasswordField id="login-password" value="Fictitious#42" onChange={onChange} />);
  const input = screen.getByLabelText("Пароль");
  expect(input).toHaveAttribute("type", "password");
  expect(input).toHaveAttribute("autocomplete", "current-password");
  expect(screen.queryByRole("meter")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Показать пароль" }));
  expect(input).toHaveAttribute("type", "text");
  fireEvent.change(input, { target: { value: "Changed#42" } });
  expect(onChange).toHaveBeenCalledWith("Changed#42");
  fireEvent.click(screen.getByRole("button", { name: "Скрыть пароль" }));
  expect(input).toHaveAttribute("type", "password");
});

it("shows Caps Lock guidance from keyboard state and clears it on blur", () => {
  render(<PasswordField id="login-password" value="" onChange={vi.fn()} />);
  const input = screen.getByLabelText("Пароль");
  fireEvent.keyUp(input, { key: "A", modifierCapsLock: true });
  expect(screen.getByText("Включён Caps Lock")).toBeInTheDocument();
  expect(input).toHaveAttribute("aria-describedby", "login-password-caps-lock");
  fireEvent.blur(input);
  expect(screen.queryByText("Включён Caps Lock")).not.toBeInTheDocument();
});

it("uses creation strength only when requested and combines accessible descriptions", () => {
  render(<PasswordField id="new-password" label="Новый пароль" value="Longer#42Password" onChange={vi.fn()} autoComplete="new-password" showStrength hint="Подсказка" />);
  const input = screen.getByLabelText("Новый пароль");
  expect(input).toHaveAttribute("autocomplete", "new-password");
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "3");
  expect(input.getAttribute("aria-describedby")).toContain("new-password-strength");
  expect(input.getAttribute("aria-describedby")).toContain("new-password-hint");
});
