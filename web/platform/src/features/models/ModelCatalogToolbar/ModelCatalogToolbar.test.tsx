import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ModelCatalogToolbar } from "./ModelCatalogToolbar";

describe("ModelCatalogToolbar", () => {
  it("shows the number of matching models and reports changed sort", () => {
    const onSortChange = vi.fn();

    render(
      <ModelCatalogToolbar
        onClear={vi.fn()}
        onQualityChange={vi.fn()}
        onQueryChange={vi.fn()}
        onReferenceOnlyChange={vi.fn()}
        onSortChange={onSortChange}
        qualities={["1K"]}
        quality={null}
        query=""
        referenceOnly={false}
        resultCount={2}
        sort="catalog"
      />,
    );

    fireEvent.change(screen.getByRole("combobox", { name: ru.modelsCatalog.sortLabel }), {
      target: { value: "name" },
    });

    expect(onSortChange).toHaveBeenCalledWith("name");
    expect(screen.getByText(ru.modelsCatalog.resultCount(2))).toBeInTheDocument();
  });
});
