import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { inspirationExamples } from "@/features/inspiration/inspiration-examples";
import { INSPIRATION_IMAGE_SIZES } from "@/features/inspiration/InspirationExampleMedia/InspirationExampleMedia";
import { ru } from "@/i18n/ru";

import { ImageTemplatePicker } from "./ImageTemplatePicker";

describe("ImageTemplatePicker", () => {
  afterEach(() => {
    cleanup();
  });

  it("shows shared image templates without search and reports the selected template", () => {
    const onSelect = vi.fn();
    render(<ImageTemplatePicker onSelect={onSelect} />);

    const trigger = screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open });
    expect(trigger.querySelector(
      'img[src="/assets/icons/ui/template-select-white.svg"]',
    )).toBeInTheDocument();
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: ru.imageGeneration.templatePicker.title });
    expect(dialog).toBeVisible();
    expect(dialog.querySelector('[data-track-placement="outside"]')).toBeInTheDocument();
    expect(screen.getByRole("img", { name: inspirationExamples[0].mediaAlt })).toHaveAttribute(
      "sizes",
      INSPIRATION_IMAGE_SIZES,
    );

    expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.close })).toHaveFocus();
    expect(screen.getAllByRole("button", { name: /^Выбрать шаблон .+/ })).toHaveLength(
      inspirationExamples.filter((template) => template.mediaType === "image").length,
    );
    fireEvent.click(screen.getByRole("button", {
      name: `${ru.imageGeneration.templatePicker.select} ${inspirationExamples[0].title}`,
    }));

    expect(onSelect).toHaveBeenCalledWith(inspirationExamples[0]);
    fireEvent.animationEnd(screen.getByRole("dialog").closest("[data-state]")!, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("closes from Escape without selecting a template", () => {
    const onSelect = vi.fn();
    render(<ImageTemplatePicker onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open }));
    const backdrop = screen.getByRole("dialog").closest("[data-state]")!;
    fireEvent.keyDown(document, { key: "Escape" });

    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open })).toHaveFocus();
    expect(onSelect).not.toHaveBeenCalled();
  });
});
