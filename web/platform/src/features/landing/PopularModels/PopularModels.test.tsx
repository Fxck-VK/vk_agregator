import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PopularModels } from "./PopularModels";

describe("PopularModels", () => {
  it("reveals the local model set without a network request", () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    render(<PopularModels />);

    expect(screen.getAllByTestId("public-model-card")).toHaveLength(4);
    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
    expect(screen.getAllByTestId("public-model-card")).toHaveLength(10);
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });
});
