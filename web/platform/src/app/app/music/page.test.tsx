import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/features/music/MusicWorkspace", () => ({
  MusicWorkspaceController: (props: Record<string, unknown>) => (
    <p>music workspace {String(props.requestedModelId ?? "default")}</p>
  ),
}));

import MusicPage from "./page";

it.each([undefined, "lyria_3_5", ["suno_v6", "lyria_3_5"]])("passes a single model query to the music workspace: %s", async model => {
  render(await MusicPage({ searchParams: Promise.resolve({ model }) }));

  expect(screen.getByText(`music workspace ${typeof model === "string" ? model : "default"}`)).toBeInTheDocument();
});
