import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/features/session/local-workspace-preview", () => ({
  isLocalWorkspacePreviewEnabled: () => true,
}));

vi.mock("@/features/models/ModelsCatalog/ModelsCatalog", () => ({
  ModelsCatalog: ({ includePlaceholders }: { includePlaceholders?: boolean }) => (
    <p>model catalog {includePlaceholders ? "with placeholders" : "without placeholders"}</p>
  ),
}));

import ModelsPage from "./page";

it("renders the model catalog route", () => {
  render(<ModelsPage />);

  expect(screen.getByText("model catalog with placeholders")).toBeInTheDocument();
});
