import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const replace = vi.hoisted(() => vi.fn());
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("./LoginShowcase", () => ({ LoginShowcase: () => <div /> }));
import { LoginWindow } from "./LoginWindow";

afterEach(() => { cleanup(); vi.clearAllMocks(); });
describe("LoginWindow", () => {
  it("closes to the localized guest workspace using the shared close control", () => {
    render(<LoginWindow><p>Sign in</p></LoginWindow>);
    fireEvent.click(screen.getByRole("button", { name: "Закрыть окно входа" }));
    expect(replace).toHaveBeenCalledWith("/app");
  });
  it("supports Escape and removes the document listener when unmounted", () => {
    const { unmount } = render(<LoginWindow><p>Sign in</p></LoginWindow>);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(replace).toHaveBeenCalledTimes(1);
    unmount(); fireEvent.keyDown(document, { key: "Escape" });
    expect(replace).toHaveBeenCalledTimes(1);
  });
});
