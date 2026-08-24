import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { inspirationExamples } from "@/features/inspiration/inspiration-examples";
import { ru } from "@/i18n/ru";

import { ImageTemplatePicker } from "./ImageTemplatePicker";

describe("ImageTemplatePicker", () => {
  afterEach(() => {
    cleanup();
  });

  it("filters shared Inspiration templates and reports the selected template", () => {
    const onSelect = vi.fn();
    render(<ImageTemplatePicker onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open }));
    expect(screen.getByRole("dialog", { name: ru.imageGeneration.templatePicker.title })).toBeVisible();

    fireEvent.change(screen.getByRole("searchbox", { name: ru.imageGeneration.templatePicker.searchLabel }), {
      target: { value: "шаблон которого нет" },
    });
    expect(screen.getByText(ru.imageGeneration.templatePicker.empty)).toBeVisible();

    fireEvent.change(screen.getByRole("searchbox", { name: ru.imageGeneration.templatePicker.searchLabel }), {
      target: { value: inspirationExamples[0].title },
    });
    fireEvent.click(screen.getByRole("button", {
      name: `${ru.imageGeneration.templatePicker.select} ${inspirationExamples[0].title}`,
    }));

    expect(onSelect).toHaveBeenCalledWith(inspirationExamples[0]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("closes from Escape without selecting a template", () => {
    const onSelect = vi.fn();
    render(<ImageTemplatePicker onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.templatePicker.open }));
    fireEvent.keyDown(document, { key: "Escape" });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onSelect).not.toHaveBeenCalled();
  });
});
