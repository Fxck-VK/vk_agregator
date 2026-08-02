import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/features/models/ModelsCatalog/ModelsCatalog", () => ({
  ModelsCatalog: () => <p>model catalog</p>,
}));

import ModelsPage from "./page";

it("renders the model catalog route", () => {
  render(<ModelsPage />);

  expect(screen.getByText("model catalog")).toBeInTheDocument();
});
