import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AccountUpdatesPanel } from "./AccountUpdatesPanel";
const releaseView = vi.hoisted(() => ({ view: "prod" }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ enabled: () => releaseView.view === "dev" }) }));
afterEach(cleanup);
it("hides the unconnected suggestion in PROD and preserves the disabled DEV action", () => {
  const view = render(<AccountUpdatesPanel id="updates" />);
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
  releaseView.view = "dev";
  view.rerender(<AccountUpdatesPanel id="updates" />);
  expect(screen.getByRole("button")).toBeDisabled();
});
