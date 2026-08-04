import type { AnchorHTMLAttributes, ReactNode } from "react";

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ children, onClick, ...props }: AnchorHTMLAttributes<HTMLAnchorElement> & { children: ReactNode }) => (
    <a
      {...props}
      onClick={(event) => {
        event.preventDefault();
        onClick?.(event);
      }}
    >
      {children}
    </a>
  ),
}));

import { PublicSidebar } from "./PublicSidebar";

describe("PublicSidebar", () => {
  afterEach(() => cleanup());

  it("shows the approved public navigation without private account data", () => {
    render(<PublicSidebar />);

    expect(screen.getByRole("link", { name: "NeiroHub" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Новый чат" })).toHaveAttribute("href", "/login?next=/app/chats");
    expect(screen.getByRole("link", { name: "Мои файлы" })).toHaveAttribute("href", "/login?next=/app/files");
    expect(screen.getByRole("link", { name: "Все нейросети" })).toHaveAttribute("href", "/login?next=/app/models");
    expect(screen.getByRole("link", { name: "Вдохновение" })).toHaveAttribute("href", "/login?next=/app/inspiration");
    expect(screen.queryByText(/недавние чаты|аккаунт/i)).not.toBeInTheDocument();
  });

  it("opens and closes the mobile drawer with the keyboard", () => {
    render(<PublicSidebar />);

    const trigger = screen.getByRole("button", { name: "Открыть меню" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("dialog", { name: "Основная навигация" })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "Escape" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("closes the mobile drawer after choosing a destination", () => {
    render(<PublicSidebar />);

    const trigger = screen.getByRole("button", { name: "Открыть меню" });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("link", { name: "Все нейросети" }));

    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});
