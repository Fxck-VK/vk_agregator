import { act, cleanup, createEvent, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AccountMenu } from "./AccountMenu";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function openMenu() {
  render(<AccountMenu identityLabel="preview@neirohub.local" isLogoutPending={false} onLogout={vi.fn()} />);
  const trigger = screen.getByRole("button", { name: "Открыть меню аккаунта" });
  fireEvent.click(trigger);
  return { trigger, menu: document.getElementById("account-menu")! };
}

function finishTransition(element: Element, propertyName = "opacity") {
  const event = createEvent.transitionEnd(element);
  Object.defineProperty(event, "propertyName", { value: propertyName });
  fireEvent(element, event);
}

describe("AccountMenu motion", () => {
  it.each(["escape", "trigger", "outside"])("retains an inert panel until the %s close transition finishes", (method) => {
    const { trigger, menu } = openMenu();
    expect(menu).toHaveAttribute("data-motion-state", "open");
    if (method === "escape") fireEvent.keyDown(menu, { key: "Escape" });
    else if (method === "trigger") fireEvent.click(trigger);
    else fireEvent.pointerDown(document.body);

    expect(menu).toBeInTheDocument();
    expect(menu).toHaveAttribute("inert");
    expect(menu).toHaveAttribute("aria-hidden", "true");
    expect(menu).toHaveAttribute("data-motion-state", "closing");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    if (method !== "trigger") expect(trigger).toHaveFocus();

    finishTransition(menu.querySelector("a")!);
    finishTransition(menu, "background-color");
    expect(menu).toBeInTheDocument();
    finishTransition(menu);
    expect(menu).not.toBeInTheDocument();
  });

  it("keeps a rapidly reopened panel alive when the old close would finish", () => {
    vi.useFakeTimers();
    const { trigger, menu } = openMenu();
    fireEvent.click(trigger);
    fireEvent.click(trigger);
    expect(document.getElementById("account-menu")).toBe(menu);
    expect(menu).not.toHaveAttribute("inert");
    finishTransition(menu);
    act(() => vi.advanceTimersByTime(1000));
    expect(menu).toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-expanded", "true");
  });

  it("finishes closing if the browser does not send a transition event", () => {
    vi.useFakeTimers();
    const { trigger, menu } = openMenu();
    fireEvent.click(trigger);
    act(() => vi.advanceTimersByTime(200));
    expect(menu).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1000));
    expect(menu).not.toBeInTheDocument();
  });

  it("closes immediately with reduced motion", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
    const { trigger, menu } = openMenu();
    fireEvent.click(trigger);
    expect(menu).not.toBeInTheDocument();
  });

  it("responds when reduced motion is enabled during closing and removes the listener", () => {
    let reduced = false;
    const listeners = new Set<() => void>();
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      get matches() { return reduced; },
      addEventListener: (_: string, listener: () => void) => listeners.add(listener),
      removeEventListener: (_: string, listener: () => void) => listeners.delete(listener),
    })));
    const { trigger, menu } = openMenu();
    fireEvent.click(trigger);
    expect(menu).toBeInTheDocument();
    act(() => { reduced = true; listeners.forEach((listener) => listener()); });
    expect(menu).not.toBeInTheDocument();
    cleanup();
    expect(listeners.size).toBe(0);
  });
});
