import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { LoginForm } from "./LoginForm/LoginForm";
import { previewAuthMethods } from "@/lib/auth/methods";
import { ru } from "@/i18n/ru";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
afterEach(cleanup);

it("puts linked continuation terms on registration only, without an acceptance checkbox", () => {
  render(<LoginForm entry="choice" preview methods={previewAuthMethods} />);
  expect(screen.queryByRole("link", { name: "Пользовательское соглашение" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: ru.auth.minimalSignupAction }));
  expect(screen.getByText(/Продолжая, вы соглашаетесь/)).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Пользовательское соглашение" })).toHaveAttribute("href", "/ru/legal/user-agreement");
  expect(screen.getByRole("link", { name: "Политика обработки данных" })).toHaveAttribute("href", "/ru/legal/privacy-policy");
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Вернуться ко входу" }));
  expect(screen.queryByRole("link", { name: "Пользовательское соглашение" })).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "legal-notice@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: ru.auth.minimalContinue }));
  expect(screen.queryByRole("link", { name: "Пользовательское соглашение" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: ru.auth.forgot }));
  expect(screen.queryByRole("link", { name: "Пользовательское соглашение" })).not.toBeInTheDocument();
});

it("does not put terms or a dialog on ordinary social login", () => {
  render(<LoginForm entry="choice" methods={previewAuthMethods} />);
  expect(screen.queryByRole("link", { name: "Пользовательское соглашение" })).not.toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});
