import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ChatScrollToBottom } from "./ChatScrollToBottom";

describe("ChatScrollToBottom", () => {
  afterEach(() => {
    cleanup();
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
});
