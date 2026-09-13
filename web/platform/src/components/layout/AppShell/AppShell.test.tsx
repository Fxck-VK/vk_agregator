import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { AppShell } from "./AppShell";

describe("AppShell", () => {
  afterEach(cleanup);

  it("provides a named complementary navigation region and one workspace scroll region", () => {
    render(
      <AppShell sidebar={<nav>Navigation</nav>}>
        <h1>Workspace</h1>
      </AppShell>,
    );

    expect(screen.getByRole("complementary", { name: ru.navigation.regionLabel })).toContainElement(
      screen.getByRole("navigation"),
    );
    expect(screen.getByTestId("workspace-scroll-region")).toHaveTextContent("Workspace");
  });

  it("keeps an optional workspace header outside the scroll region", () => {
    render(
      <AppShell header={<header data-testid="workspace-header">Section</header>} sidebar={<nav>Navigation</nav>}>
        <h1>Workspace</h1>
      </AppShell>,
    );

    const scrollRegion = screen.getByTestId("workspace-scroll-region");
    const workspaceHeader = screen.getByTestId("workspace-header");

    expect(scrollRegion).not.toContainElement(workspaceHeader);
    expect(workspaceHeader.parentElement).toContainElement(scrollRegion);
  });

  it("removes the desktop sidebar layout offset when the sidebar is collapsed", () => {
    render(
      <AppShell isDesktopSidebarCollapsed sidebar={<nav>Navigation</nav>}>
        <h1>Workspace</h1>
      </AppShell>,
    );

    expect(screen.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
    expect(screen.getByRole("complementary", { name: ru.navigation.regionLabel })).toHaveAttribute(
      "data-desktop-sidebar-collapsed",
      "true",
    );
  });
});
