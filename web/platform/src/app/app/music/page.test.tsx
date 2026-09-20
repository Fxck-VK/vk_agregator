import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/features/music/MusicWorkspace", () => ({
  MusicWorkspaceController: (props: Record<string, unknown>) => (
    <p>music workspace {Object.keys(props).length === 0 ? "without props" : "with props"}</p>
  ),
}));

import MusicPage from "./page";

it("renders the music workspace route", () => {
  render(<MusicPage />);

  expect(screen.getByText("music workspace without props")).toBeInTheDocument();
});
