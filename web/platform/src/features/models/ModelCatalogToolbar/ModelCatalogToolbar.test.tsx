import { useState } from "react";

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ModelCatalogToolbar, type ModelCatalogCategory } from "./ModelCatalogToolbar";

const categories = [
  { id: "popular", label: "Популярные" },
  { id: "images", label: "Изображения" },
  { id: "text", label: "Текст" },
] as const;

function ControlledToolbar({ onCategoryChange }: { onCategoryChange: (value: ModelCatalogCategory["id"]) => void }) {
  const [category, setCategory] = useState<ModelCatalogCategory["id"]>("images");

  return (
    <ModelCatalogToolbar
      categories={categories}
      category={category}
      onCategoryChange={(value) => {
        onCategoryChange(value);
        setCategory(value);
      }}
      onQueryChange={() => {}}
      query=""
      tabPanelId="models-panel"
    />
  );
}

afterEach(cleanup);

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
        tabPanelId="models-panel"
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

  it("exposes a named tablist and connects every category tab to the catalog panel", () => {
    render(
      <ModelCatalogToolbar
        categories={categories}
        category="popular"
        onCategoryChange={() => {}}
        onQueryChange={() => {}}
        query=""
        tabPanelId="models-panel"
      />,
    );

    expect(screen.getByRole("tablist")).toHaveAccessibleName("Категории нейросетей");
    for (const category of categories) {
      expect(screen.getByRole("tab", { name: category.label })).toHaveAttribute("id", `model-category-tab-${category.id}`);
      expect(screen.getByRole("tab", { name: category.label })).toHaveAttribute("aria-controls", "models-panel");
    }
  });

  it.each([
    ["ArrowLeft", "popular", "Популярные"],
    ["ArrowRight", "text", "Текст"],
    ["Home", "popular", "Популярные"],
    ["End", "text", "Текст"],
  ] as const)("moves focus and controlled selection on %s", (key, expectedCategory, expectedLabel) => {
    const onCategoryChange = vi.fn();
    render(<ControlledToolbar onCategoryChange={onCategoryChange} />);
    const selectedTab = screen.getByRole("tab", { name: "Изображения" });
    selectedTab.focus();

    fireEvent.keyDown(selectedTab, { key });

    const nextTab = screen.getByRole("tab", { name: expectedLabel });
    expect(nextTab).toHaveFocus();
    expect(nextTab).toHaveAttribute("aria-selected", "true");
    expect(selectedTab).toHaveAttribute("aria-selected", "false");
    expect(onCategoryChange).toHaveBeenCalledWith(expectedCategory);
  });
});
