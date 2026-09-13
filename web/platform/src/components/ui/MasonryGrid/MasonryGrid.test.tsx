import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { MasonryGrid } from "./MasonryGrid";

describe("MasonryGrid", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders a semantic ordered list and forwards list attributes", () => {
    render(
      <MasonryGrid aria-label="Галерея" className="consumer-grid" data-testid="masonry-grid">
        <li>Первое фото</li>
        <li>Второе фото</li>
      </MasonryGrid>,
    );

    const grid = screen.getByRole("list", { name: "Галерея" });
    expect(grid).toHaveClass("consumer-grid");
    expect(grid.classList).toHaveLength(2);
    expect(grid).toHaveAttribute("data-testid", "masonry-grid");
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });
});
