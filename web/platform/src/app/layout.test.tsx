import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({
  headers: vi.fn(),
}));

import { headers } from "next/headers";

import RootLayout, { metadata } from "./layout";

describe("RootLayout", () => {
  beforeEach(() => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-nonce": "test-theme-nonce" }) as never);
  });

  it("sets Russian as the document language", async () => {
    const markup = renderToStaticMarkup(
      await RootLayout({
        children: <main>Тест</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");

    expect(document.documentElement.getAttribute("lang")).toBe("ru");
  });

  it("uses the NeiroHub chip as the browser tab icon", () => {
    expect(metadata.icons).toEqual({
      icon: "/assets/brand/marks/neirohub-chip.png",
      shortcut: "/assets/brand/marks/neirohub-chip.png",
    });
  });

  it("bootstraps the persisted theme in the head before page content with the request CSP nonce", async () => {
    const markup = renderToStaticMarkup(
      await RootLayout({
        children: <main>Theme content</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");
    const bootstrapScript = document.querySelector("head script");

    expect(document.documentElement.getAttribute("data-theme")).toBe("system");
    expect(bootstrapScript?.textContent).toContain("neirohub.theme");
    expect(bootstrapScript?.getAttribute("nonce")).toBe("test-theme-nonce");
    expect(markup.indexOf("<script")).toBeLessThan(markup.indexOf("<body"));
  });
});
