import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { WorkspaceSectionNavigation } from "./WorkspaceSectionNavigation";
const releaseView = vi.hoisted(() => ({ view: "prod" }));
vi.mock("@/i18n/navigation", () => ({ usePathname: () => "/app/music", useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ allowsPath: () => releaseView.view === "dev" }) }));
afterEach(cleanup);
it("hides unavailable audio sections in PROD and retains DEV navigation", () => {
  const view = render(<WorkspaceSectionNavigation />);
  expect(screen.queryByRole("toolbar")).not.toBeInTheDocument();
  releaseView.view = "dev";
  view.rerender(<WorkspaceSectionNavigation />);
  expect(screen.getByRole("toolbar")).toBeInTheDocument();
});
