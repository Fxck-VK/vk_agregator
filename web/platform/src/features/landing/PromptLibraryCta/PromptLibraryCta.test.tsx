import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PromptLibraryCta } from "./PromptLibraryCta";

describe("PromptLibraryCta", () => {
  it("uses the authenticated inspiration target", () => {
    render(<PromptLibraryCta />);
    expect(screen.getByRole("link", { name: "Смотреть примеры" })).toHaveAttribute(
      "href",
      "/login?next=/app/inspiration",
    );
  });
});
