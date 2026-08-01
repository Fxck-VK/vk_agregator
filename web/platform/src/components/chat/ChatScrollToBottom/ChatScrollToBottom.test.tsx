import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ChatScrollToBottom } from "./ChatScrollToBottom";

describe("ChatScrollToBottom", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows away from the bottom and smoothly returns to the latest message", () => {
    const region = document.createElement("div");
    const scrollTo = vi.fn();
    Object.defineProperties(region, {
      clientHeight: { configurable: true, value: 400 },
      scrollHeight: { configurable: true, value: 1600 },
      scrollTo: { configurable: true, value: scrollTo },
      scrollTop: { configurable: true, writable: true, value: 0 },
    });

    render(<ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region} />);

    region.scrollTop = 200;
    fireEvent.scroll(region);

    const button = screen.getByRole("button", { name: ru.conversations.scrollToLatest });
    expect(button).toBeVisible();

    fireEvent.click(button);
    expect(scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: 1200 });
  });

  it("follows new content only while it was already at the bottom", () => {
    const region = createScrollRegion({ scrollTop: 1200 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    region.setScrollHeight(1800);
    rerender(<ChatScrollToBottom contentVersion="2" forceScrollRequest={0} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: 1800 });
    expect(screen.queryByRole("button", { name: ru.conversations.scrollToLatest })).toBeNull();

    region.scrollTo.mockClear();
    region.element.scrollTop = 100;
    fireEvent.scroll(region.element);
    region.setScrollHeight(2000);
    rerender(<ChatScrollToBottom contentVersion="3" forceScrollRequest={0} scrollContainer={region.element} />);

    expect(region.scrollTo).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
  });

  it("forces the latest position and hides the control", () => {
    const region = createScrollRegion({ scrollTop: 100 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
    rerender(<ChatScrollToBottom contentVersion="1" forceScrollRequest={1} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: 1600 });
    expect(screen.queryByRole("button", { name: ru.conversations.scrollToLatest })).toBeNull();
  });

  it("uses automatic scrolling when reduced motion is preferred", () => {
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({ matches: true }));
    const region = createScrollRegion({ scrollTop: 100 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    rerender(<ChatScrollToBottom contentVersion="1" forceScrollRequest={1} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "auto", top: 1600 });
  });
});

function createScrollRegion({ scrollTop }: { scrollTop: number }) {
  const element = document.createElement("div");
  const scrollTo = vi.fn();
  Object.defineProperties(element, {
    clientHeight: { configurable: true, value: 400 },
    scrollHeight: { configurable: true, writable: true, value: 1600 },
    scrollTo: { configurable: true, value: scrollTo },
    scrollTop: { configurable: true, writable: true, value: scrollTop },
  });

  return {
    element,
    scrollTo,
    setScrollHeight: (scrollHeight: number) => {
      Object.defineProperty(element, "scrollHeight", { configurable: true, value: scrollHeight });
    },
  };
}
