import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ChatScrollToBottom } from "./ChatScrollToBottom";

describe("ChatScrollToBottom", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
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

  it("replaces the arrow with typing dots while a reply is pending and restores it afterwards", () => {
    const region = createScrollRegion({ scrollTop: 100 });
    const { rerender } = render(
      <ChatScrollToBottom
        contentVersion="1"
        forceScrollRequest={0}
        isAwaitingResponse
        scrollContainer={region.element}
      />,
    );

    expect(screen.queryByRole("button", { name: ru.conversations.scrollToLatest })).toBeNull();
    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeVisible();

    rerender(
      <ChatScrollToBottom
        contentVersion="2"
        forceScrollRequest={0}
        isAwaitingResponse={false}
        scrollContainer={region.element}
      />,
    );

    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
  });

  it("follows new content only while it was already at the bottom", () => {
    const region = createScrollRegion({ scrollTop: 1200 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    region.setScrollHeight(1800);
    rerender(<ChatScrollToBottom contentVersion="2" forceScrollRequest={0} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: 1800 });
    region.element.scrollTop = 1400;
    fireEvent.scroll(region.element);
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
    region.element.scrollTop = 1200;
    fireEvent.scroll(region.element);
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

  it("keeps following through intermediate programmatic scroll events", () => {
    const region = createScrollRegion({ scrollTop: 1200 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    region.setScrollHeight(1800);
    rerender(<ChatScrollToBottom contentVersion="2" forceScrollRequest={0} scrollContainer={region.element} />);
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();

    region.element.scrollTop = 1300;
    fireEvent.scroll(region.element);
    region.setScrollHeight(2000);
    rerender(<ChatScrollToBottom contentVersion="3" forceScrollRequest={0} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenLastCalledWith({ behavior: "smooth", top: 2000 });
  });

  it("shows the control until a programmatic scroll actually reaches the bottom", () => {
    const region = createScrollRegion({ scrollTop: 1200 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    region.setScrollHeight(1800);
    rerender(<ChatScrollToBottom contentVersion="2" forceScrollRequest={0} scrollContainer={region.element} />);
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();

    region.element.scrollTop = 1400;
    fireEvent.scroll(region.element);

    expect(screen.queryByRole("button", { name: ru.conversations.scrollToLatest })).toBeNull();
  });

  it("stops following after a programmatic scroll settles away from the bottom", () => {
    vi.useFakeTimers();
    const region = createScrollRegion({ scrollTop: 1200 });
    const { rerender } = render(
      <ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region.element} />,
    );

    region.setScrollHeight(1800);
    rerender(<ChatScrollToBottom contentVersion="2" forceScrollRequest={0} scrollContainer={region.element} />);
    region.element.scrollTop = 900;
    fireEvent.scroll(region.element);
    act(() => {
      vi.advanceTimersByTime(200);
    });

    region.setScrollHeight(2000);
    rerender(<ChatScrollToBottom contentVersion="3" forceScrollRequest={0} scrollContainer={region.element} />);

    expect(region.scrollTo).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
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
