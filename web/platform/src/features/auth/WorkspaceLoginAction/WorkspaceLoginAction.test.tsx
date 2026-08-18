import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const workspaceLogout = vi.hoisted(() => ({
  controller: undefined as undefined | {
    phase: "failed" | "pending";
    requestLogin: () => void;
  },
}));

vi.mock("@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary", () => ({
  useOptionalWorkspaceLogout: () => workspaceLogout.controller,
}));

import { ru } from "@/i18n/ru";

import { WorkspaceLoginAction } from "./WorkspaceLoginAction";

describe("WorkspaceLoginAction", () => {
  afterEach(() => {
    cleanup();
    workspaceLogout.controller = undefined;
  });

  it.each(["header", "sidebar"] as const)("links the %s guest action to the existing login page", (placement) => {
    render(<WorkspaceLoginAction placement={placement} />);

    expect(screen.getByRole("link", { name: ru.login.submitLabel })).toHaveAttribute("href", "/login");
    expect(screen.getByRole("link", { name: ru.login.submitLabel })).toHaveAttribute("data-placement", placement);
  });

  it("keeps the optimistic guest shell visible while the server logout finishes", () => {
    const requestLogin = vi.fn();
    workspaceLogout.controller = { phase: "pending", requestLogin };

    render(<WorkspaceLoginAction placement="header" />);
    fireEvent.click(screen.getByRole("button", { name: ru.login.submitLabel }));

    expect(requestLogin).toHaveBeenCalledOnce();
    expect(screen.queryByRole("link", { name: ru.login.submitLabel })).not.toBeInTheDocument();
  });
});
