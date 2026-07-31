import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { AppShell } from "./AppShell";

describe("AppShell", () => {
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
});
