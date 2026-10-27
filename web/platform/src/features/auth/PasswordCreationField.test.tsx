import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { PasswordCreationField } from "./PasswordCreationField";

it("shows three accessible strength states without exposing the password as text", () => {
  const props = { id: "new-password", email: "test@example.test", onChange: vi.fn() };
  const { rerender } = render(<PasswordCreationField {...props} value="" />);
  expect(screen.queryByRole("meter")).not.toBeInTheDocument();
  rerender(<PasswordCreationField {...props} value="12345678" />);
  expect(screen.getByRole("meter", { name: "Надёжность пароля" })).toHaveAttribute("aria-valuenow", "1");
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuetext", "Слабый");
  rerender(<PasswordCreationField {...props} value="BooksAndMaps" />);
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "2");
  rerender(<PasswordCreationField {...props} value="Sunset#42River!" />);
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "3");
  expect(screen.getByText("Сильный")).toBeInTheDocument();
  expect(screen.getByLabelText("Пароль")).toHaveAttribute("aria-describedby", "new-password-strength");
  expect(screen.queryByText("Sunset#42River!")).not.toBeInTheDocument();
});

it("reveals only on explicit user action and keeps the input on new-password autocomplete", () => {
  render(<PasswordCreationField id="new-password" email="test@example.test" onChange={vi.fn()} value="Sunset#42River!" />);
  const input = screen.getByLabelText("Пароль");
  expect(input).toHaveAttribute("type", "password");
  expect(input).toHaveAttribute("autocomplete", "new-password");
  fireEvent.click(screen.getByRole("button", { name: "Показать пароль" }));
  expect(input).toHaveAttribute("type", "text");
  fireEvent.click(screen.getByRole("button", { name: "Скрыть пароль" }));
  expect(input).toHaveAttribute("type", "password");
});
