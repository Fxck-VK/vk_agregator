import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ModelCatalogToolbar } from "./ModelCatalogToolbar";

describe("ModelCatalogToolbar", () => {
  it("reports search input and category-tab selection through the refreshed public API", () => {
    const onCategoryChange = vi.fn();
    const onQueryChange = vi.fn();

    render(
      <ModelCatalogToolbar
        categories={[
          { id: "popular", label: "Популярные" },
          { id: "images", label: "Изображения" },
          { id: "text", label: "Текст" },
        ]}
        category="popular"
        onCategoryChange={onCategoryChange}
        onQueryChange={onQueryChange}
        query=""
      />,
    );

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "banana" },
    });
    fireEvent.click(screen.getByRole("tab", { name: "Текст" }));

    expect(onQueryChange).toHaveBeenCalledWith("banana");
    expect(onCategoryChange).toHaveBeenCalledWith("text");
    expect(screen.getByRole("tab", { name: "Популярные" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("checkbox", { name: ru.modelsCatalog.referenceFilterLabel })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: ru.modelsCatalog.qualityFilterLabel })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: ru.modelsCatalog.sortLabel })).not.toBeInTheDocument();
  });
});
