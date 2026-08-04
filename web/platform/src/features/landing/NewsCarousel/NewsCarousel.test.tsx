import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { NewsCarousel } from "./NewsCarousel";

describe("NewsCarousel", () => {
  it("moves through local news with accessible controls", () => {
    render(<NewsCarousel />);

    expect(screen.getByRole("heading", { name: "Новости NeiroHub" })).toBeInTheDocument();
    expect(screen.getByText("01 / 02")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Следующая новость" }));

    expect(screen.getByText("02 / 02")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Создать изображение" })).toHaveAttribute(
      "href",
      "/login?next=/app/image",
    );
  });
});
