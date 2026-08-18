import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import { WorkspaceLoginAction } from "./WorkspaceLoginAction";

describe("WorkspaceLoginAction", () => {
  afterEach(() => cleanup());

  it.each(["header", "sidebar"] as const)("links the %s guest action to the existing login page", (placement) => {
    render(<WorkspaceLoginAction placement={placement} />);

    expect(screen.getByRole("link", { name: ru.login.submitLabel })).toHaveAttribute("href", "/login");
    expect(screen.getByRole("link", { name: ru.login.submitLabel })).toHaveAttribute("data-placement", placement);
  });
});
