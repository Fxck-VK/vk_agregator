import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { presentationStyles } from "@/i18n/presentation-deck";
import { DeckAppearanceSettings, PresentationSettings, type PresentationAppearance } from "./PresentationSettings";
import { defaultDeckAppearance, resolveDeckAppearance, type DeckAppearance } from "./deck-appearance";

afterEach(cleanup);

describe("presentation appearance settings", () => {
  it("controls owned deck formatting and transitions and resets them together", () => {
    const onChange = vi.fn();
    function OwnedAppearance() {
      const [value, setValue] = useState<DeckAppearance>({
        font: "georgia", background: "dark", animation: "off", transition: "fade", colors: { background: "#252649", text: "#eeeeee" },
      });
      return <DeckAppearanceSettings value={value} onChange={appearance => { setValue(appearance); onChange(appearance); }} />;
    }
    render(<OwnedAppearance />);
    fireEvent.click(screen.getByRole("button", { name: "Настройки показа" }));
    const panel = screen.getByRole("dialog", { name: "Настройки показа" });
    expect(within(panel).getByRole("button", { name: "Georgia" })).toHaveAttribute("aria-pressed", "true");
    expect(within(panel).getByRole("button", { name: "Тёмный" })).toHaveAttribute("aria-pressed", "true");
    expect(within(panel).getByRole("button", { name: "Выключена" })).toHaveAttribute("aria-pressed", "true");
    const transition = within(panel).getByRole("toolbar", { name: "Переход" });
    expect(within(transition).getByRole("button", { name: "Плавный" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(within(transition).getByRole("button", { name: "Без перехода" }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ transition: "none" }));
    fireEvent.click(within(transition).getByRole("button", { name: "Сдвиг" }));
    expect(within(transition).getByRole("button", { name: "Сдвиг" })).toHaveAttribute("aria-pressed", "true");
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ transition: "push", font: "georgia" }));

    fireEvent.click(within(panel).getByRole("button", { name: "Цвета" }));
    expect(within(panel).queryByRole("textbox", { name: "HEX: Шапка таблицы" })).not.toBeInTheDocument();
    const background = within(panel).getByRole("textbox", { name: "HEX: Фон слайда" });
    fireEvent.change(background, { target: { value: "#345" } });
    fireEvent.blur(background);
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ transition: "push", colors: { background: "#334455", text: "#eeeeee" } }));

    const changesBeforeReset = onChange.mock.calls.length;
    fireEvent.click(within(panel).getByRole("button", { name: "Сбросить" }));
    expect(onChange).toHaveBeenCalledTimes(changesBeforeReset + 1);
    expect(onChange).toHaveBeenLastCalledWith({ font: "geist", background: "original", animation: "on", transition: "fade", colors: {} });
    expect(background).toHaveValue("");
  });

  it("uses the rendered cover palette and automatic contrast in color pickers", () => {
    const onChange = vi.fn();
    const settings = (value: DeckAppearance) => <DeckAppearanceSettings value={value} onChange={onChange}
      colorFallbacks={resolveDeckAppearance(value, presentationStyles.business, true)} />;
    const { rerender } = render(settings(defaultDeckAppearance));
    fireEvent.click(screen.getByRole("button", { name: "Настройки показа" }));
    fireEvent.click(screen.getByRole("button", { name: "Цвета" }));
    const background = screen.getByLabelText("Выбрать цвет: Фон слайда");
    const text = screen.getByLabelText("Выбрать цвет: Текст слайда");
    expect(background).toHaveValue("#173c3a");
    expect(text).toHaveValue("#ffffff");

    rerender(settings({ ...defaultDeckAppearance, colors: { background: "#efefef" } }));
    expect(background).toHaveValue("#efefef");
    expect(text).toHaveValue("#000000");
    expect(screen.getByRole("textbox", { name: "HEX: Текст слайда" })).toHaveValue("");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("preserves the example's table controls and module appearance when resetting", () => {
    const value: PresentationAppearance = {
      font: "arial", background: "dark", animation: "off", colors: { tableHeader: "#123456" },
      modules: { model: { title: { font: "georgia", ink: "#222222" } } },
    };
    const onChange = vi.fn();
    render(<PresentationSettings value={value} table lightByDefault={false} onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Настройки показа" }));
    const panel = screen.getByRole("dialog", { name: "Настройки показа" });
    expect(within(panel).queryByRole("toolbar", { name: "Переход" })).not.toBeInTheDocument();
    fireEvent.click(within(panel).getByRole("button", { name: "Цвета" }));
    expect(within(panel).getByRole("textbox", { name: "HEX: Шапка таблицы" })).toHaveValue("#123456");
    expect(within(panel).getByLabelText("Выбрать цвет: Заливка таблицы")).toHaveValue("#151b23");
    fireEvent.click(within(panel).getByRole("button", { name: "Сбросить" }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith({ font: "geist", background: "original", animation: "on", colors: {}, modules: value.modules });
  });

  it("places the settings above the slideshow and handles Escape before the modal", () => {
    const value = { font: "geist", background: "original", animation: "on", colors: {}, transition: "none" } as const;
    const closeSlideshow = vi.fn();
    render(<LandingDialog title="Показ презентации" closeLabel="Закрыть показ" onClose={closeSlideshow}>
      <DeckAppearanceSettings value={value} onChange={vi.fn()} />
    </LandingDialog>);
    const trigger = screen.getByRole("button", { name: "Настройки показа" });
    fireEvent.click(trigger);
    const panel = screen.getByRole("dialog", { name: "Настройки показа" });
    expect(panel).toHaveStyle({ zIndex: "170" });
    fireEvent.keyDown(within(panel).getByRole("button", { name: "Плавный" }), { key: "Escape" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
    expect(screen.getByRole("dialog", { name: "Показ презентации" })).toBeVisible();
    expect(screen.getByTestId("landing-dialog-backdrop")).toHaveAttribute("data-state", "open");
    expect(closeSlideshow).not.toHaveBeenCalled();
  });
});
