import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { PublicHeader } from "./PublicHeader";

describe("PublicHeader", () => {
  afterEach(() => cleanup());

  it("shows the current tool and a guest login action", () => {
    render(<PublicHeader selectedToolName="NeiroHub Chat" />);

    expect(screen.getByText("NeiroHub Chat")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Войти" })).toHaveAttribute("href", "/login?next=/app");
    expect(screen.queryByLabelText(/баланс/i)).not.toBeInTheDocument();
  });
});
