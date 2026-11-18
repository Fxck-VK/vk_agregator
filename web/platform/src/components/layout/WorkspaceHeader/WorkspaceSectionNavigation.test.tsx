import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { WorkspaceSectionNavigation } from "./WorkspaceSectionNavigation";
const releaseView = vi.hoisted(() => ({ view: "prod", path: "/app/audio-download" }));
vi.mock("@/i18n/navigation", () => ({ usePathname: () => releaseView.path, useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ allowsPath: (href: string) => releaseView.view === "dev" || href === "/app/audio-download" }) }));
afterEach(cleanup);
it("hides unavailable audio sections in PROD and retains DEV navigation", () => {
  const view = render(<WorkspaceSectionNavigation />);
  expect(screen.queryByRole("toolbar")).not.toBeInTheDocument();
  releaseView.view = "dev";
  view.rerender(<WorkspaceSectionNavigation />);
  expect(screen.getByRole("toolbar")).toHaveTextContent("Скачать MP3");
});
