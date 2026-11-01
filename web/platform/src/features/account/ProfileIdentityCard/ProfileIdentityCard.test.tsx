import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ProfileIdentityCard } from "./ProfileIdentityCard";

const props = { hasVerifiedIdentity: true, identityLabel: "Test account", publicID: "EP-104582" };

describe("ProfileIdentityCard public ID", () => {
  afterEach(() => { vi.useRealTimers(); Reflect.deleteProperty(navigator, "clipboard"); });

  it("hides success after five seconds while keeping the ID and copy button available", async () => {
    vi.useFakeTimers();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
    render(<ProfileIdentityCard {...props} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Скопировать ID аккаунта" })); });
    expect(screen.getByText("ID скопирован")).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(4999); });
    expect(screen.getByText("ID скопирован")).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(1); });
    expect(screen.queryByText("ID скопирован")).not.toBeInTheDocument();
    expect(screen.getByText(props.publicID)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Скопировать ID аккаунта" })).toBeEnabled();
  });

  it("restarts the five-second countdown after another successful copy", async () => {
    vi.useFakeTimers();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
    render(<ProfileIdentityCard {...props} />);
    const button = screen.getByRole("button", { name: "Скопировать ID аккаунта" });
    await act(async () => { fireEvent.click(button); });
    act(() => { vi.advanceTimersByTime(4000); });
    await act(async () => { fireEvent.click(button); });
    act(() => { vi.advanceTimersByTime(4999); });
    expect(screen.getByText("ID скопирован")).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(1); });
    expect(screen.queryByText("ID скопирован")).not.toBeInTheDocument();
  });

  it("cleans up the dismissal timer when the account card unmounts", async () => {
    vi.useFakeTimers();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
    const { unmount } = render(<ProfileIdentityCard {...props} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Скопировать ID аккаунта" })); });
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("shows the server ID and copies only its exact value on explicit activation", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    render(<ProfileIdentityCard {...props} />);
    expect(screen.getByText(props.publicID)).toBeInTheDocument();
    expect(writeText).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Скопировать ID аккаунта" }));
    expect(await screen.findByText("ID скопирован")).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(props.publicID);
  });

  it("does not invent an ID or copy action for an older profile", () => {
    render(<ProfileIdentityCard hasVerifiedIdentity identityLabel="Test account" />);
    expect(screen.queryByText(/EP-/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText(ru.profile.verifiedIdentity)).toBeInTheDocument();
  });

  it("announces pending honestly and prevents duplicate copies until the clipboard resolves", async () => {
    let finish!: () => void;
    const writeText = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    render(<ProfileIdentityCard {...props} />);
    const button = screen.getByRole("button", { name: "Скопировать ID аккаунта" });
    fireEvent.click(button);
    expect(button).toBeDisabled();
    expect(screen.getByRole("status")).toHaveTextContent("Копирование ID…");
    expect(screen.queryByText("ID скопирован")).not.toBeInTheDocument();
    fireEvent.click(button);
    await act(async () => { finish(); });
    expect(button).toBeEnabled();
    expect(writeText).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("status")).toHaveTextContent("ID скопирован");
  });

  it.each(["unavailable", "rejected"])("keeps ID text available for manual copy when clipboard is %s", async kind => {
    if (kind === "rejected") Object.defineProperty(navigator, "clipboard", {
      configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error("denied")) },
    });
    render(<ProfileIdentityCard {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Скопировать ID аккаунта" }));
    expect(await screen.findByText("Не удалось скопировать. Выделите ID и скопируйте вручную.")).toBeInTheDocument();
    expect(screen.getByText(props.publicID)).toBeInTheDocument();
    expect(screen.queryByText("ID скопирован")).not.toBeInTheDocument();
  });

  it("clears copy feedback and ignores an old pending result after the ID changes", async () => {
    let finish!: () => void;
    const writeText = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const { rerender } = render(<ProfileIdentityCard {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Скопировать ID аккаунта" }));
    rerender(<ProfileIdentityCard {...props} publicID="EP-104583" />);
    await act(async () => { finish(); });
    expect(screen.getByText("EP-104583")).toBeInTheDocument();
    expect(screen.queryByText(props.publicID)).not.toBeInTheDocument();
    expect(screen.queryByText("ID скопирован")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Скопировать ID аккаунта" })).toBeEnabled();
  });
});
