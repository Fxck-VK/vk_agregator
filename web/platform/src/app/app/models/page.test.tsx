import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/features/models/ModelsCatalog/ModelsCatalog", () => ({
  ModelsCatalog: (props: Record<string, unknown>) => (
    <p>model catalog {Object.keys(props).length === 0 ? "without props" : "with props"}</p>
  ),
}));

import ModelsPage from "./page";

it("renders the model catalog route", () => {
  render(<ModelsPage />);

  expect(screen.getByText("model catalog without props")).toBeInTheDocument();
});
