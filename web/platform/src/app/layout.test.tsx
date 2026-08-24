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

  it("uses dedicated square NeiroHub assets for browser and device icons", () => {
    expect(metadata.icons).toEqual({
      icon: [
        {
          url: "/assets/brand/favicons/neirohub-favicon-32.png",
          sizes: "32x32",
          type: "image/png",
        },
        {
          url: "/assets/brand/favicons/neirohub-favicon-48.png",
          sizes: "48x48",
          type: "image/png",
        },
      ],
      shortcut: "/assets/brand/favicons/neirohub-favicon-32.png",
      apple: [
        {
          url: "/assets/brand/favicons/neirohub-apple-touch-icon-180.png",
          sizes: "180x180",
          type: "image/png",
        },
      ],
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
