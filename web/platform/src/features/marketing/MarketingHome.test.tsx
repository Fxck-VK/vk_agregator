import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ useRouter: vi.fn(() => ({ push: vi.fn() })) }));
vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();

import { LocaleProvider } from "@/i18n/LocaleProvider";
import { MarketingHome } from "./MarketingHome";

describe("MarketingHome", () => {
  it("keeps Epifera branding and sends primary actions to the local workspace", () => {
    render(<MarketingHome />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Креативная платформа");
    expect(screen.getAllByRole("link", { name: "Начать создавать" })[0]).toHaveAttribute("href", "/ru/app");
    expect(within(screen.getByRole("banner")).getByRole("link", { name: "Войти" })).toHaveAttribute("href", "/ru/login");
    expect(document.querySelector('a[href*="magnific.com"]')).toBeNull();
    expect(screen.getByRole("link", { name: "Место, где идеи обретают форму" })).toHaveAttribute("href", "#creative-suite");
    expect(screen.getByRole("link", { name: "Ваше пространство для творчества с ИИ" })).toHaveAttribute("href", "/ru/app/models");
  });

  it("switches the product showcase with accessible keyboard tabs", () => {
    render(<MarketingHome />);
    const categories = screen.getByRole("tablist", { name: "Категории творческого пакета" });
    const image = within(categories).getByRole("tab", { name: "Изображения" });
    fireEvent.keyDown(image, { key: "ArrowRight" });
    expect(within(categories).getByRole("tab", { name: "Видео" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel", { name: "Видео" })).toHaveTextContent("Движение начинается с идеи");
  });

  it("filters tool cards without navigating away or starting a paid job", () => {
    render(<MarketingHome />);
    fireEvent.click(within(screen.getByRole("region", { name: "Выбрать инструмент, начать создавать" })).getByRole("button", { name: "Аудио" }));
    const tools = screen.getByRole("region", { name: "Инструменты для творчества" });
    expect(within(tools).getByRole("link", { name: /Генератор музыки/ })).toHaveAttribute("href", "/ru/app/music");
    expect(within(tools).queryByRole("link", { name: /Генератор изображений/ })).not.toBeInTheDocument();
  });

  it("closes the navigation disclosure with Escape and restores focus", () => {
    render(<MarketingHome />);
    const menu = within(screen.getByRole("banner")).getByRole("button", { name: "Творческий пакет" });
    fireEvent.click(menu);
    expect(menu).toHaveAttribute("aria-expanded", "true");
    fireEvent.keyDown(menu, { key: "Escape" });
    expect(menu).toHaveAttribute("aria-expanded", "false");
    expect(menu).toHaveFocus();
  });

  it("closes a hover-style disclosure with Escape even when focus is outside the header", () => {
    render(<MarketingHome />);
    const menu = within(screen.getByRole("banner")).getByRole("button", { name: "Творческий пакет" });
    fireEvent.click(menu);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(menu).toHaveAttribute("aria-expanded", "false");
    expect(menu).toHaveFocus();
  });

  it("uses the current locale for copy and internal destinations", () => {
    render(<LocaleProvider locale="en"><MarketingHome /></LocaleProvider>);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("A creative platform");
    expect(within(screen.getByRole("banner")).getByRole("link", { name: "Log in" })).toHaveAttribute("href", "/en/login");
    expect(screen.getAllByRole("link", { name: "Start creating" })[0]).toHaveAttribute("href", "/en/app");
  });

  it("restores the mobile menu trigger when Escape closes its navigation", () => {
    render(<MarketingHome />);
    const mobileMenu = screen.getByRole("button", { name: "Открыть меню" });
    fireEvent.click(mobileMenu);
    fireEvent.keyDown(mobileMenu, { key: "Escape" });
    expect(mobileMenu).toHaveAttribute("aria-expanded", "false");
    expect(mobileMenu).toHaveFocus();
  });

  it("closes the nested suite menu when the mobile navigation is toggled off", () => {
    render(<MarketingHome />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть меню" }));
    const suiteMenu = within(screen.getByRole("banner")).getByRole("button", { name: "Творческий пакет" });
    fireEvent.click(suiteMenu);
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));
    expect(suiteMenu).toHaveAttribute("aria-expanded", "false");
    expect(document.getElementById("creative-suite-menu")).toBeNull();
  });

  it("opens a local searchable tool palette and supports keyboard selection", () => {
    render(<MarketingHome />);
    fireEvent.click(screen.getByRole("button", { name: /Поиск или создание/ }));
    const dialog = screen.getByRole("dialog", { name: "Поиск или создание" });
    const search = within(dialog).getByRole("searchbox");
    expect(search).toHaveFocus();
    fireEvent.change(search, { target: { value: "музык" } });
    const music = within(dialog).getByRole("link", { name: /Генератор музыки/ });
    expect(music).toHaveAttribute("href", "/ru/app/music");
    expect(within(dialog).queryByRole("link", { name: /Генератор видео/ })).not.toBeInTheDocument();
    fireEvent.keyDown(search, { key: "ArrowDown" });
    expect(music).toHaveFocus();
    fireEvent.change(search, { target: { value: "no-such-tool-12345" } });
    expect(within(dialog).getByRole("status")).toHaveTextContent("Ничего не найдено");
  });

  it("opens an inline video introduction instead of navigating to an anchor", () => {
    render(<MarketingHome />);
    fireEvent.click(screen.getByRole("button", { name: "Почему Epifera?" }));
    const dialog = screen.getByRole("dialog", { name: "Почему Epifera?" });
    expect(dialog.querySelector("video")).toHaveAttribute("controls");
    expect(within(dialog).getByRole("link", { name: "Начать создавать" })).toHaveAttribute("href", "/ru/app");
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByRole("dialog", { name: "Почему Epifera?" })).toBeInTheDocument();
  });

  it("gives quick categories their own panel and does not duplicate the inner tabs", async () => {
    render(<MarketingHome />);
    fireEvent.click(within(screen.getByRole("tablist", { name: "С чего начать" })).getByRole("tab", { name: "Видео" }));
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Движение начинается с идеи");
    await waitFor(() => expect(document.querySelector('video[src$="creative-suite-video.webm"]')).not.toBeNull());
    expect(screen.getAllByRole("tablist")).toHaveLength(1);
  });

  it("returns focus and scrolling after the search dialog is closed", () => {
    render(<MarketingHome />);
    const trigger = screen.getByRole("button", { name: /Поиск или создание/ });
    trigger.focus(); fireEvent.click(trigger);
    expect(document.body.style.overflow).toBe("hidden");
    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("marketing-dialog-backdrop"), { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(document.body.style.overflow).not.toBe("hidden");
  });

  it("lets visitors jump directly between inspiration slides", () => {
    render(<MarketingHome />);
    fireEvent.click(screen.getByRole("button", { name: "Показать пример 3" }));
    expect(screen.getByRole("heading", { name: "История начинается с вас" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Показать пример 3" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Следующий пример" }));
    expect(screen.getByRole("button", { name: "Показать пример 1" })).toHaveAttribute("aria-pressed", "true");
  });
});
