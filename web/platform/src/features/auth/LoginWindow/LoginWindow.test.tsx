import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const replace = vi.hoisted(() => vi.fn());
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("./LoginShowcase", () => ({ LoginShowcase: () => <div /> }));
import { LoginWindow } from "./LoginWindow";

afterEach(() => { cleanup(); vi.clearAllMocks(); });
describe("LoginWindow", () => {
  it("uses the selected reference shell by default, with one brand link and no modal close", () => {
    render(<LoginWindow><p>Sign in</p></LoginWindow>);
    expect(screen.getByTestId("login-window")).toHaveAttribute("data-presentation", "reference");
    expect(screen.getByRole("link", { name: "Epifera" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Закрыть окно входа" })).not.toBeInTheDocument();
  });
  it("keeps the standalone login open when Escape is pressed", () => {
    render(<LoginWindow><p>Sign in</p></LoginWindow>);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByText("Sign in")).toBeInTheDocument();
  });
});
